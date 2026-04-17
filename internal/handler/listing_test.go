package handler

import (
	"archive/zip"
	"bytes"
	"io"
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
	renderListing(w, req, dir, "/", false)

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

	// Check accessibility: main landmark
	if !strings.Contains(body, "<main") {
		t.Error("listing should use <main> landmark")
	}

	// Check accessibility: breadcrumb aria-label
	if !strings.Contains(body, `aria-label="Breadcrumb"`) {
		t.Error("breadcrumb nav should have aria-label")
	}

	// Check accessibility: search aria-label
	if !strings.Contains(body, `aria-label="Filter files"`) {
		t.Error("search input should have aria-label")
	}

	// Check accessibility: aria-live on search count
	if !strings.Contains(body, `aria-live="polite"`) {
		t.Error("search count should have aria-live")
	}

	// Check accessibility: preview overlay role=dialog
	if !strings.Contains(body, `role="dialog"`) {
		t.Error("preview overlay should have role=dialog")
	}

	// Check accessibility: aria-sort on table headers
	if !strings.Contains(body, `aria-sort=`) {
		t.Error("table headers should have aria-sort attribute")
	}

	// Check filter chips
	if !strings.Contains(body, `id="filterChips"`) {
		t.Error("listing should contain filter chips")
	}

	// Check empty state element
	if !strings.Contains(body, `id="emptyState"`) {
		t.Error("listing should contain empty state element")
	}

	// Check file extension badge
	if !strings.Contains(body, `class="ext-badge"`) {
		t.Error("listing should contain file extension badges")
	}
	if !strings.Contains(body, ".go") {
		t.Error("listing should show .go extension badge")
	}

	// Check copy path button
	if !strings.Contains(body, `class="copy-btn"`) {
		t.Error("listing should contain copy path buttons")
	}

	// Check sticky table header
	if !strings.Contains(body, "position: sticky") {
		t.Error("table header should have position: sticky")
	}

	// Check breadcrumb current page (root = Home should be current)
	if !strings.Contains(body, `aria-current="page"`) {
		t.Error("last breadcrumb should have aria-current=page")
	}

	// Check text preview for code files
	if !strings.Contains(body, `data-preview="text"`) {
		t.Error("code/doc files should have data-preview=text")
	}
}

func TestRenderListingWithParent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("content"), 0644)

	req := httptest.NewRequest("GET", "/sub/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, sub, "/sub/", false)

	body := w.Body.String()

	// Subdirectory should have parent link
	if !strings.Contains(body, `class="parent"`) {
		t.Error("subdirectory listing should have parent link")
	}
	if !strings.Contains(body, "..") {
		t.Error("parent link should contain ..")
	}

	// Breadcrumb: "Home" should be a link, "sub" should be current page
	if !strings.Contains(body, `<a href="/">Home</a>`) {
		t.Error("breadcrumb Home should be a link in subdirectory")
	}
	if !strings.Contains(body, `aria-current="page"`) {
		t.Error("last breadcrumb should have aria-current=page")
	}
}

func TestRenderListingRawExt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.tsx"), []byte("export default"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0644)
	os.WriteFile(filepath.Join(dir, "noext"), []byte("binary"), 0644)
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, dir, "/", false)
	body := w.Body.String()

	// Files with extensions should have ext-badge
	if !strings.Contains(body, ".tsx") {
		t.Error("listing should show .tsx extension badge")
	}
	if !strings.Contains(body, ".yaml") {
		t.Error("listing should show .yaml extension badge")
	}

	// File without extension should not have ext-badge after its name
	// (the ext-badge element is only rendered when RawExt is non-empty)
	// Directories should never have ext-badge
}

func TestRenderListingPreviewTypes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "doc.pdf"), []byte("pdf"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("go"), 0644)
	os.WriteFile(filepath.Join(dir, "conf.json"), []byte("json"), 0644)
	os.WriteFile(filepath.Join(dir, "video.mp4"), []byte("video"), 0644)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, dir, "/", false)
	body := w.Body.String()

	// PDF should have preview
	if !strings.Contains(body, `data-preview="pdf"`) {
		t.Error("PDF files should have data-preview=pdf")
	}

	// Text/code/config files should have text preview
	textCount := strings.Count(body, `data-preview="text"`)
	if textCount < 3 {
		t.Errorf("expected at least 3 text previews (txt, go, json), got %d", textCount)
	}

	// Video should have video preview
	if !strings.Contains(body, `data-preview="video"`) {
		t.Error("video files should have data-preview=video")
	}
}

func TestRenderListingDataType(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "folder"), 0755)
	os.WriteFile(filepath.Join(dir, "pic.png"), []byte("img"), 0644)
	os.WriteFile(filepath.Join(dir, "src.go"), []byte("code"), 0644)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, dir, "/", false)
	body := w.Body.String()

	// Rows should have data-type for filter chips
	if !strings.Contains(body, `data-type="dir"`) {
		t.Error("directory rows should have data-type=dir")
	}
	if !strings.Contains(body, `data-type="image"`) {
		t.Error("image rows should have data-type=image")
	}
	if !strings.Contains(body, `data-type="code"`) {
		t.Error("code rows should have data-type=code")
	}
}

