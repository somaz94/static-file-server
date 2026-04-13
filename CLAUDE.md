# static-file-server

## Overview

Lightweight, zero-dependency static file server written in Go.
Feature-compatible with halverneus/static-file-server with improved directory listing UI.

## Architecture

- `cmd/` - CLI entry point (Cobra-based)
- `cmd/cli/` - Command definitions (root serve, version)
- `internal/config/` - Configuration loading (env vars > YAML > defaults)
- `internal/handler/` - HTTP handler middleware chain + directory listing
- `internal/server/` - HTTP/HTTPS server lifecycle
- `internal/version/` - Build version metadata injected via ldflags

## Build & Run

```bash
make build        # Build binary to bin/
make test         # Run all tests with race detection
make run          # Build and run
make install      # Install to /usr/local/bin
make cover        # Generate HTML coverage report
make fmt          # Format code
make vet          # Run go vet
```

## Configuration Priority

1. Environment variables (highest priority)
2. YAML config file (`-c` / `--config` flag)
3. Default values (lowest priority)

## Environment Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `CORS` | bool | `false` | Enable CORS headers for all responses |
| `DEBUG` | bool | `false` | Enable debug logging (config summary + request log) |
| `HOST` | string | `""` | Hostname to bind (empty = all interfaces) |
| `PORT` | uint16 | `8080` | Port number |
| `FOLDER` | string | `/web` | Root folder to serve |
| `ALLOW_INDEX` | bool | `true` | Serve index.html for directory requests |
| `SHOW_LISTING` | bool | `true` | Show directory listing when no index.html |
| `URL_PREFIX` | string | `""` | URL path prefix (e.g. `/my/prefix`) |
| `TLS_CERT` | string | `""` | TLS certificate file path |
| `TLS_KEY` | string | `""` | TLS private key file path |
| `TLS_MIN_VERS` | string | `""` | Minimum TLS version (TLS10/TLS11/TLS12/TLS13) |
| `REFERRERS` | string | `""` | Comma-separated allowed referrer prefixes |
| `ACCESS_KEY` | string | `""` | URL parameter access key |

## Middleware Chain (outer to inner)

1. Debug logging
2. URL prefix stripping
3. Access key verification
4. Referrer validation
5. CORS headers
6. File handler (index/listing/basic)

## Testing

```bash
make test         # All tests
make test-unit    # Internal packages only
make cover        # HTML coverage report
```
