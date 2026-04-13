package handler

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/somaz94/static-file-server/internal/config"
)

// setupTestDir creates a temporary directory with test files.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create files and subdirectories.
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>index</html>"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "file.txt"), []byte("sub file"), 0644)
	os.MkdirAll(filepath.Join(dir, "indexed"), 0755)
	os.WriteFile(filepath.Join(dir, "indexed", "index.html"), []byte("<html>subindex</html>"), 0644)

	return dir
}

func TestBasicHandler(t *testing.T) {
	dir := setupTestDir(t)
	h := basic(dir)

	tests := []struct {
		path string
		code int
		body string
	}{
		{"/hello.txt", http.StatusOK, "hello world"},
		{"/notfound.txt", http.StatusNotFound, ""},
		{"/", http.StatusNotFound, ""},        // directory
		{"/subdir/", http.StatusNotFound, ""}, // directory
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.code, w.Code)
			}
			if tt.body != "" && w.Body.String() != tt.body {
				t.Errorf("path %s: expected body %q, got %q", tt.path, tt.body, w.Body.String())
			}
		})
	}
}

func TestIndexHandler(t *testing.T) {
	dir := setupTestDir(t)
	h := index(dir)

	tests := []struct {
		path string
		code int
		body string
	}{
		{"/hello.txt", http.StatusOK, "hello world"},
		{"/", http.StatusOK, "<html>index</html>"},
		{"/indexed/", http.StatusOK, "<html>subindex</html>"},
		{"/subdir/", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.code, w.Code)
			}
			if tt.body != "" && w.Body.String() != tt.body {
				t.Errorf("path %s: expected body %q, got %q", tt.path, tt.body, w.Body.String())
			}
		})
	}
}

func TestListingHandler(t *testing.T) {
	dir := setupTestDir(t)
	h := listing(dir, false)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %s", ct)
	}
	if !containsAll(body, "hello.txt", "subdir") {
		t.Error("listing should contain file and directory names")
	}
}

func TestListingAndIndexHandler(t *testing.T) {
	dir := setupTestDir(t)
	h := listingAndIndex(dir, false)

	// Root has index.html → serve it.
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "<html>index</html>" {
		t.Errorf("expected index.html content, got %q", body)
	}

	// subdir has no index.html → listing.
	req = httptest.NewRequest("GET", "/subdir/", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !containsAll(w.Body.String(), "file.txt") {
		t.Error("listing should contain file.txt")
	}
}

func TestWithCORS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS(inner)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	expected := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "*",
		"Cross-Origin-Resource-Policy": "cross-origin",
	}
	for key, val := range expected {
		if got := w.Header().Get(key); got != val {
			t.Errorf("header %s: expected %q, got %q", key, val, got)
		}
	}
}

func TestWithReferrer(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withReferrer(inner, []string{"https://example.com", ""})

	tests := []struct {
		referer string
		code    int
	}{
		{"https://example.com/page", http.StatusOK},
		{"", http.StatusOK}, // empty referer allowed by "" in list
		{"https://evil.com", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.referer, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("referer %q: expected %d, got %d", tt.referer, tt.code, w.Code)
			}
		})
	}
}

func TestWithAccessKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAccessKey(inner, "mykey")

	tests := []struct {
		query string
		code  int
	}{
		{"?key=mykey", http.StatusOK},
		{"?key=wrong", http.StatusNotFound},
		{"", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("query %q: expected %d, got %d", tt.query, tt.code, w.Code)
			}
		})
	}
}

func TestWithAccessKeySHA256(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAccessKey(inner, "secret")

	// SHA-256("/test" + "secret") in uppercase hex
	expected := fmt.Sprintf("%X", sha256.Sum256([]byte("/test"+"secret")))
	req := httptest.NewRequest("GET", "/test?code="+expected, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid SHA-256 code, got %d", w.Code)
	}
}

func TestWithPrefix(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.Path)
	})
	h := withPrefix(inner, "/api")

	tests := []struct {
		path     string
		code     int
		expected string
	}{
		{"/api/file.txt", http.StatusOK, "/file.txt"},
		{"/api", http.StatusOK, "/"},
		{"/other", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.code, w.Code)
			}
			if tt.expected != "" && w.Body.String() != tt.expected {
				t.Errorf("path %s: expected body %q, got %q", tt.path, tt.expected, w.Body.String())
			}
		})
	}
}

