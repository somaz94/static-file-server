#!/usr/bin/env bash
set -euo pipefail

# test-deploy.sh - Smoke test against a running static-file-server container
#
# Usage:
#   ./hack/test-deploy.sh [PORT]
#   make deploy-smoke
#   make deploy-all    # deploy + smoke in one step

PORT="${1:-8080}"
BASE="http://localhost:${PORT}"

PASS=0
FAIL=0

check() {
    local desc="$1" result="$2"
    if [ "$result" = "true" ]; then
        echo "  ✓ ${desc}"
        PASS=$((PASS + 1))
    else
        echo "  ✗ ${desc}"
        FAIL=$((FAIL + 1))
    fi
}

body_contains() {
    grep -q "$1" <<< "$BODY" && echo "true" || echo "false"
}

echo "=== Smoke Test: ${BASE} ==="
echo ""

# ---------------------------------------------------------------
# 1. Wait for server
# ---------------------------------------------------------------
echo "[1/8] Server health..."
for i in 1 2 3 4 5; do
    STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/healthz" 2>/dev/null) || true
    if [ "$STATUS" = "200" ]; then break; fi
    if [ "$i" = "5" ]; then
        echo "  ✗ Server not responding after 5 attempts"
        exit 1
    fi
    sleep 1
done
check "GET /healthz => 200" "true"

# Fetch root directory listing
BODY=$(curl -s "${BASE}/")

# ---------------------------------------------------------------
# 2. HTML structure & accessibility
# ---------------------------------------------------------------
echo "[2/8] HTML structure & accessibility..."
check "<main> landmark"           "$(body_contains '<main')"
check "Breadcrumb aria-label"     "$(body_contains 'aria-label="Breadcrumb"')"
check "Current page indicator"    "$(body_contains 'aria-current="page"')"
check "Table aria-sort"           "$(body_contains 'aria-sort=')"
check "Preview role=dialog"       "$(body_contains 'role="dialog"')"
check "Search aria-live"          "$(body_contains 'aria-live="polite"')"

# ---------------------------------------------------------------
# 3. UI components
# ---------------------------------------------------------------
echo "[3/8] UI components..."
check "Filter chips"              "$(body_contains 'id="filterChips"')"
check "Grid/list toggle"          "$(body_contains 'id="viewToggle"')"
check "Grid container"            "$(body_contains 'id="gridContainer"')"
check "Scroll to top button"      "$(body_contains 'id="scrollTop"')"
check "Keyboard shortcuts help"   "$(body_contains 'id="helpOverlay"')"
check "Selection bar"             "$(body_contains 'id="selectionBar"')"
check "Empty state message"       "$(body_contains 'id="emptyState"')"
check "Sticky table header"       "$(body_contains 'position: sticky')"
check "Row fade-in animation"     "$(body_contains 'fadeIn')"

# ---------------------------------------------------------------
# 4. File type rendering
# ---------------------------------------------------------------
echo "[4/8] File type rendering..."
check "Directory icons"           "$(body_contains 'icon-dir')"
check "Code file icons"           "$(body_contains 'icon-code')"
check "Image file icons"          "$(body_contains 'icon-image')"
check "Config file icons"         "$(body_contains 'icon-config')"
check "Archive file icons"        "$(body_contains 'icon-archive')"
check "Doc file icons"            "$(body_contains 'icon-doc')"
check "File extension badges"     "$(body_contains 'ext-badge')"

# ---------------------------------------------------------------
# 5. Preview features
# ---------------------------------------------------------------
echo "[5/8] Preview features..."
IMG_BODY=$(curl -s "${BASE}/images/")
check "Image preview link"        "$(echo "$IMG_BODY" | grep -q 'data-preview="image"' && echo true || echo false)"
check "Text preview link"         "$(body_contains 'data-preview="text"')"
check "Gallery prev button"       "$(body_contains 'id="prevBtn"')"
check "Gallery next button"       "$(body_contains 'id="nextBtn"')"
check "Preview download button"   "$(body_contains 'id="previewDownload"')"

