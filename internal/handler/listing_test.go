package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBreadcrumbs(t *testing.T) {
	tests := []struct {
		path     string
		expected []Breadcrumb
	}{
		{"/", []Breadcrumb{{Name: "Home", Path: "/"}}},
		{"/foo/", []Breadcrumb{
			{Name: "Home", Path: "/"},
			{Name: "foo", Path: "/foo/"},
		}},
		{"/a/b/c/", []Breadcrumb{
			{Name: "Home", Path: "/"},
			{Name: "a", Path: "/a/"},
			{Name: "b", Path: "/a/b/"},
			{Name: "c", Path: "/a/b/c/"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := buildBreadcrumbs(tt.path)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d breadcrumbs, got %d", len(tt.expected), len(got))
			}
			for i := range got {
				if got[i].Name != tt.expected[i].Name || got[i].Path != tt.expected[i].Path {
					t.Errorf("breadcrumb %d: expected {%q, %q}, got {%q, %q}",
						i, tt.expected[i].Name, tt.expected[i].Path,
						got[i].Name, got[i].Path)
				}
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{5368709120, "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.expected {
				t.Errorf("formatSize(%d): expected %q, got %q", tt.bytes, tt.expected, got)
			}
		})
	}
}

func TestFileCategory(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"photo.jpg", "image"},
		{"photo.JPEG", "image"},
		{"photo.png", "image"},
		{"photo.svg", "image"},
		{"movie.mp4", "video"},
		{"movie.mkv", "video"},
		{"song.mp3", "audio"},
		{"song.flac", "audio"},
		{"doc.pdf", "pdf"},
		{"readme.md", "doc"},
		{"readme.txt", "doc"},
		{"data.csv", "doc"},
		{"data.xlsx", "sheet"},
		{"slides.pptx", "slide"},
		{"archive.tar.gz", "archive"},
		{"backup.zip", "archive"},
		{"main.go", "code"},
		{"index.html", "code"},
		{"style.css", "code"},
		{"app.tsx", "code"},
		{"config.yaml", "config"},
		{"config.json", "config"},
		{".env", "config"},
		{"app.exe", "binary"},
		{"lib.so", "binary"},
		{"font.woff2", "font"},
		{"unknown.xyz", "file"},
		{"noext", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileCategory(tt.name)
			if got != tt.expected {
				t.Errorf("fileCategory(%q): expected %q, got %q", tt.name, tt.expected, got)
			}
		})
	}
}

func TestRenderListing(t *testing.T) {
	dir := t.TempDir()

	// Create test files and directories
	os.MkdirAll(filepath.Join(dir, "images"), 0755)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("fake jpg"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, dir, "/")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Check content type
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %s", ct)
	}

	// Check entries appear
	for _, name := range []string{"images", "readme.md", "photo.jpg", "main.go"} {
		if !strings.Contains(body, name) {
			t.Errorf("listing should contain %q", name)
		}
	}

	// Check directories come first (images/ before files)
	idxDir := strings.Index(body, "images/")
	idxFile := strings.Index(body, "readme.md")
	if idxDir > idxFile {
		t.Error("directories should appear before files")
	}

	// Check icon classes
	if !strings.Contains(body, "icon-dir") {
		t.Error("listing should contain directory icon class")
	}
	if !strings.Contains(body, "icon-image") {
		t.Error("listing should contain image icon class for .jpg")
	}
	if !strings.Contains(body, "icon-code") {
		t.Error("listing should contain code icon class for .go")
	}
	if !strings.Contains(body, "icon-doc") {
		t.Error("listing should contain doc icon class for .md")
	}

	// Check preview data attributes
	if !strings.Contains(body, `data-preview="image"`) {
		t.Error("image files should have data-preview attribute")
	}

	// Check search input
	if !strings.Contains(body, `id="search"`) {
		t.Error("listing should contain search input")
	}

	// Root path should not have parent link
	if strings.Contains(body, `class="parent"`) {
		t.Error("root listing should not have parent link")
	}
}

func TestRenderListingWithParent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("content"), 0644)

	req := httptest.NewRequest("GET", "/sub/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, sub, "/sub/")

	body := w.Body.String()

	// Subdirectory should have parent link
	if !strings.Contains(body, `class="parent"`) {
		t.Error("subdirectory listing should have parent link")
	}
	if !strings.Contains(body, "..") {
		t.Error("parent link should contain ..")
	}
}

func TestRenderListingInvalidDir(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, "/nonexistent/path/xyz", "/")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid dir, got %d", w.Code)
	}
}