func TestBuild(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.CORS = true

	h := Build(cfg)

	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Error("expected CORS headers")
	}
}

func TestWithLoggingText(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withLogging(inner, "text")

	req := httptest.NewRequest("GET", "/test-path", nil)
	req.Header.Set("Referer", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWithLoggingJSON(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withLogging(inner, "json")

	req := httptest.NewRequest("GET", "/test-path", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBuildAllCombinations(t *testing.T) {
	dir := setupTestDir(t)

	// Test with all middleware enabled
	cfg := config.Default()
	cfg.Folder = dir
	cfg.CORS = true
	cfg.Debug = true
	cfg.URLPrefix = "/pfx"
	cfg.AccessKey = "key123"
	cfg.Referrers = []string{"https://example.com", ""}

	h := Build(cfg)

	// Valid request with all middleware
	req := httptest.NewRequest("GET", "/pfx/hello.txt?key=key123", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with all middleware, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Error("expected CORS headers")
	}
}

func TestBuildListingOnly(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.ShowListing = true
	cfg.AllowIndex = false

	h := Build(cfg)

	// Directory should show listing, not index.html
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `id="listing"`) {
		t.Error("expected listing HTML")
	}
}

func TestBuildIndexOnly(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.ShowListing = false
	cfg.AllowIndex = true

	h := Build(cfg)

	// Root with index.html should serve it
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "<html>index</html>" {
		t.Errorf("expected index content, got %q", w.Body.String())
	}
}

func TestBuildBasicOnly(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.ShowListing = false
	cfg.AllowIndex = false

	h := Build(cfg)

	// File should work
	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for file, got %d", w.Code)
	}

	// Directory should 404
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for directory, got %d", w.Code)
	}
}

func TestListingHandlerServeFile(t *testing.T) {
	dir := setupTestDir(t)
	h := listing(dir, false)

	// File request
	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for file, got %d", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("expected body 'hello world', got %q", w.Body.String())
	}
}

func TestListingHandlerNotFound(t *testing.T) {
	dir := setupTestDir(t)
	h := listing(dir, false)

	req := httptest.NewRequest("GET", "/no-such-file.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSafePath(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		urlPath  string
		expected string
	}{
		{"/file.txt", "/file.txt"},
		// /../etc/passwd is cleaned to /etc/passwd, which stays within folder.
		{"/../etc/passwd", "/etc/passwd"},
		{"/subdir/../file.txt", "/file.txt"},
		{"///double///slashes", "/double/slashes"},
	}

	for _, tt := range tests {
		t.Run(tt.urlPath, func(t *testing.T) {
			got, err := safePath(dir, tt.urlPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expectedFull := filepath.Join(dir, tt.expected)
			if got != expectedFull {
				t.Errorf("expected %s, got %s", expectedFull, got)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir

	h := Build(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("expected text/plain, got %s", ct)
	}
}

func TestHealthzBypassesMiddleware(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AccessKey = "secret"

	h := Build(cfg)

	// /healthz should work even with access key required
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz should bypass access key, got %d", w.Code)
	}
}

func TestWithCustomHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	headers := map[string]string{
		"X-Custom-Test":          "hello",
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
	}
	h := withCustomHeaders(inner, headers)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	for key, val := range headers {
		if got := w.Header().Get(key); got != val {
			t.Errorf("header %s: expected %q, got %q", key, val, got)
		}
	}
}

// --- SPA mode tests ---

func TestSPAHandler(t *testing.T) {
	dir := setupTestDir(t)
	h := spa(dir)

	// Existing file: serve it directly.
	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for existing file, got %d", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", w.Body.String())
	}

	// Non-existing path: fallback to /index.html.
	req = httptest.NewRequest("GET", "/app/dashboard", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA fallback, got %d", w.Code)
	}
	if w.Body.String() != "<html>index</html>" {
		t.Errorf("expected index.html content, got %q", w.Body.String())
	}

	// Directory path: fallback to /index.html.
	req = httptest.NewRequest("GET", "/subdir/", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA directory fallback, got %d", w.Code)
	}
}

func TestSPAHandlerNoIndex(t *testing.T) {
	dir := t.TempDir()
	// No index.html at all
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)
	h := spa(dir)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no index.html, got %d", w.Code)
	}
}