# ---------------------------------------------------------------
# 6. Interactive features (JS presence)
# ---------------------------------------------------------------
echo "[6/8] Interactive features..."
check "Select all checkbox"       "$(body_contains 'id="selectAll"')"
check "Row checkboxes"            "$(body_contains 'class="row-checkbox"')"
check "Copy path buttons"         "$(body_contains 'class="copy-btn"')"
check "Sort localStorage logic"   "$(body_contains 'sfs-sort')"
check "Theme localStorage logic"  "$(body_contains 'sfs-theme')"
check "Relative time display"     "$(body_contains 'relativeTime')"
check "URL hash state"            "$(body_contains 'syncHash')"

# ---------------------------------------------------------------
# 7. Subdirectory + static file serving
# ---------------------------------------------------------------
echo "[7/8] Subdirectory & static files..."
SUB_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/subdir/" 2>/dev/null) || true
check "GET /subdir/ => 200"       "$([ "$SUB_STATUS" = "200" ] && echo true || echo false)"

FILE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/hello.txt" 2>/dev/null) || true
check "GET /hello.txt => 200"     "$([ "$FILE_STATUS" = "200" ] && echo true || echo false)"

SVG_CT=$(curl -s -I "${BASE}/images/logo.svg" 2>/dev/null | grep -i content-type | tr -d '\r') || true
check "SVG served with valid type" "$(echo "$SVG_CT" | grep -qi 'svg\|xml\|octet' && echo true || echo false)"

# Check subdirectory listing has parent link
SUB_BODY=$(curl -s "${BASE}/subdir/")
check "Subdir has parent link"    "$(echo "$SUB_BODY" | grep -q 'class="parent"' && echo true || echo false)"

# ---------------------------------------------------------------
# 8. Metrics endpoint
# ---------------------------------------------------------------
echo "[8/9] Metrics endpoint..."
METRICS_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/metrics" 2>/dev/null) || true
check "GET /metrics => 200"         "$([ "$METRICS_STATUS" = "200" ] && echo true || echo false)"

METRICS_BODY=$(curl -s "${BASE}/metrics" 2>/dev/null) || true
check "Metrics has requests_total"  "$(echo "$METRICS_BODY" | grep -q 'static_file_server_requests_total' && echo true || echo false)"
check "Metrics has duration_sum"    "$(echo "$METRICS_BODY" | grep -q 'static_file_server_request_duration_seconds_sum' && echo true || echo false)"
check "Metrics has duration_count"  "$(echo "$METRICS_BODY" | grep -q 'static_file_server_request_duration_seconds_count' && echo true || echo false)"

# ---------------------------------------------------------------
# 9. Batch download API
# ---------------------------------------------------------------
echo "[9/9] Batch download API..."
ZIP_OK=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' \
    -d '{"files":["hello.txt"]}' \
    "${BASE}/?batch=zip" 2>/dev/null) || true
check "POST ?batch=zip => 200"    "$([ "$ZIP_OK" = "200" ] && echo true || echo false)"

ZIP_EMPTY=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' \
    -d '{"files":[]}' \
    "${BASE}/?batch=zip" 2>/dev/null) || true
check "Empty files => 400"        "$([ "$ZIP_EMPTY" = "400" ] && echo true || echo false)"

ZIP_GET=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/?batch=zip" 2>/dev/null) || true
check "GET ?batch=zip => 405"     "$([ "$ZIP_GET" = "405" ] && echo true || echo false)"

ZIP_TRAVERSAL=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' \
    -d '{"files":["../go.mod"]}' \
    "${BASE}/?batch=zip" 2>/dev/null) || true
check "Path traversal => rejected" "$([ "$ZIP_TRAVERSAL" = "400" ] && echo true || echo false)"

# ---------------------------------------------------------------
# Summary
# ---------------------------------------------------------------
echo ""
TOTAL=$((PASS + FAIL))
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
