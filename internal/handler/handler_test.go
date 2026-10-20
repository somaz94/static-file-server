package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Root with index.html, but ?format=json bypasses index.html → JSON listing.
	req = httptest.NewRequest("GET", "/?format=json", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON listing to bypass index.html, got content type %s", ct)
	}
	if body := w.Body.String(); strings.Contains(body, "<html>index</html>") {
		t.Error("JSON request should not return index.html content")
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

func TestWithCORSPreflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS(inner)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, HEAD, OPTIONS" {
		t.Errorf("expected Allow-Methods header, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("expected Max-Age header, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected Allow-Origin *, got %q", got)
	}
	if w.Body.Len() != 0 {
		t.Error("OPTIONS response should have no body")
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
		{"/apifile.txt", http.StatusNotFound, ""}, // prefix must end at a path boundary
		{"/api-v2/file.txt", http.StatusNotFound, ""},
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

func TestWithPrefixRawPath(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s rawpath=%s", r.URL.Path, r.URL.RawPath)
	})
	h := withPrefix(inner, "/api")

	req := httptest.NewRequest("GET", "/api/my%20file.txt", nil)
	// Manually set RawPath to simulate percent-encoded URL.
	req.URL.RawPath = "/api/my%20file.txt"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	expected := "path=/my file.txt rawpath=/my%20file.txt"
	if w.Body.String() != expected {
		t.Errorf("expected %q, got %q", expected, w.Body.String())
	}
}

func TestBuild(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.CORS = true

	h := Build(cfg, slog.New(slog.DiscardHandler))

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
	var buf bytes.Buffer
	h := withLogging(inner, slog.New(slog.NewTextHandler(&buf, nil)))

	req := httptest.NewRequest("GET", "/test-path", nil)
	req.Header.Set("Referer", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	for _, want := range []string{"msg=request", "path=/test-path", "status=200", "referer=https://example.com"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log line missing %q: %s", want, buf.String())
		}
	}
}

func TestWithLoggingJSON(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	var buf bytes.Buffer
	h := withLogging(inner, slog.New(slog.NewJSONHandler(&buf, nil)))

	req := httptest.NewRequest("GET", "/test-path", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v: %s", err, buf.String())
	}
	// The pre-slog request log keys stay, so existing log pipelines keep parsing them.
	for _, key := range []string{"time", "remote", "method", "path", "proto", "host", "status", "duration_ms"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("JSON log missing %q: %s", key, buf.String())
		}
	}
	if _, ok := entry["referer"]; ok {
		t.Errorf("empty referer should be omitted: %s", buf.String())
	}
}

func TestBuildLogsRequestsOnlyWithDebug(t *testing.T) {
	for _, debug := range []bool{false, true} {
		var buf bytes.Buffer
		cfg := config.Default()
		cfg.Folder = setupTestDir(t)
		cfg.Debug = debug
		h := Build(cfg, slog.New(slog.NewTextHandler(&buf, nil)))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if got := strings.Contains(buf.String(), "msg=request"); got != debug {
			t.Errorf("debug=%t: request logged=%t: %s", debug, got, buf.String())
		}
	}
}

func TestWithLoggingUsesStartTime(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
	})
	var buf bytes.Buffer
	before := time.Now()
	withLogging(inner, slog.New(slog.NewJSONHandler(&buf, nil))).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	var entry struct{ Time time.Time }
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Time.Sub(before) > 10*time.Millisecond {
		t.Fatalf("time should be the request start, got %s after it", entry.Time.Sub(before))
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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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
	absDir := resolveAbsFolder(dir)

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
			got, err := safePath(dir, absDir, tt.urlPath)
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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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
	types := map[string]string{
		"/app.apk":      "application/vnd.android.package-archive",
		"/image.jpg":    "image/jpeg",
		"/video.mp4":    "video/mp4",
		"/archive.zip":  "application/zip",
		"/blob.unknown": "application/octet-stream",
	}
	for p, ct := range types {
		t.Run(p, func(t *testing.T) {
			h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ct)
				w.Write([]byte("binary"))
			}))
			req := httptest.NewRequest("GET", p, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Header().Get("Content-Encoding") == "gzip" || w.Body.String() != "binary" {
				t.Errorf("should not compress %s (%s)", p, ct)
			}
		})
	}
}

func TestCompressibleType(t *testing.T) {
	for ct, want := range map[string]bool{
		"text/html; charset=utf-8": true,
		"application/json":         true,
		"text/javascript":          true,
		"image/svg+xml":            true,
		"application/ld+json":      true,
		"application/zip":          false,
		"image/png":                false,
		"application/octet-stream": false,
		"":                         false,
		"not a media type;;":       false,
	} {
		if got := compressibleType(ct); got != want {
			t.Errorf("compressibleType(%q) = %t, want %t", ct, got, want)
		}
	}
}

func TestWithCompressionSniffsUntypedBody(t *testing.T) {
	h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text without a declared type"))
	}))
	req := httptest.NewRequest("GET", "/notes", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("sniffed text should be compressed: %v", w.Header())
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := io.ReadAll(zr); string(body) != "plain text without a declared type" {
		t.Fatalf("round trip: %q", body)
	}
}

func TestWithCompressionSkipsNonOK(t *testing.T) {
	h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	req := httptest.NewRequest("GET", "/x.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatal("error responses should not be compressed")
	}
}

// readFromRecorder records whether ReadFrom reached the underlying writer (the sendfile path).
type readFromRecorder struct {
	*httptest.ResponseRecorder
	readFrom bool
}

func (r *readFromRecorder) ReadFrom(src io.Reader) (int64, error) {
	r.readFrom = true
	return io.Copy(r.ResponseRecorder, src)
}