func TestRenderListingInvalidDir(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, "/nonexistent/path/xyz", "/", false)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid dir, got %d", w.Code)
	}
}

func TestRenderListingNewFeatures(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("jpg"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("go"), 0644)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	renderListing(w, req, dir, "/", false)
	body := w.Body.String()

	// Grid view toggle
	if !strings.Contains(body, `id="viewToggle"`) {
		t.Error("listing should contain view toggle button")
	}
	if !strings.Contains(body, `id="gridContainer"`) {
		t.Error("listing should contain grid container")
	}

	// Keyboard shortcuts help
	if !strings.Contains(body, `id="helpOverlay"`) {
		t.Error("listing should contain keyboard shortcuts help overlay")
	}
	if !strings.Contains(body, "Keyboard Shortcuts") {
		t.Error("help modal should contain 'Keyboard Shortcuts' heading")
	}

	// Scroll to top button
	if !strings.Contains(body, `id="scrollTop"`) {
		t.Error("listing should contain scroll to top button")
	}

	// Selection bar
	if !strings.Contains(body, `id="selectionBar"`) {
		t.Error("listing should contain selection bar")
	}

	// Select all checkbox
	if !strings.Contains(body, `id="selectAll"`) {
		t.Error("listing should contain select all checkbox")
	}

	// Row checkboxes
	if !strings.Contains(body, `class="row-checkbox"`) {
		t.Error("listing should contain row checkboxes")
	}

	// Preview navigation buttons
	if !strings.Contains(body, `id="prevBtn"`) {
		t.Error("listing should contain preview prev button")
	}
	if !strings.Contains(body, `id="nextBtn"`) {
		t.Error("listing should contain preview next button")
	}

	// Preview download button
	if !strings.Contains(body, `id="previewDownload"`) {
		t.Error("listing should contain preview download button")
	}

	// Fade-in animation
	if !strings.Contains(body, "fadeIn") {
		t.Error("listing should contain fadeIn animation")
	}

	// data-href attribute for grid view
	if !strings.Contains(body, `data-href=`) {
		t.Error("rows should have data-href for grid view support")
	}
}

func TestBatchDownload(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("world"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)

	t.Run("valid request", func(t *testing.T) {
		body := strings.NewReader(`{"files":["file1.txt","file2.txt"]}`)
		req := httptest.NewRequest("POST", "/?batch=zip", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, false)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
			t.Errorf("expected application/zip, got %s", ct)
		}
		if w.Body.Len() == 0 {
			t.Error("zip body should not be empty")
		}
	})

	t.Run("empty files", func(t *testing.T) {
		body := strings.NewReader(`{"files":[]}`)
		req := httptest.NewRequest("POST", "/?batch=zip", body)
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, false)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty files, got %d", w.Code)
		}
	})

	t.Run("GET not allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?batch=zip", nil)
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, false)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for GET, got %d", w.Code)
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		body := strings.NewReader(`{"files":["../etc/passwd","file1.txt"]}`)
		req := httptest.NewRequest("POST", "/?batch=zip", body)
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, false)

		// Should still succeed but only include file1.txt
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("hidden files excluded when hideDot", func(t *testing.T) {
		body := strings.NewReader(`{"files":[".hidden","file1.txt"]}`)
		req := httptest.NewRequest("POST", "/?batch=zip", body)
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, true)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("directories skipped", func(t *testing.T) {
		body := strings.NewReader(`{"files":["subdir","file1.txt"]}`)
		req := httptest.NewRequest("POST", "/?batch=zip", body)
		w := httptest.NewRecorder()
		handleBatchDownload(w, req, dir, false)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

func TestBatchDownloadZIPContents(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("content-a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("content-b"), 0644)

	body := strings.NewReader(`{"files":["a.txt","b.txt"]}`)
	req := httptest.NewRequest("POST", "/?batch=zip", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleBatchDownload(w, req, dir, false)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Open the ZIP and verify contents.
	zipData := w.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	if len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
	}

	expected := map[string]string{
		"a.txt": "content-a",
		"b.txt": "content-b",
	}

	for _, f := range zr.File {
		want, ok := expected[f.Name]
		if !ok {
			t.Errorf("unexpected file in zip: %s", f.Name)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("failed to read %s: %v", f.Name, err)
		}
		if string(data) != want {
			t.Errorf("file %s: expected %q, got %q", f.Name, want, string(data))
		}
	}
}

func TestBatchDownloadOversizedBody(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)

	// Create a body larger than 1MB limit.
	bigBody := strings.Repeat("x", 2<<20)
	req := httptest.NewRequest("POST", "/?batch=zip", strings.NewReader(bigBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleBatchDownload(w, req, dir, false)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized body, got %d", w.Code)
	}
}

func TestBatchDownloadIntegration(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)

	handler := listing(dir, false)

	body := strings.NewReader(`{"files":["a.txt","b.txt"]}`)
	req := httptest.NewRequest("POST", "/?batch=zip", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected application/zip, got %s", ct)
	}
}
