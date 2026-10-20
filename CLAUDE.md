# static-file-server

## Overview

Lightweight, zero-dependency static file server written in Go with modern directory listing UI.

## Architecture

- `cmd/` - CLI entry point (Cobra-based)
- `cmd/cli/` - Command definitions (root serve, `cache` for the cache endpoint only, version)
- `internal/config/` - Configuration loading (env vars > YAML > defaults)
- `internal/handler/` - HTTP handler middleware chain + directory listing
- `internal/handler/templates/` - Embedded HTML template with icons, search, preview
- `internal/httpx/` - Helpers shared by both listeners: status/bytes `Recorder` (keeps `ReadFrom` for sendfile, `Unwrap`) and the bounded metric method label
- `internal/s3cache/` - GitLab Runner cache endpoint: SigV4 presigned GET/HEAD/PUT, filesystem store, TTL janitor
- `internal/server/` - HTTP/HTTPS server lifecycle (file server + optional S3 cache listener)
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
| `METRICS` | bool | `false` | Enable Prometheus metrics at `/metrics` (cumulative histogram with `_sum`/`_count`) |
| `S3_ENABLED` | bool | `false` | Enable the GitLab Runner cache endpoint on a separate listener |
| `S3_PORT` | uint16 | `9000` | Cache endpoint port (must differ from `PORT`) |
| `S3_FOLDER` | string | `""` | Cache object directory (required; must not be served, see below) |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | string | `""` | SigV4 credentials the runner signs with (required) |
| `S3_BUCKETS` | string | `""` | Comma-separated bucket allowlist (empty = any) |
| `S3_TTL` | duration | `0` | Expire objects neither uploaded nor read for this long (`0` = never) |
| `S3_MAX_OBJECT_SIZE` | int | `0` | Max upload bytes (`0` = unlimited) |
| `S3_MAX_TOTAL_SIZE` | int | `0` | Total size cap; the janitor evicts least recently used first (`0` = unlimited) |

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

## JSON Listing API

- Directory listings can be returned as JSON via `?format=json` query or `Accept: application/json` header
- A JSON request bypasses `index.html` (returns the listing even when an index page exists)
- Honors `HIDE_DOT_FILES`; entries sorted directories-first then alphabetically (same as HTML)
- Response: `{ path, entries[], total_files, total_dirs, total_size, total_size_bytes }`

## GitLab Runner Cache Endpoint

- `internal/s3cache` serves only what the runner cache uses: SigV4 query-presigned GET (with Range), HEAD and PUT on path-style URLs. Everything else is `501`. Not a general S3 server; do not grow it into one.
- Separate `http.Server` on `S3_PORT`, none of the file-server middleware. `HEAD` signed for `GET` is accepted on purpose (the runner's cache extractor probes that way).
- Layout: `<S3_FOLDER>/<bucket>/<key>`, metadata sidecar in `.meta/`, uploads staged in `.tmp/` then renamed.
- Janitor (every 10 min, or the TTL if shorter): TTL expiry, then least-recently-used eviction above `S3_MAX_TOTAL_SIZE`, then gauge update. Last use = the later of the data file mtime (upload) and the `.meta` sidecar mtime, which a `GET` / `HEAD` bumps at most hourly. The runner skips re-uploading an unchanged cache, so upload time alone would expire caches in daily use. An object another replica already deleted counts as freed during eviction. With `METRICS=true` the cache listener serves its own `/metrics` (`static_file_server_s3_*`).
- Config validation rejects an `S3_FOLDER` inside `FOLDER` unless it is under a dot directory with `HIDE_DOT_FILES=true` (cache archives can hold secrets). The Helm chart enforces the same for a shared PVC.
- Request shapes were checked against the gitlab-runner `cache/s3`, `cache/s3v2` and `commands/helpers/cache_*` sources; re-check them there when the runner changes its cache client.
- `static-file-server cache` (chart `mode: cache`) runs only this listener, for a cache deployment separate from a public file server; it skips the FOLDER/PORT separation checks since neither is used.
- Docs: `docs/s3-cache.md`.

<br/>

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

CI fails below 90% total coverage; `make cover` shows the current per-package figures. `make test-helm` runs in CI too.

## CI/CD Workflows

- `test.yml` - Run Go tests (90% coverage gate) and Helm chart tests on push to main / PRs
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
- Security: non-root, no privilege escalation, read-only rootfs, seccomp `RuntimeDefault`, no service account token
- Scheduling (opt-in): PodDisruptionBudget, topologySpreadConstraints, update strategy
- Customizable probes, resources, env vars
- Prometheus ServiceMonitor for both listeners (optional, needs `config.metrics`)
- Helm test (wget connectivity check)
