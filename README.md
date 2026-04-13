# static-file-server

A lightweight, zero-dependency static file server written in Go with a modern directory listing UI.

<br/>

## Features

- Modern, responsive directory listing with dark mode support
- Extension-based file icons (13 categories: image, video, audio, code, config, etc.)
- Client-side search/filter with keyboard shortcuts (`/` to focus, `Esc` to clear)
- Inline preview for images, video, and audio files
- Column sorting (name, size, modified date)
- Breadcrumb navigation
- CORS support
- TLS/HTTPS
- Access control (URL keys, referrer validation)
- URL prefix routing
- Index.html support
- Four serving modes (basic / index / listing / both)
- Debug request logging

<br/>

## Quick Start

```bash
# Build
make build

# Run (serves /web on :8080 by default)
./bin/static-file-server

# Or with environment variables
FOLDER=./public PORT=3000 ./bin/static-file-server

# Or with a config file
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
| `TLS_MIN_VERS` | string | `""` | Minimum TLS version (TLS10/TLS11/TLS12/TLS13) |
| `REFERRERS` | string | `""` | Comma-separated allowed referrer prefixes |
| `ACCESS_KEY` | string | `""` | URL parameter access key |

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
```

<br/>

## Build & Development

```bash
make help             # Show all targets
make build            # Build binary to bin/
make run              # Build and run
make test             # Run all tests with race detection
make test-unit        # Run unit tests only
make test-integration # Run integration tests only
make cover            # Generate HTML coverage report
make lint             # Run golangci-lint
make lint-fix         # Run golangci-lint with auto-fix
make fmt              # Format code
make vet              # Run go vet
make cross-build      # Build for linux/darwin amd64/arm64
make test-helm        # Helm chart lint + template tests
make version          # Show version across all files
make bump-version VERSION=v0.2.0  # Bump version in all files
```

<br/>

## Docker

```bash
make docker-build          # Build Docker image
make docker-push           # Push to registry
make docker-buildx         # Multi-arch build + push (versioned + latest)
make docker-buildx-tag     # Multi-arch build + push (versioned only)
make docker-buildx-latest  # Multi-arch build + push (latest only)
```

<br/>

## Deploy

### Local (Docker)

```bash
make deploy               # Build image + run container on :8080
make test-deploy           # Smoke test against running container
make undeploy              # Stop and remove container

# Custom port and volume
make deploy DEPLOY_PORT=3000 DEPLOY_VOLUME=/path/to/files
```

### Kubernetes

```bash
make deploy-k8s                         # Deploy to default namespace
make deploy-k8s K8S_NAMESPACE=web       # Deploy to specific namespace
make undeploy-k8s                       # Remove from cluster
make undeploy-k8s K8S_NAMESPACE=web     # Remove from specific namespace
```

Manifests are in `deploy/deployment.yaml` (Deployment + Service).

### Helm

```bash
# Install from local chart
helm install my-server ./helm/static-file-server

# With custom values
helm install my-server ./helm/static-file-server \
  --set config.cors=true \
  --set service.type=LoadBalancer

# From Helm repository
helm repo add static-file-server https://somaz94.github.io/static-file-server/helm-repo
helm repo update
helm install my-server static-file-server/static-file-server

# Uninstall
helm uninstall my-server
```

See [docs/deployment.md](docs/deployment.md) for advanced deployment options (PVC, ConfigMap, Ingress).

<br/>

## Version Management

```bash
make version                      # Show version across all files
make bump-version VERSION=v0.2.0  # Bump version in all files at once
```

See [docs/version.md](docs/version.md) for the release process.

<br/>

## Workflow

```bash
make check-gh                     # Verify gh CLI is installed and authenticated
make branch name=search-filter    # Create feature branch (feat/search-filter)
make pr title="Add search filter" # Run tests, push, and create PR
```

<br/>

## Directory Listing UI

The directory listing features a modern, responsive design:

- **Dark mode**: Automatically follows system preference
- **File icons**: 13 categories with distinct colors (folder, image, video, audio, PDF, doc, spreadsheet, slides, archive, code, config, binary, font)
- **Search**: Real-time filter with keyboard shortcut (`/` to focus, `Esc` to clear)
- **Preview**: Click image/video/audio files to preview in a modal overlay
- **Sorting**: Click column headers to sort by name, size, or date

<br/>

## Architecture

```
cmd/                    # CLI entry point (Cobra-based)
cmd/cli/                # Command definitions (root serve, version)
internal/config/        # Configuration loading (env > YAML > defaults)
internal/handler/       # HTTP middleware chain + directory listing
internal/server/        # HTTP/HTTPS server lifecycle
internal/version/       # Build version metadata (ldflags)
deploy/                 # Kubernetes manifests (Deployment + Service)
helm/                   # Helm chart
docs/                   # Documentation (configuration, deployment, version)
hack/                   # Scripts (bump-version, test-helm)
scripts/                # Utility scripts (create-pr)
testdata/               # Sample files for local deploy testing
.github/workflows/      # CI/CD (test, lint, release, helm-release)
```

<br/>

## Middleware Chain

Applied outer to inner:

1. Debug logging (optional)
2. URL prefix stripping (optional)
3. Access key verification (optional)
4. Referrer validation (optional)
5. CORS headers (optional)
6. File handler (index/listing/basic)

<br/>

## Contributing

Issues and pull requests are welcome.

<br/>

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.