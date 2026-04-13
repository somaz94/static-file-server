# CORS & Custom Headers Testing Guide

<br/>

## CORS

When `CORS=true` (or `config.cors: "true"` in Helm values), the server adds these headers to every response:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Headers: *
Cross-Origin-Resource-Policy: cross-origin
```

<br/>

### Test CORS Headers

```bash
# Basic check - look for Access-Control headers
curl -v http://your-server:8080/ 2>&1 | grep -i "access-control"

# Simulate cross-origin request
curl -H "Origin: https://other-domain.com" \
     -v http://your-server:8080/ 2>&1 | grep -i "access-control"

# Preflight (OPTIONS) request
curl -X OPTIONS \
     -H "Origin: https://other-domain.com" \
     -H "Access-Control-Request-Method: GET" \
     -v http://your-server:8080/ 2>&1 | grep -i "access-control"
```

Expected output:
```
< Access-Control-Allow-Origin: *
< Access-Control-Allow-Headers: *
< Cross-Origin-Resource-Policy: cross-origin
```

<br/>

### Test CORS from Browser

Open browser DevTools (F12) → Console:

```javascript
// From any website, fetch a file from your server
fetch('http://your-server:8080/some-file.txt')
  .then(r => r.text())
  .then(t => console.log('OK:', t))
  .catch(e => console.error('CORS blocked:', e));
```

If CORS is enabled, the fetch succeeds. If disabled, browser blocks it with a CORS error.

<br/>

### Test CORS Disabled

```bash
# With CORS=false, these headers should NOT appear
curl -v http://your-server:8080/ 2>&1 | grep -i "access-control"
# (no output = correct)
```

<br/>

## Custom Response Headers

Custom headers allow adding arbitrary HTTP response headers to all responses.
This is useful for:
- Setting custom `Content-Type` for specific file extensions (e.g., `.apk`)
- Adding security headers (`X-Frame-Options`, `X-Content-Type-Options`)
- Cache control (`Cache-Control`, `Expires`)

<br/>

### Configuration

#### Environment Variable

Comma-separated `Key:Value` pairs:

```bash
CUSTOM_HEADERS="X-Custom-Header:my-value,X-Frame-Options:DENY"
```

#### YAML Config

```yaml
custom-headers:
  X-Custom-Header: "my-value"
  X-Frame-Options: "DENY"
  X-Content-Type-Options: "nosniff"
  Cache-Control: "public, max-age=3600"
```

#### Helm Values

```yaml
config:
  # ... other config ...

extraEnv:
  - name: CUSTOM_HEADERS
    value: "X-Custom-Header:my-value,X-Frame-Options:DENY"
```

<br/>

### Test Custom Headers

```bash
# Set custom headers via env
CUSTOM_HEADERS="X-Test:hello,X-Frame-Options:DENY" \
  FOLDER=./testdata PORT=9090 ./bin/static-file-server &

# Verify headers appear
curl -v http://localhost:9090/ 2>&1 | grep -i "x-test\|x-frame"
```

Expected output:
```
< X-Test: hello
< X-Frame-Options: DENY
```

<br/>

### Real-World Example: APK Content-Type

Previously required nginx `configuration-snippet`:
```yaml
# BEFORE (nginx workaround)
nginx.ingress.kubernetes.io/configuration-snippet: |
  if ($request_filename ~* \.apk$) {
    more_set_headers "Content-Type: application/vnd.android.package-archive";
  }
```

Now handled at app level:
```yaml
# AFTER (app-level custom header)
custom-headers:
  X-Content-Type-Options: "nosniff"
```

Note: For file-extension-specific Content-Type, Go's `http.ServeFile` already detects most MIME types correctly. The custom headers feature is for adding *extra* headers on top, not for overriding Content-Type per extension.

<br/>

## Health Endpoint

The `/healthz` endpoint returns `200 OK` with body `ok`. It bypasses all middleware (no auth, no CORS, no logging).

```bash
# Test health endpoint
curl -v http://your-server:8080/healthz
```

Expected:
```
< HTTP/1.1 200 OK
< Content-Type: text/plain
ok
```

Use this for Kubernetes probes instead of `/`:

```yaml
# Helm values
probes:
  liveness:
    path: /healthz    # lightweight, no HTML rendering
    port: http
  readiness:
    path: /healthz
    port: http
```

<br/>

## Combined Test Script

```bash
#!/bin/bash
SERVER="http://your-server:8080"

echo "=== Health Check ==="
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$SERVER/healthz")
echo "  /healthz: $STATUS (expect 200)"

echo ""
echo "=== CORS Headers ==="
curl -s -D- -o /dev/null "$SERVER/" | grep -i "access-control" | sed 's/^/  /'

echo ""
echo "=== Custom Headers ==="
curl -s -D- -o /dev/null "$SERVER/" | grep -i "x-" | sed 's/^/  /'

echo ""
echo "=== Done ==="
```
