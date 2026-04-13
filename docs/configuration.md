# Configuration Guide

static-file-server supports three configuration methods. Priority (highest to lowest):

1. Environment variables
2. YAML config file
3. Default values

<br/>

## Environment Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `CORS` | bool | `false` | Enable CORS headers (`Access-Control-Allow-Origin: *`) |
| `DEBUG` | bool | `false` | Enable debug logging (config summary on startup + per-request log) |
| `HOST` | string | `""` | Hostname to bind (empty = all interfaces) |
| `PORT` | uint16 | `8080` | Port number |
| `FOLDER` | string | `/web` | Root folder to serve |
| `ALLOW_INDEX` | bool | `true` | Serve `index.html` for directory requests |
| `SHOW_LISTING` | bool | `true` | Show directory listing when no `index.html` |
| `URL_PREFIX` | string | `""` | URL path prefix (must start with `/`, no trailing `/`) |
| `TLS_CERT` | string | `""` | TLS certificate file path |
| `TLS_KEY` | string | `""` | TLS private key file path |
| `TLS_MIN_VERS` | string | `""` | Minimum TLS version (`TLS10`, `TLS11`, `TLS12`, `TLS13`) |
| `REFERRERS` | string | `""` | Comma-separated allowed referrer prefixes |
| `ACCESS_KEY` | string | `""` | URL parameter access key |
| `CUSTOM_HEADERS` | string | `""` | Comma-separated `Key:Value` custom response headers (e.g. `X-Frame-Options:DENY,Cache-Control:no-cache`) |
| `SPA` | bool | `false` | SPA mode: serve `index.html` for all non-file routes (incompatible with `SHOW_LISTING=true`) |
| `COMPRESSION` | bool | `false` | Enable gzip compression (skips binary files and Range requests) |
| `HIDE_DOT_FILES` | bool | `false` | Hide dot files/directories (e.g. `.env`, `.git`) from serving and listings |
| `LOG_FORMAT` | string | `text` | Log format: `text` (default) or `json` (structured logging) |
| `METRICS` | bool | `false` | Enable Prometheus-compatible metrics at `/metrics` |

Boolean values accept: `1`, `true`, `t`, `yes`, `y` (true) and `0`, `false`, `f`, `no`, `n` (false).

<br/>

## YAML Config File

Use the `-c` or `--config` flag to specify a YAML config file:

```bash
static-file-server -c config.yaml
```

Example config file:

```yaml
cors: true
debug: false
host: "0.0.0.0"
port: 8080
folder: /var/www
allow-index: true
show-listing: true
url-prefix: "/files"
referrers:
  - "https://example.com"
  - "https://cdn.example.com"
access-key: "my-secret-key"
custom-headers:
  X-Frame-Options: "DENY"
  Cache-Control: "public, max-age=3600"
spa: false
compression: true
hide-dot-files: true
log-format: "json"
metrics: true
```

<br/>

## Serving Modes

The combination of `ALLOW_INDEX` and `SHOW_LISTING` determines directory behavior:

| ALLOW_INDEX | SHOW_LISTING | Directory Behavior |
|---|---|---|
| `true` | `true` | Prefer `index.html`, fall back to listing |
| `true` | `false` | Serve `index.html` only, 404 if missing |
| `false` | `true` | Always show directory listing (ignore `index.html`) |
| `false` | `false` | Files only, directories return 404 |

<br/>

## Access Control

### Access Key

When `ACCESS_KEY` is set, all requests must include authentication:

```
# Direct key
GET /file.txt?key=my-secret-key

# SHA-256 code (case-insensitive)
GET /file.txt?code=<SHA256("/file.txt" + "my-secret-key")>
```

### Referrer Validation

When `REFERRERS` is set, the `Referer` header must match one of the allowed prefixes:

```bash
REFERRERS="https://example.com,https://cdn.example.com"
```

Include an empty string to allow requests without a `Referer` header:

```bash
REFERRERS=",https://example.com"  # allows empty referer + example.com
```

<br/>

## TLS/HTTPS

Both `TLS_CERT` and `TLS_KEY` must be set together:

```bash
TLS_CERT=/path/to/cert.pem TLS_KEY=/path/to/key.pem static-file-server
```

Supported minimum TLS versions: `TLS11`, `TLS12`, `TLS13`.
Default is `TLS12` when not specified.

<br/>

## URL Prefix

Route the server under a subpath:

```bash
URL_PREFIX="/static" static-file-server
# Files accessible at: http://host:8080/static/file.txt
```

Rules:
- Must start with `/`
- Must not end with `/`
- Requests without the prefix return 404

<br/>

## SPA Mode

For single-page applications (React, Vue, Angular). When enabled, non-file routes serve `/index.html` instead of 404:

```bash
SPA=true SHOW_LISTING=false static-file-server
```

Note: `SPA=true` is incompatible with `SHOW_LISTING=true`.

<br/>

## Gzip Compression

Compress text-based responses (HTML, CSS, JS, JSON, etc.) to reduce transfer size:

```bash
COMPRESSION=true static-file-server
```

Compression is automatically skipped for:
- Already compressed files (`.apk`, `.jpg`, `.mp4`, `.zip`, `.gz`, `.woff2`, etc.)
- Range requests (partial downloads / resume)
- Clients that don't send `Accept-Encoding: gzip`

<br/>

## Hidden Dot Files

Block access to dot files and directories (e.g. `.env`, `.git`, `.DS_Store`):

```bash
HIDE_DOT_FILES=true static-file-server
```

Dot files are also excluded from directory listings when enabled.

<br/>

## Structured Logging

Switch debug logs to JSON format for log aggregation (Loki, ELK, CloudWatch):

```bash
DEBUG=true LOG_FORMAT=json static-file-server
```

JSON log example:
```json
{"time":"2026-04-13T14:30:00Z","remote":"192.168.1.10:54321","method":"GET","path":"/files/app.apk","proto":"HTTP/1.1","host":"example.com","status":200,"duration_ms":42}
```

<br/>

## Prometheus Metrics

Enable a `/metrics` endpoint with Prometheus-compatible metrics:

```bash
METRICS=true static-file-server
```

Exposed metrics:
- `static_file_server_requests_total{method,status}` — request counter
- `static_file_server_response_bytes_total` — total response bytes
- `static_file_server_request_duration_seconds_bucket{le}` — latency histogram