func TestWithCompressionReadFrom(t *testing.T) {
	for _, tt := range []struct {
		ct           string
		wantGzip     bool
		wantReadFrom bool
	}{
		{"application/vnd.android.package-archive", false, true},
		{"text/plain", true, false},
	} {
		h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", tt.ct)
			w.(io.ReaderFrom).ReadFrom(strings.NewReader("payload"))
		}))
		req := httptest.NewRequest("GET", "/f", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
		h.ServeHTTP(rec, req)
		if (rec.Header().Get("Content-Encoding") == "gzip") != tt.wantGzip || rec.readFrom != tt.wantReadFrom {
			t.Errorf("%s: gzip=%q readFrom=%t", tt.ct, rec.Header().Get("Content-Encoding"), rec.readFrom)
		}
	}
}

func TestBuildCompressionHEADKeepsLength(t *testing.T) {
	dir := setupTestDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.Compression = true
	h := Build(cfg, slog.New(slog.DiscardHandler))

	info, err := os.Stat(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("HEAD", "/hello.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Length"); got != fmt.Sprint(info.Size()) || w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("HEAD should report the identity length %d: %v", info.Size(), w.Header())
	}
	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("HEAD of a compressible file should still vary on Accept-Encoding: %v", w.Header())
	}

	req = httptest.NewRequest("GET", "/hello.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Content-Length") != "" {
		t.Fatalf("GET should be compressed without a Content-Length: %v", w.Header())
	}
}

// --- Flush tests ---

func TestResponseRecorderFlush(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	h := withLogging(inner, slog.New(slog.DiscardHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Body.String() != "data" {
		t.Errorf("expected 'data', got %q", w.Body.String())
	}
	if w.Flushed != true {
		t.Error("expected Flush to be called on underlying ResponseWriter")
	}
}

func TestGzipResponseWriterFlush(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("streaming data"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	h := withCompression(inner)

	req := httptest.NewRequest("GET", "/page.html", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip encoding")
	}
	if w.Flushed != true {
		t.Error("expected Flush to be called through gzip wrapper")
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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

	h := Build(cfg, slog.New(slog.DiscardHandler))

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

func TestDirectoryWithoutSlashRedirects(t *testing.T) {
	dir := setupTestDir(t)
	handlers := map[string]http.Handler{
		"listing":         listing(dir, false),
		"listingAndIndex": listingAndIndex(dir, false),
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/subdir?sort=name", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusMovedPermanently {
				t.Fatalf("expected 301, got %d", w.Code)
			}
			if loc := w.Header().Get("Location"); loc != "subdir/?sort=name" {
				t.Errorf("unexpected Location %q", loc)
			}
		})
	}
}

func TestIndexHTMLServedDirectly(t *testing.T) {
	dir := setupTestDir(t)
	handlers := map[string]http.Handler{
		"basic":   basic(dir),
		"listing": listing(dir, false),
	}
	for name, h := range handlers {
		for path, body := range map[string]string{"/index.html": "<html>index</html>", "/indexed/index.html": "<html>subindex</html>"} {
			t.Run(name+path, func(t *testing.T) {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				if w.Code != http.StatusOK || w.Body.String() != body {
					t.Fatalf("got %d %q (Location %q)", w.Code, w.Body.String(), w.Header().Get("Location"))
				}
			})
		}
	}
}

func TestIndexModeRedirectsDirectoryWithoutSlash(t *testing.T) {
	dir := setupTestDir(t)
	h := index(dir)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/indexed", nil))
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "indexed/" {
		t.Fatalf("expected 301 to indexed/, got %d %q", w.Code, w.Header().Get("Location"))
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/subdir", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("directory without index.html: expected 404, got %d", w.Code)
	}
}

func TestServeFileRejectsMissingAndDirectories(t *testing.T) {
	dir := setupTestDir(t)
	for _, p := range []string{filepath.Join(dir, "missing.txt"), filepath.Join(dir, "subdir")} {
		w := httptest.NewRecorder()
		serveFile(w, httptest.NewRequest("GET", "/x", nil), p)
		if w.Code != http.StatusNotFound {
			t.Errorf("serveFile(%s): expected 404, got %d", p, w.Code)
		}
	}
}

func TestWithMetricsBoundsMethodLabel(t *testing.T) {
	h := withMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, m := range []string{"XYZRANDOM1", "XYZRANDOM2", "PROPFIND"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(m, "/", nil))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, `static_file_server_requests_total{method="OTHER",status="200"} 3`) || strings.Contains(body, "XYZRANDOM") {
		t.Fatalf("unknown methods must collapse into OTHER:\n%s", body)
	}
}

func TestAcceptsGzip(t *testing.T) {
	for h, want := range map[string]bool{
		"gzip":                true,
		"gzip, deflate, br":   true,
		"deflate, GZIP;q=0.5": true,
		"gzip;q=0":            false,
		"gzip; q=0.000":       false,
		"deflate":             false,
		"":                    false,
	} {
		if got := acceptsGzip(h); got != want {
			t.Errorf("acceptsGzip(%q) = %t, want %t", h, got, want)
		}
	}
}

func TestWithCompressionNotModifiedVaries(t *testing.T) {
	h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x.txt", nil))
	if w.Code != http.StatusNotModified || w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("304 should carry Vary: %d %v", w.Code, w.Header())
	}
}

func TestWithCompressionInformationalThenOK(t *testing.T) {
	h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>hello</html>"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("the final 200 after a 1xx should still be compressed: %v", w.Header())
	}
}

func TestWithCompressionRespectsExplicitNoType(t *testing.T) {
	h := withCompression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = nil // net/http's "do not sniff"
		w.Write([]byte("plain text"))
	}))
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("an explicitly untyped body must not be sniffed into compression: %v", w.Header())
	}
}
