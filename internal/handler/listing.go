package handler

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/somaz94/static-file-server/internal/version"
)

//go:embed templates/listing.html
var templateFS embed.FS

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

var listingFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
}

var listingTmpl = template.Must(
	template.New("").Funcs(listingFuncs).ParseFS(templateFS, "templates/listing.html"),
)

// ListingData holds the data passed to the directory listing template.
type ListingData struct {
	Path        string
	Breadcrumbs []Breadcrumb
	HasParent   bool
	ParentPath  string
	Entries     []ListingEntry
	Version     string
	TotalFiles  int
	TotalDirs   int
	TotalSize   string
}

// Breadcrumb represents a single breadcrumb navigation element.
type Breadcrumb struct {
	Name string
	Path string
}

// ListingEntry represents a file or directory in the listing.
type ListingEntry struct {
	Name        string
	Href        string
	IsDir       bool
	Size        string
	SizeBytes   int64
	ModTime     string
	ModTimeUnix int64
	Ext         string // file category for icon selection (e.g. "image", "code", "archive")
	RawExt      string // raw file extension without dot (e.g. "go", "py", "tsx")
}

// fileCategory returns a category string based on file extension for icon display.
func fileCategory(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".ico", ".tiff":
		return "image"
	case ".mp4", ".avi", ".mov", ".mkv", ".webm", ".flv", ".wmv":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx", ".odt", ".rtf", ".txt", ".md", ".csv", ".tsv":
		return "doc"
	case ".xls", ".xlsx", ".ods":
		return "sheet"
	case ".ppt", ".pptx", ".odp":
		return "slide"
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".rar", ".7z", ".tgz":
		return "archive"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb",
		".php", ".swift", ".kt", ".scala", ".sh", ".bash", ".zsh", ".ps1",
		".html", ".css", ".scss", ".less", ".jsx", ".tsx", ".vue", ".svelte":
		return "code"
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".env", ".conf", ".cfg":
		return "config"
	case ".exe", ".bin", ".dll", ".so", ".dylib", ".wasm":
		return "binary"
	case ".ttf", ".otf", ".woff", ".woff2", ".eot":
		return "font"
	default:
		return "file"
	}
}

// renderListing reads a directory and renders the HTML listing template.
func renderListing(w http.ResponseWriter, _ *http.Request, fsPath, urlPath string, hideDot bool) {
	entries, err := os.ReadDir(fsPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	// Normalize URL path.
	urlPath = "/" + strings.Trim(urlPath, "/")
	if urlPath != "/" {
		urlPath += "/"
	}

	data := ListingData{
		Path:        urlPath,
		Breadcrumbs: buildBreadcrumbs(urlPath),
		HasParent:   urlPath != "/",
		ParentPath:  path.Dir(strings.TrimSuffix(urlPath, "/")),
		Version:     version.Version,
	}

	if data.ParentPath == "." {
		data.ParentPath = "/"
	}
	if !strings.HasSuffix(data.ParentPath, "/") {
		data.ParentPath += "/"
	}

	listing := make([]ListingEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()

		// Skip dot files when hidden.
		if hideDot && strings.HasPrefix(name, ".") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		href := name
		if e.IsDir() {
			href += "/"
		}

		ext := ""
		rawExt := ""
		if !e.IsDir() {
			ext = fileCategory(name)
			if fe := filepath.Ext(name); fe != "" {
				rawExt = strings.TrimPrefix(fe, ".") // "go", "py", etc.
			}
		}

		listing = append(listing, ListingEntry{
			Name:        name,
			Href:        href,
			IsDir:       e.IsDir(),
			Size:        formatSize(info.Size()),
			SizeBytes:   info.Size(),
			ModTime:     info.ModTime().Format(time.DateTime),
			ModTimeUnix: info.ModTime().Unix(),
			Ext:         ext,
			RawExt:      rawExt,
		})
	}

	// Sort: directories first, then alphabetically.
	sort.Slice(listing, func(i, j int) bool {
		if listing[i].IsDir != listing[j].IsDir {
			return listing[i].IsDir
		}
		return strings.ToLower(listing[i].Name) < strings.ToLower(listing[j].Name)
	})

	data.Entries = listing

	var totalSize int64
	for _, e := range listing {
		if e.IsDir {
			data.TotalDirs++
		} else {
			data.TotalFiles++
			totalSize += e.SizeBytes
		}
	}
	data.TotalSize = formatSize(totalSize)

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := listingTmpl.ExecuteTemplate(buf, "listing.html", data); err != nil {
		http.Error(w, "Template rendering failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// buildBreadcrumbs generates navigation breadcrumbs from a URL path.
func buildBreadcrumbs(urlPath string) []Breadcrumb {
	crumbs := []Breadcrumb{{Name: "Home", Path: "/"}}

	trimmed := strings.Trim(urlPath, "/")
	if trimmed == "" {
		return crumbs
	}

	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		crumbs = append(crumbs, Breadcrumb{
			Name: part,
			Path: "/" + strings.Join(parts[:i+1], "/") + "/",
		})
	}

	return crumbs
}

// batchDownloadRequest is the JSON body for batch download.
type batchDownloadRequest struct {
	Files []string `json:"files"`
}

const (
	maxBatchFiles       = 100
	maxBatchSize        = 500 << 20 // 500 MB
	maxConcurrentBatch  = 5
)

// batchSem limits the number of concurrent batch download operations.
var batchSem = make(chan struct{}, maxConcurrentBatch)

// handleBatchDownload creates a zip archive of the requested files and streams it.
func handleBatchDownload(w http.ResponseWriter, r *http.Request, fsPath string, hideDot bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit concurrent batch downloads.
	select {
	case batchSem <- struct{}{}:
		defer func() { <-batchSem }()
	default:
		http.Error(w, "Too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}

	// Limit request body to 1 MB to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req batchDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Files) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}
	if len(req.Files) > maxBatchFiles {
		http.Error(w, fmt.Sprintf("Too many files (max %d)", maxBatchFiles), http.StatusBadRequest)
		return
	}

	// Resolve absolute base path for traversal check.
	absBase, err := filepath.Abs(fsPath)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Pre-validate all files and enforce total size limit.
	type validFile struct {
		name string
		path string
	}
	var files []validFile
	var totalSize int64

	for _, name := range req.Files {
		if name == "" || strings.ContainsAny(name, "/\\") || name == ".." || name == "." {
			continue
		}
		if hideDot && strings.HasPrefix(name, ".") {
			continue
		}

		fpath := filepath.Join(fsPath, name)
		absPath, err := filepath.Abs(fpath)
		if err != nil || !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
			continue // path traversal attempt
		}

		info, err := os.Lstat(fpath)
		if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue // skip dirs and symlinks
		}

		totalSize += info.Size()
		if totalSize > maxBatchSize {
			http.Error(w, "Total file size exceeds limit", http.StatusRequestEntityTooLarge)
			return
		}

		files = append(files, validFile{name: name, path: fpath})
	}

	if len(files) == 0 {
		http.Error(w, "No valid files to download", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="download.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, vf := range files {
		fw, err := zw.Create(vf.name)
		if err != nil {
			return
		}

		f, err := os.Open(vf.path)
		if err != nil {
			return
		}
		_, copyErr := io.Copy(fw, f)
		f.Close()
		if copyErr != nil {
			return
		}
	}
}

// formatSize converts bytes to a human-readable size string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
