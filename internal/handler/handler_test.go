package handler

import (
	"crypto/md5"
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
		{"/", http.StatusNotFound, ""},       // directory
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
	h := listing(dir)

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
	h := listingAndIndex(dir)

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

func TestWithAccessKeyMD5(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAccessKey(inner, "secret")

	// MD5("/test" + "secret") in uppercase hex
	expected := fmt.Sprintf("%X", md5Sum("/test"+"secret"))
	req := httptest.NewRequest("GET", "/test?code="+expected, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid MD5 code, got %d", w.Code)
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

// helpers

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func md5Sum(s string) [16]byte {
	return md5.Sum([]byte(s))
}
