# Testing Guide

<br/>

## Quick Start

```bash
make test           # Run all tests
make test-unit      # Unit tests only (internal packages)
make test-helm      # Helm chart lint + template render tests
```

<br/>

## Unit Tests

Run unit tests with race detection and coverage:

```bash
make test-unit
```

Coverage by package:

| Package | Coverage | Description |
|---------|----------|-------------|
| `internal/version` | 100% | Build version metadata |
| `internal/server` | 100% | HTTP/HTTPS server lifecycle, TLS version parsing |
| `internal/config` | 98.9% | Config loading (env, YAML, defaults), validation |
| `internal/handler` | 90.3% | Middleware chain, file serving, directory listing |
| **Total** | **93.5%** | |

<br/>

## Integration Tests

End-to-end HTTP tests using `httptest.NewServer`:

```bash
make test-integration
```

12 scenarios tested:

| Test | Description |
|------|-------------|
| `ListingAndIndex` | Root index.html served, subdirectory listing fallback |
| `StaticFile` | File serving with correct Content-Type |
| `NestedFile` | Files in nested directories |
| `NotFound` | 404 for missing files |
| `CORS` | CORS headers present when enabled |
| `URLPrefix` | Prefix stripping, 404 without prefix |
| `AccessKey` | Direct key + reject wrong key |
| `ReferrerValidation` | Valid/invalid referrer handling |
| `ListingOnly` | Listing mode (no index.html preference) |
| `BasicOnly` | Files only, directories return 404 |
| `DirectoryTraversal` | Path traversal prevention |
| `ListingContainsExtIcons` | Icon classes, search bar, preview modal in listing HTML |

<br/>

## Helm Chart Tests

Lint and template render verification:

```bash
make test-helm
```

15 scenarios tested:

| Scenario | Description |
|----------|-------------|
| Lint | Chart structure and syntax validation |
| Default values | Renders with no overrides |
| Ingress enabled | Ingress resource created correctly |
| Dynamic provisioning | PVC with StorageClass |
| Static NFS | PV + PVC with NFS volume source |
| Static hostPath | PV + PVC with hostPath volume source |
| ConfigMap content | ConfigMap-based static site |
| Full options | All features enabled simultaneously |
| Example: configmap-site | ConfigMap small site example |
| Example: csi-ebs | AWS EBS CSI example |
| Example: dynamic-provisioning | Dynamic PVC example |
| Example: hostpath-static | HostPath development example |
| Example: ingress-tls | Ingress with cert-manager TLS |
| Example: multi-volume | Multiple volumes + extraVolumes |
| Example: nfs-static | NFS static provisioning example |

<br/>

## Coverage Report

Generate an HTML coverage report:

```bash
make cover
open coverage.html
```

<br/>

## Coverage Threshold

CI enforces a 90% minimum coverage gate on the `internal/` packages.
See `.github/workflows/test.yml` for details.

<br/>

## Deployment Smoke Tests

After deploying to a live environment, verify the server is working correctly with these manual checks.

<br/>

### Health Check

```bash
# Verify the server is responding
curl -sI http://<YOUR_HOST>/
# Expected: HTTP/1.1 200 OK
```

<br/>

### CORS Verification

```bash
# Check CORS headers are present
curl -sI http://<YOUR_HOST>/ | grep -i "access-control"
# Expected:
#   Access-Control-Allow-Headers: *
#   Access-Control-Allow-Origin: *
#   Cross-Origin-Resource-Policy: cross-origin

# Test CORS preflight (OPTIONS request)
curl -sI -X OPTIONS \
  -H "Origin: http://example.com" \
  -H "Access-Control-Request-Method: GET" \
  http://<YOUR_HOST>/
# Expected: HTTP/1.1 200 OK with CORS headers
```

<br/>

### File Download Verification

```bash
# Verify directory listing returns HTML with file entries
curl -s http://<YOUR_HOST>/path/to/files/ | grep -o '<a href="[^"]*">'
# Expected: list of file links

# Verify file download with correct Content-Type (e.g. APK)
curl -sI http://<YOUR_HOST>/path/to/files/example.apk
# Expected:
#   HTTP/1.1 200 OK
#   Content-Type: application/vnd.android.package-archive
#   Content-Length: <file size in bytes>
#   Accept-Ranges: bytes

# Verify partial download support (Range header)
curl -sI -H "Range: bytes=0-1023" http://<YOUR_HOST>/path/to/files/example.apk
# Expected: HTTP/1.1 206 Partial Content
```

<br/>

### Checklist

| Check | Command | Expected |
|-------|---------|----------|
| Server responds | `curl -sI <host>/` | `200 OK` |
| CORS headers | `curl -sI <host>/ \| grep Access-Control` | `Allow-Origin: *` |
| Directory listing | `curl -s <host>/path/` | HTML with file links |
| File Content-Type | `curl -sI <host>/file.apk` | `application/vnd.android.package-archive` |
| Range support | `curl -sI -H "Range: bytes=0-1023" <host>/file` | `206 Partial Content` |

<br/>

## Running Specific Tests

```bash
# Run a specific test by name
go test ./internal/handler/ -run TestRenderListing -v

# Run all tests matching a pattern
go test ./... -run TestIntegration -v

# Run with verbose output and race detection
go test ./internal/... -v -race -cover
```
