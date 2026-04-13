# static-file-server

![Top Language](https://img.shields.io/github/languages/top/somaz94/static-file-server?color=green&logo=go&logoColor=b)
![static-file-server](https://img.shields.io/github/v/tag/somaz94/static-file-server?label=static-file-server&logo=go&logoColor=white)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/somaz94/static-file-server)](https://goreportcard.com/report/github.com/somaz94/static-file-server)
![Docker Pulls](https://img.shields.io/docker/pulls/somaz940/static-file-server?logo=docker&logoColor=white)
![GitHub Stars](https://img.shields.io/github/stars/somaz94/static-file-server?style=social)

A lightweight, zero-dependency static file server written in Go with a modern directory listing UI.

<br/>

## Features

![Directory Listing](https://img.shields.io/badge/Directory_Listing-blue?logo=files&logoColor=white)
![Dark Mode](https://img.shields.io/badge/Dark_Mode-blue?logo=files&logoColor=white)
![File Preview](https://img.shields.io/badge/File_Preview-green?logo=files&logoColor=white)
![Search Filter](https://img.shields.io/badge/Search_Filter-green?logo=files&logoColor=white)
![Grid View](https://img.shields.io/badge/Grid_View-green?logo=files&logoColor=white)
![Batch Download](https://img.shields.io/badge/Batch_Download-green?logo=files&logoColor=white)
![CORS](https://img.shields.io/badge/CORS-orange?logo=files&logoColor=white)
![TLS](https://img.shields.io/badge/TLS%2FHTTPS-orange?logo=files&logoColor=white)
![Helm](https://img.shields.io/badge/Helm_Chart-0F1689?logo=helm&logoColor=white)
![Health Check](https://img.shields.io/badge/Health_Check-green?logo=files&logoColor=white)
![Custom Headers](https://img.shields.io/badge/Custom_Headers-orange?logo=files&logoColor=white)
![SPA Mode](https://img.shields.io/badge/SPA_Mode-purple?logo=files&logoColor=white)
![Gzip](https://img.shields.io/badge/Gzip_Compression-purple?logo=files&logoColor=white)
![Metrics](https://img.shields.io/badge/Prometheus_Metrics-purple?logo=files&logoColor=white)

- Modern, responsive directory listing with dark mode support
- Dark/light mode toggle (manual switch + system preference detection)
- Grid/list view toggle with image thumbnail cards (`g` to switch)
- Extension-based file icons (13 categories: image, video, audio, code, config, etc.)
- File extension badges (`.go`, `.py`, `.tsx`, etc.)
- Client-side search/filter with keyboard shortcuts (`/` to focus, `Esc` to clear)
- Search text highlight and file type filter chips with counts
- URL hash state for shareable search/filter links
- Inline preview for images, video, audio, PDF, and text/code files
- Preview gallery navigation (prev/next with `←` `→` keys)
- Image preview zoom (click to toggle, scroll wheel to scale)
- Text/code preview with line numbers
- Download button in preview modal
- Column sorting with localStorage persistence
- Relative time display ("3h ago") with absolute time tooltip
- Breadcrumb navigation with current page indicator
- Keyboard navigation (arrow keys, Enter, Space to select, `?` for help)
- Multi-file selection with batch ZIP download
- Copy file path to clipboard (hover to reveal)
- Scroll to top button for long listings
- Row fade-in animation on page load
- File stats in footer (total files, directories, combined size)
- Version display in footer
- `/healthz` health check endpoint (bypasses all middleware)
- Custom response headers (`CUSTOM_HEADERS`)
- CORS support
- TLS/HTTPS with configurable minimum version
- Access control (URL keys, referrer validation)
- URL prefix routing
- Four serving modes (basic / index / listing / both)
- SPA mode for single-page applications (React, Vue, Angular)
- Gzip compression (auto-skips binary files and Range requests)
- Prometheus-compatible metrics endpoint (`/metrics`)
- JSON structured logging for log aggregation
- Dot file filtering (`.env`, `.git`, etc.)
- Debug request logging with status code and response time

<br/>

## Installation

<br/>

### Prerequisites
- Go 1.26+ (for building from source)
- Docker (optional, for container deployment)
- Kubernetes v1.16+ (optional, for K8s/Helm deployment)

<br/>

### Option 1: Helm (Recommended)

```bash
# Add the Helm repository
helm repo add static-file-server https://somaz94.github.io/static-file-server/helm-repo
helm repo update

# Install with default values
helm install my-server static-file-server/static-file-server

# Or install with custom values
helm install my-server static-file-server/static-file-server \
  --set config.cors=true \
  --set persistence.enabled=true
```

For full Helm chart options, see [Helm Chart Documentation](docs/deployment.md#kubernetes-helm).

<br/>

### Option 2: Docker

```bash
docker run -d \
  --name static-file-server \
  -p 8080:8080 \
  -v /path/to/files:/web:ro \
  somaz940/static-file-server:v0.4.0
```

<br/>

### Option 3: Build from Source

```bash
git clone https://github.com/somaz94/static-file-server.git
cd static-file-server
make build
./bin/static-file-server
```

<br/>

## Quick Start

```bash
# With environment variables
FOLDER=./public PORT=3000 CORS=true ./bin/static-file-server

# With a config file
./bin/static-file-server -c config.yaml
```

<br/>

## Configuration

### Priority

1. Environment variables (highest)
2. YAML config file (`-c` / `--config` flag)
3. Default values (lowest)

### Environment Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `CORS` | bool | `false` | Enable CORS headers |
| `DEBUG` | bool | `false` | Enable debug logging |
| `HOST` | string | `""` | Hostname to bind |
| `PORT` | uint16 | `8080` | Port number |
| `FOLDER` | string | `/web` | Root folder to serve |
| `ALLOW_INDEX` | bool | `true` | Serve index.html for directories |
| `SHOW_LISTING` | bool | `true` | Show directory listing |
| `URL_PREFIX` | string | `""` | URL path prefix (e.g. `/my/prefix`) |
| `TLS_CERT` | string | `""` | TLS certificate file path |
| `TLS_KEY` | string | `""` | TLS private key file path |
| `TLS_MIN_VERS` | string | `""` | Minimum TLS version (TLS12/TLS13, default: TLS12) |
| `REFERRERS` | string | `""` | Comma-separated allowed referrer prefixes |
| `ACCESS_KEY` | string | `""` | URL parameter access key |
| `CUSTOM_HEADERS` | string | `""` | Comma-separated `Key:Value` response headers |
| `SPA` | bool | `false` | SPA mode: serve `index.html` for non-file routes |
| `COMPRESSION` | bool | `false` | Enable gzip compression |
| `HIDE_DOT_FILES` | bool | `false` | Hide dot files from serving and listings |
| `LOG_FORMAT` | string | `text` | Log format: `text` or `json` |
| `METRICS` | bool | `false` | Enable Prometheus metrics at `/metrics` |

For detailed configuration, see [Configuration Guide](docs/configuration.md).

### YAML Config Example

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
access-key: "my-secret-key"
custom-headers:
  X-Frame-Options: "DENY"
  Cache-Control: "public, max-age=3600"
compression: true
hide-dot-files: true
log-format: "json"
metrics: true
```

<br/>

## Directory Listing UI

The directory listing features a modern, responsive design:

- **Dark mode**: Automatically follows system preference, manual toggle
- **Grid/List view**: Toggle between table and card layout (`g` key), image thumbnails in grid mode
- **File icons**: 13 categories with distinct colors + file extension badges
- **Search**: Real-time filter with highlight (`/` to focus, `Esc` to clear), URL hash for shareable links
- **Filter chips**: Category filters (All, Folders, Images, Code, etc.) with file count badges
- **Preview**: Image/video/audio/PDF/text/code files with gallery navigation (`←` `→`), zoom, line numbers, download
- **Sorting**: Click column headers to sort by name, size, or date (persisted in localStorage)
- **Keyboard navigation**: Arrow keys to move, Enter to open, Space to select, `?` for shortcuts help
- **Multi-select**: Checkbox selection with batch ZIP download
- **Accessibility**: ARIA attributes, focus trap, keyboard-only navigation support
- **Copy path**: Hover to reveal clipboard copy button
- **Relative time**: "3h ago" display with absolute time tooltip
- **Animations**: Row fade-in on load, sort arrow transitions

<br/>

## Deploy

### Local (Docker)

```bash
make deploy               # Build image + run container on :8080
make test-deploy           # Smoke test against running container
make undeploy              # Stop and remove container
```

### Kubernetes (kubectl)

```bash
make deploy-k8s                         # Deploy to default namespace
make undeploy-k8s                       # Remove from cluster
```

### Helm Storage Examples

| Example | File | Description |
|---------|------|-------------|
| Dynamic provisioning | [dynamic-provisioning.yaml](helm/static-file-server/examples/dynamic-provisioning.yaml) | StorageClass auto-creates PV |
| NFS static | [nfs-static.yaml](helm/static-file-server/examples/nfs-static.yaml) | Manual NFS PV + PVC |
| HostPath | [hostpath-static.yaml](helm/static-file-server/examples/hostpath-static.yaml) | Single-node / development |
| AWS EBS CSI | [csi-ebs.yaml](helm/static-file-server/examples/csi-ebs.yaml) | AWS EBS volume |
| ConfigMap site | [configmap-site.yaml](helm/static-file-server/examples/configmap-site.yaml) | Small static site from ConfigMap |
| Ingress + TLS | [ingress-tls.yaml](helm/static-file-server/examples/ingress-tls.yaml) | cert-manager TLS termination |
| Multi-volume | [multi-volume.yaml](helm/static-file-server/examples/multi-volume.yaml) | NFS + extra ConfigMap volume |

For full deployment instructions, see [Deployment Guide](docs/deployment.md).

<br/>

## Version Management

```bash
make version                      # Show version across all files
make bump-version VERSION=v0.4.0  # Bump version in all files at once
```

See [Version Guide](docs/version.md) for the release process.

<br/>

## Development

```bash
make help             # Show all targets
make build            # Build binary
make test             # Run all tests (93.5% coverage)
make test-unit        # Unit tests only
make test-integration # Integration tests only
make test-helm        # Helm chart lint + template tests (15 scenarios)
make cover            # HTML coverage report
make lint             # Run golangci-lint
make fmt              # Format code
make vet              # Run go vet
make cross-build      # Build for linux/darwin amd64/arm64
```

<br/>

## Workflow

```bash
make check-gh                     # Verify gh CLI
make branch name=search-filter    # Create feature branch
make pr title="Add search filter" # Test + push + create PR
```

<br/>

## Architecture

```
cmd/                    # CLI entry point (Cobra-based)
cmd/cli/                # Command definitions (root serve, version)
internal/config/        # Configuration loading (env > YAML > defaults)
internal/handler/       # HTTP middleware chain + directory listing
internal/server/        # HTTP/HTTPS server lifecycle
internal/version/       # Build version metadata (ldflags)
deploy/                 # Kubernetes manifests + Helmfile examples
helm/                   # Helm chart (7 templates + 7 examples)
docs/                   # Documentation
hack/                   # Build/version scripts
scripts/                # Utility scripts
testdata/               # Sample files for local deploy testing
.github/workflows/      # CI/CD (9 workflows)
```

### Middleware Chain

Applied outer to inner:

1. Prometheus metrics (optional)
2. Health check (`/healthz`, bypasses all middleware)
3. Debug logging with status/duration (optional, text or JSON)
4. URL prefix stripping (optional)
5. Access key verification (optional)
6. Referrer validation (optional)
7. CORS headers (optional)
8. Custom response headers (optional)
9. Gzip compression (optional, skips binary/Range)
10. Dot file filtering (optional)
11. File handler (SPA / index / listing / basic)

<br/>

## Documentation

| Document | Description |
|----------|-------------|
| [Configuration Guide](docs/configuration.md) | Environment variables, YAML config, serving modes, access control |
| [Deployment Guide](docs/deployment.md) | Binary, Docker, Kubernetes, Helm (with storage examples) |
| [Testing Guide](docs/test.md) | Unit tests, integration tests, Helm tests, deployment smoke tests, coverage |
| [Version Guide](docs/version.md) | Version management, bump process, release workflow |

<br/>

## Contributing

Issues and pull requests are welcome.

<br/>

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