func TestBuildSPA(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.SPA = true
	cfg.ShowListing = false

	h := Build(cfg)

	req := httptest.NewRequest("GET", "/any/route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA route, got %d", w.Code)
	}
}

// --- Hidden dot files tests ---

func TestWithHideDotFiles(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withHideDotFiles(inner)

	tests := []struct {
		path string
		code int
	}{
		{"/normal.txt", http.StatusOK},
		{"/.env", http.StatusNotFound},
		{"/.git/config", http.StatusNotFound},
		{"/subdir/.hidden", http.StatusNotFound},
		{"/", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tt.code {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.code, w.Code)
			}
		})
	}
}

func TestListingHideDotFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("public"), 0644)

	h := listing(dir, true)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, ".hidden") {
		t.Error("listing should not contain .hidden when hideDot=true")
	}
	if !strings.Contains(body, "visible.txt") {
		t.Error("listing should contain visible.txt")
	}
}

// --- Compression tests ---

func TestWithCompression(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>hello world</html>"))
	})
	h := withCompression(inner)

	// With gzip support
	req := httptest.NewRequest("GET", "/page.html", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected Content-Encoding: gzip")
	}
	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Error("expected Vary: Accept-Encoding")
	}
}

func TestWithCompressionSkipNoAccept(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain"))
	})
	h := withCompression(inner)

	// Without Accept-Encoding: no compression
	req := httptest.NewRequest("GET", "/page.html", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not compress without Accept-Encoding")
	}
	if w.Body.String() != "plain" {
		t.Errorf("expected 'plain', got %q", w.Body.String())
	}
}

func TestWithCompressionSkipRange(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	})
	h := withCompression(inner)

	req := httptest.NewRequest("GET", "/file.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-100")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not compress Range requests")
	}
}

func TestWithCompressionSkipBinary(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("binary"))
	})
	h := withCompression(inner)

	binaryPaths := []string{"/app.apk", "/image.jpg", "/video.mp4", "/archive.zip"}
	for _, p := range binaryPaths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest("GET", p, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Header().Get("Content-Encoding") == "gzip" {
				t.Errorf("should not compress %s", p)
			}
		})
	}
}

// --- Metrics tests ---

func TestWithMetrics(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	h := withMetrics(inner)

	// Make a few requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/file.txt", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}

	// Check /metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /metrics, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "static_file_server_requests_total") {
		t.Error("metrics should contain request counter")
	}
	if !strings.Contains(body, "static_file_server_response_bytes_total") {
		t.Error("metrics should contain response bytes counter")
	}
	if !strings.Contains(body, "static_file_server_request_duration_seconds_bucket") {
		t.Error("metrics should contain duration histogram")
	}
	if !strings.Contains(body, `method="GET"`) {
		t.Error("metrics should contain GET method label")
	}
	if !strings.Contains(body, `status="200"`) {
		t.Error("metrics should contain 200 status label")
	}
	if !strings.Contains(body, "static_file_server_request_duration_seconds_sum") {
		t.Error("metrics should contain duration sum")
	}
	if !strings.Contains(body, "static_file_server_request_duration_seconds_count 3") {
		t.Error("metrics should contain duration count of 3")
	}
	if !strings.Contains(body, `le="+Inf"`) {
		t.Error("metrics should contain +Inf bucket")
	}
}

func TestBuildWithMetrics(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.Metrics = true

	h := Build(cfg)

	// Normal request to populate metrics
	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// 404 request to exercise WriteHeader in metricsResponseWriter
	req = httptest.NewRequest("GET", "/nonexistent", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Check /metrics endpoint
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /metrics, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `status="200"`) {
		t.Error("metrics should contain 200 status")
	}
	if !strings.Contains(body, `status="404"`) {
		t.Error("metrics should contain 404 status")
	}
}

// --- Build with new features tests ---

func TestBuildWithCompression(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.Compression = true

	h := Build(cfg)

	req := httptest.NewRequest("GET", "/hello.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBuildWithHideDotFiles(t *testing.T) {
	dir := setupTestDir(t)
	os.WriteFile(filepath.Join(dir, ".secret"), []byte("hidden"), 0644)

	cfg := config.Default()
	cfg.Folder = dir
	cfg.HideDotFiles = true

	h := Build(cfg)

	req := httptest.NewRequest("GET", "/.secret", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for dot file, got %d", w.Code)
	}
}

// helpers

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
