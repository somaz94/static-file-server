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
- `deploy/` - Kubernetes manifests (Deployment + Service) + Helmfile examples
- `helm/static-file-server/` - Helm chart (deployment, service, ingress, PVC, configmap)
- `docs/` - Documentation (configuration, deployment, version/release)
- `hack/` - Build/version scripts (bump-version.sh, test-helm.sh)
- `scripts/` - Utility scripts (create-pr.sh)
- `testdata/` - Sample files for local deploy testing
- `.github/workflows/` - CI/CD pipelines (test, lint, release, helm-release, etc.)
- `.github/dependabot.yml` - Dependabot config (docker, actions, gomod)

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
make docker-build     # Build Docker image
make docker-buildx    # Multi-arch Docker build + push
make deploy           # Docker: build + run container on :8080
make test-deploy      # Smoke test against running container
make undeploy         # Docker: stop + remove container
make deploy-k8s       # Kubernetes: apply deployment + service
make undeploy-k8s     # Kubernetes: delete deployment + service
make install          # Install binary to /usr/local/bin
make uninstall        # Remove binary from /usr/local/bin
make version          # Show version across all files
make bump-version VERSION=v0.2.0  # Bump version everywhere
make test-helm        # Helm chart lint + template tests
make check-gh         # Verify gh CLI auth
make branch name=foo  # Create feature branch
make pr title="..."   # Test + push + create PR
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
| `CUSTOM_HEADERS` | string | `""` | Comma-separated `Key:Value` custom response headers |

## Middleware Chain (outer to inner)

1. Debug logging
2. URL prefix stripping
3. Access key verification
4. Referrer validation
5. CORS headers
6. Custom response headers
7. File handler (index/listing/basic)

## Directory Listing UI Features

- Extension-based file icons (13 categories: image, video, audio, pdf, doc, sheet, slide, archive, code, config, binary, font, file)
- Client-side search/filter (keyboard: `/` to focus, `Esc` to clear)
- Inline preview modal for images, video, and audio
- Column sorting (name, size, modified date)
- Breadcrumb navigation
- Dark/light mode toggle (manual switch + system preference detection)
- File stats in footer (total files, directories, combined size)
- Version display in footer
- Responsive design (mobile-friendly)

## Health Check

- `/healthz` endpoint returns `200 OK` with body `ok`
- Bypasses all middleware (no auth, no CORS, no logging)

## Testing

```bash
make test             # All tests
make test-unit        # Internal packages only
make test-integration # Integration (end-to-end HTTP) tests only
make cover            # HTML coverage report
```

Test coverage (93.5% total):
- `internal/version` - 100%
- `internal/server` - 100%
- `internal/config` - 98.9%
- `internal/handler` - 90.3%
- Integration tests: 12 end-to-end scenarios

## CI/CD Workflows

- `test.yml` - Run tests on push to main / PRs (90% coverage gate)
- `lint.yml` - golangci-lint (manual dispatch)
- `release.yml` - Docker build + push + GitHub release on tags
- `helm-release.yml` - Package and publish Helm chart on tags
- `changelog-generator.yml` - Auto-generate CHANGELOG.md
- `contributors.yml` - Auto-generate CONTRIBUTORS.md
- `stale-issues.yml` - Close stale issues after 30 days
- `dependabot-auto-merge.yml` - Auto-merge minor/patch dependabot PRs
- `issue-greeting.yml` - Greet new issue authors

## Helm Chart

Location: `helm/static-file-server/`

Key features:
- ServiceAccount, Deployment, Service, Ingress (optional)
- PersistentVolumeClaim for file storage (optional)
- ConfigMap-based content for small static sites (optional)
- Security: non-root, no privilege escalation, read-only rootfs
- Customizable probes, resources, env vars
- Helm test (wget connectivity check)
