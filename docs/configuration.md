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

# MD5 code (case-insensitive)
GET /file.txt?code=<MD5("/file.txt" + "my-secret-key")>
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

Supported minimum TLS versions: `TLS10`, `TLS11`, `TLS12`, `TLS13`.
Default is `TLS10` when not specified.

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
