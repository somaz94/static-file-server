# GitLab Runner Cache Endpoint

static-file-server can act as the distributed cache backend of a GitLab Runner. With `S3_ENABLED=true` it opens a second listener that speaks the part of the S3 API the runner uses, and stores cache archives on the local filesystem, for example the same NFS share the file server already mounts.

This is **not** a general-purpose S3 server. It implements exactly what the runner's cache needs:

| Request | Used by the runner for |
|---|---|
| Presigned `GET /<bucket>/<key>` (incl. `Range`) | Cache download, parallel chunked download |
| Presigned `HEAD /<bucket>/<key>` | Existence and `Last-Modified` checks |
| Presigned `PUT /<bucket>/<key>` | Cache upload, with `Content-Type` and `x-amz-meta-*` |

Everything else (listing, multipart upload, `DELETE`, ACLs, header-signed requests) returns `501 NotImplemented`. Both runner adapters, `Type = "s3"` (minio-go) and `Type = "s3v2"` (AWS SDK v2), sign URLs offline once `BucketLocation` is set, so the runner manager never calls this endpoint; only the job pods' cache helper does.

<br/>

## How it works

- **Separate listener.** The endpoint listens on `S3_PORT` with its own handler. None of the file server's middleware (listing, access key, CORS) applies, and the file server never gains a write path.
- **Authentication.** Every request must carry an AWS Signature Version 4 query signature (`X-Amz-Signature`) made with `S3_ACCESS_KEY` / `S3_SECRET_KEY`. Signed headers, method, path and expiry are all verified; the region in the credential scope is accepted as signed. A `HEAD` on a URL presigned for `GET` is accepted, because the runner probes that way and it reveals nothing a `GET` would not.
- **Storage layout.** Objects live at `<S3_FOLDER>/<bucket>/<key>`. ETag, content type and user metadata live in a sidecar under `<S3_FOLDER>/.meta/`. Uploads are written to `<S3_FOLDER>/.tmp/` and renamed into place, so concurrent writers to the same key never expose a partial file.
- **Expiry.** With `S3_TTL` set, a background sweep deletes objects whose upload time is older than the TTL, like an S3 lifecycle expiration rule, then removes empty directories. An expired cache is simply a cache miss; the next job re-uploads it.

<br/>

## Configuration

| Variable | Type | Default | Description |
|---|---|---|---|
| `S3_ENABLED` | bool | `false` | Enable the cache endpoint |
| `S3_PORT` | uint16 | `9000` | Listener port (must differ from `PORT`; binds to `HOST`) |
| `S3_FOLDER` | string | `""` | Directory holding the objects (required) |
| `S3_ACCESS_KEY` | string | `""` | Access key ID the runner signs with (required) |
| `S3_SECRET_KEY` | string | `""` | Secret access key (required) |
| `S3_BUCKETS` | string | `""` | Comma-separated bucket allowlist; empty accepts any valid bucket name |
| `S3_TTL` | duration | `0` | Delete objects older than this (Go duration, e.g. `336h`); `0` keeps them forever |
| `S3_MAX_OBJECT_SIZE` | int | `0` | Reject uploads larger than this many bytes; `0` means unlimited |

YAML keys use the same names in kebab case: `s3-enabled`, `s3-port`, `s3-folder`, `s3-access-key`, `s3-secret-key`, `s3-buckets` (list), `s3-ttl`, `s3-max-object-size`.

When `TLS_CERT` / `TLS_KEY` are set, the endpoint serves HTTPS with the same certificate. `DEBUG=true` logs one line per request; denied requests are always logged with the reason, never with the signature.

<br/>

### Keep the cache out of the file server

Cache archives can contain build secrets. Startup fails when `S3_FOLDER` is inside `FOLDER`, unless it sits below a dot directory and `HIDE_DOT_FILES=true` (which blocks serving dot paths):

| `FOLDER` | `S3_FOLDER` | `HIDE_DOT_FILES` | Result |
|---|---|---|---|
| `/web` | `/cache` | any | allowed |
| `/web` | `/web/.runner-cache` | `true` | allowed |
| `/web` | `/web/.runner-cache` | `false` | rejected |
| `/web` | `/web/cache` | any | rejected |

This check compares paths, so it cannot see two mounts of the same volume. If `/cache` is a second mount of the volume served at `/web`, the cache is still reachable through the file server. Use a separate volume, or mount a dot subdirectory and set `HIDE_DOT_FILES=true`. The Helm chart refuses to render the unsafe combination.

<br/>

## Runner configuration

```toml
[[runners]]
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "static-file-server-s3.file-server.svc:9000"
      BucketName = "runner-cache"
      BucketLocation = "us-east-1"
      Insecure = true
```

- `Shared = true` lets every runner manager replica read the caches the others wrote.
- `BucketLocation` must be set: the runner uses it as the signing region. Left empty, the `s3v2` adapter asks the server with `GetBucketLocation`, which this endpoint does not implement.
- `Insecure = true` means plain HTTP; drop it when the endpoint serves TLS.
- With the GitLab Runner Helm chart, pass the credentials through `runners.cache.secretName`, a Secret with `accesskey` and `secretkey` keys, instead of writing them into `config.toml`.

`Type = "s3v2"` takes the same `[runners.cache.s3]` block. Both adapters detect that a cluster-internal `ServerAddress` needs path-style URLs; `PathStyle = true` makes that explicit.

<br/>

## Helm chart

```yaml
s3Cache:
  enabled: true
  existingSecret: runner-cache-s3   # keys: accessKey / secretKey (configurable)
  buckets: runner-cache
  ttl: 336h
  existingClaim: runner-cache       # ReadWriteMany claim mounted at s3Cache.folder
```

The chart adds an `s3` container port and a `<release>-s3` Service. See [examples/s3-cache.yaml](../helm/static-file-server/examples/s3-cache.yaml) and the chart README for every value.

<br/>

## Operational notes

- **Replicas.** Several pods can share one `ReadWriteMany` volume: writes are atomic renames, and the sweep only deletes files past the TTL. Each pod runs its own sweep, which is harmless.
- **Timeouts.** The listener bounds only the request header read, because cache archives can take minutes to transfer. Keep it on an internal Service; do not expose it through an Ingress.
- **Key conflicts.** A filesystem cannot hold both `a` and `a/b` as files. Such a `PUT` returns `409 KeyConflict`. Runner cache keys do not produce this layout in practice.
- **NFS.** The sweep relies on file modification times, which NFS preserves. Temporary uploads older than 24 hours are treated as abandoned and removed.
