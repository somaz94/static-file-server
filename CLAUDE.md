# static-file-server

## Overview

Lightweight, zero-dependency static file server written in Go with modern directory listing UI.

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
make deploy           # Build binary + run local server on :8080
make deploy-smoke     # Smoke test against running server (43 checks)
make deploy-all       # Build + run + smoke test (all-in-one)
make undeploy         # Stop local server
make deploy-docker    # Run as Docker container (pulls if needed)
make undeploy-docker  # Stop and remove Docker container
make deploy-k8s       # Kubernetes: apply deployment + service
make undeploy-k8s     # Kubernetes: delete deployment + service
make install          # Install binary to /usr/local/bin
make uninstall        # Remove binary from /usr/local/bin
make version          # Show version across all files
make bump-version VERSION=v0.4.0  # Bump version everywhere
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
| `TLS_MIN_VERS` | string | `""` | Minimum TLS version (TLS12/TLS13, default: TLS12) |
| `REFERRERS` | string | `""` | Comma-separated allowed referrer prefixes |
| `ACCESS_KEY` | string | `""` | URL parameter access key (SHA-256 code support) |
| `CUSTOM_HEADERS` | string | `""` | Comma-separated `Key:Value` custom response headers |
| `SPA` | bool | `false` | SPA mode: serve index.html for non-file routes |
| `COMPRESSION` | bool | `false` | Enable gzip compression (skips binary/Range) |
| `HIDE_DOT_FILES` | bool | `false` | Hide dot files from serving and listings |
| `LOG_FORMAT` | string | `text` | Log format: `text` or `json` |
| `METRICS` | bool | `false` | Enable Prometheus metrics at `/metrics` |

## Middleware Chain (outer to inner)

1. Prometheus metrics (optional)
2. Health check (`/healthz`, bypasses all middleware)
3. Debug logging with status/duration (optional, text or JSON)
4. URL prefix stripping
5. Access key verification (SHA-256)
6. Referrer validation
7. CORS headers
8. Custom response headers
9. Gzip compression (optional)
10. Dot file filtering (optional)
11. File handler (SPA / index / listing / basic)

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
