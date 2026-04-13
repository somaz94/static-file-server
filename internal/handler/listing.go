package handler

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed templates/listing.html
var templateFS embed.FS

var listingTmpl = template.Must(
	template.ParseFS(templateFS, "templates/listing.html"),
)

// ListingData holds the data passed to the directory listing template.
type ListingData struct {
	Path        string
	Breadcrumbs []Breadcrumb
	HasParent   bool
	ParentPath  string
	Entries     []ListingEntry
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
}

// renderListing reads a directory and renders the HTML listing template.
func renderListing(w http.ResponseWriter, _ *http.Request, fsPath, urlPath string) {
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
	}

	if data.ParentPath == "." {
		data.ParentPath = "/"
	}
	if !strings.HasSuffix(data.ParentPath, "/") {
		data.ParentPath += "/"
	}

	listing := make([]ListingEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		name := e.Name()
		href := name
		if e.IsDir() {
			href += "/"
		}

		listing = append(listing, ListingEntry{
			Name:        name,
			Href:        href,
			IsDir:       e.IsDir(),
			Size:        formatSize(info.Size()),
			SizeBytes:   info.Size(),
			ModTime:     info.ModTime().Format(time.DateTime),
			ModTimeUnix: info.ModTime().Unix(),
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := listingTmpl.ExecuteTemplate(w, "listing.html", data); err != nil {
		http.Error(w, "Template rendering failed", http.StatusInternalServerError)
	}
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
