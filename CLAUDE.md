# static-file-server

## Overview

Lightweight, zero-dependency static file server written in Go.
Feature-compatible with halverneus/static-file-server with improved directory listing UI.

## Architecture

- `cmd/` - CLI entry point (Cobra-based)
- `cmd/cli/` - Command definitions (root serve, version)
- `internal/config/` - Configuration loading (env vars > YAML > defaults)
- `internal/handler/` - HTTP handler middleware chain + directory listing
- `internal/handler/templates/` - Embedded HTML template with icons, search, preview
- `internal/server/` - HTTP/HTTPS server lifecycle
- `internal/version/` - Build version metadata injected via ldflags

## Build & Run

```bash
make help             # Show all available targets
make build            # Build binary to bin/
make test             # Run all tests with race detection
make test-unit        # Unit tests (internal packages)
make test-integration # Integration tests (end-to-end HTTP)
make run              # Build and run
make install          # Install to /usr/local/bin
make cover            # Generate HTML coverage report
make lint             # Run golangci-lint
make lint-fix         # Run golangci-lint with auto-fix
make fmt              # Format code
make vet              # Run go vet
make cross-build      # Cross-compile for multiple platforms
make version          # Print version info
make docker-build     # Build Docker image
make docker-buildx    # Multi-arch Docker build + push
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

## Directory Listing UI Features

- Extension-based file icons (13 categories: image, video, audio, pdf, doc, sheet, slide, archive, code, config, binary, font, file)
- Client-side search/filter (keyboard: `/` to focus, `Esc` to clear)
- Inline preview modal for images, video, and audio
- Column sorting (name, size, modified date)
- Breadcrumb navigation
- Dark mode (follows system preference)
- Responsive design (mobile-friendly)

## Testing

```bash
make test             # All tests
make test-unit        # Internal packages only
make test-integration # Integration (end-to-end HTTP) tests only
make cover            # HTML coverage report
```

Test coverage:
- `internal/version` - 100%
- `internal/handler` - ~83%
- `internal/config` - ~68%
- `internal/server` - ~29%
- Integration tests: 12 end-to-end scenarios
