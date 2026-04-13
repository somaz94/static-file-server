package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/handler"
)

// setupIntegrationDir creates a realistic directory structure for integration tests.
func setupIntegrationDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Root files
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>root index</html>"), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{margin:0}"), 0644)
	os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("fake-jpg-data"), 0644)
	os.WriteFile(filepath.Join(dir, "archive.tar.gz"), []byte("fake-archive"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value"), 0644)

	// Subdirectory with files
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "readme.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir, "docs", "index.html"), []byte("<html>docs index</html>"), 0644)

	// Nested empty directory
	os.MkdirAll(filepath.Join(dir, "assets", "images"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", "images", "logo.png"), []byte("fake-png"), 0644)

	return dir
}

func TestIntegration_ListingAndIndex(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AllowIndex = true
	cfg.ShowListing = true

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Root: has index.html → serve it
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for /, got %d", resp.StatusCode)
	}
	if string(body) != "<html>root index</html>" {
		t.Errorf("expected root index, got %q", string(body))
	}

	// /docs/ has index.html → serve it
	resp, err = http.Get(srv.URL + "/docs/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "<html>docs index</html>" {
		t.Errorf("expected docs index, got %q", string(body))
	}

	// /assets/ has no index.html → directory listing
	resp, err = http.Get(srv.URL + "/assets/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for /assets/, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "images/") {
		t.Error("assets listing should contain 'images/'")
	}
	if !strings.Contains(string(body), "icon-dir") {
		t.Error("listing should contain directory icon classes")
	}
}

func TestIntegration_StaticFile(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "body{margin:0}" {
		t.Errorf("expected CSS content, got %q", string(body))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css content type, got %s", ct)
	}
}

func TestIntegration_NestedFile(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/assets/images/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "fake-png" {
		t.Errorf("expected 'fake-png', got %q", string(body))
	}
}

func TestIntegration_NotFound(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent.txt")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_CORS(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.CORS = true

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected CORS header *, got %q", got)
	}
	if got := resp.Header.Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
		t.Errorf("expected CORP header cross-origin, got %q", got)
	}
}

func TestIntegration_URLPrefix(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.URLPrefix = "/static"

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// With prefix: should work
	resp, err := http.Get(srv.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with prefix, got %d", resp.StatusCode)
	}
	if string(body) != "body{margin:0}" {
		t.Errorf("expected CSS content, got %q", string(body))
	}

	// Without prefix: should 404
	resp, err = http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 without prefix, got %d", resp.StatusCode)
	}
}

func TestIntegration_AccessKey(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AccessKey = "testkey123"

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Without key: should 404
	resp, err := http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 without access key, got %d", resp.StatusCode)
	}

	// With correct key: should work
	resp, err = http.Get(srv.URL + "/style.css?key=testkey123")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with access key, got %d", resp.StatusCode)
	}
	if string(body) != "body{margin:0}" {
		t.Errorf("expected CSS content, got %q", string(body))
	}

	// With wrong key: should 404
	resp, err = http.Get(srv.URL + "/style.css?key=wrongkey")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 with wrong key, got %d", resp.StatusCode)
	}
}

func TestIntegration_ReferrerValidation(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.Referrers = []string{"https://example.com"}

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	client := &http.Client{}

	// Valid referrer
	req, _ := http.NewRequest("GET", srv.URL+"/style.css", nil)
	req.Header.Set("Referer", "https://example.com/page")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 with valid referrer, got %d", resp.StatusCode)
	}

	// Invalid referrer
	req, _ = http.NewRequest("GET", srv.URL+"/style.css", nil)
	req.Header.Set("Referer", "https://evil.com")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("expected 403 with invalid referrer, got %d", resp.StatusCode)
	}
}

func TestIntegration_ListingOnly(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AllowIndex = false
	cfg.ShowListing = true

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Root should show listing even though index.html exists
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	// Should contain listing HTML, not index.html content
	if !strings.Contains(string(body), `id="listing"`) {
		t.Error("expected listing table, not index.html content")
	}
}

func TestIntegration_BasicOnly(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AllowIndex = false
	cfg.ShowListing = false

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// File: should work
	resp, err := http.Get(srv.URL + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for file, got %d", resp.StatusCode)
	}

	// Directory: should 404
	resp, err = http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for directory in basic mode, got %d", resp.StatusCode)
	}
}

func TestIntegration_DirectoryTraversal(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Attempt path traversal
	resp, err := http.Get(srv.URL + "/../../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		t.Error("path traversal should not return 200")
	}
}

func TestIntegration_ListingContainsExtIcons(t *testing.T) {
	dir := setupIntegrationDir(t)
	cfg := config.Default()
	cfg.Folder = dir
	cfg.AllowIndex = false
	cfg.ShowListing = true

	h := handler.Build(cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s := string(body)

	// Check extension-based icon classes
	if !strings.Contains(s, "icon-image") {
		t.Error("listing should contain icon-image for .jpg file")
	}
	if !strings.Contains(s, "icon-code") {
		t.Error("listing should contain icon-code for .css file")
	}
	if !strings.Contains(s, "icon-archive") {
		t.Error("listing should contain icon-archive for .tar.gz file")
	}
	if !strings.Contains(s, "icon-config") {
		t.Error("listing should contain icon-config for .yaml file")
	}

	// Check search bar
	if !strings.Contains(s, `id="search"`) {
		t.Error("listing should contain search input")
	}
	if !strings.Contains(s, "Filter files") {
		t.Error("listing should contain search placeholder")
	}

	// Check preview modal
	if !strings.Contains(s, "previewOverlay") {
		t.Error("listing should contain preview overlay")
	}
	if !strings.Contains(s, `data-preview="image"`) {
		t.Error("image files should have data-preview attribute")
	}
}
