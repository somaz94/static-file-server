// Package handler provides HTTP handler middleware and file serving logic.
package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
)

// Build creates the complete HTTP handler chain from configuration.
// Middleware order (outer to inner): logging → prefix → accessKey → referrer → CORS → customHeaders → compression → dotFiles → securityHeaders → fileHandler
func Build(cfg *config.Config) http.Handler {
	var handler http.Handler

	// Base handler: file serving with index/listing/SPA control
	switch {
	case cfg.SPA:
		handler = spa(cfg.Folder)
	case cfg.ShowListing && cfg.AllowIndex:
		handler = listingAndIndex(cfg.Folder, cfg.HideDotFiles)
	case cfg.ShowListing:
		handler = listing(cfg.Folder, cfg.HideDotFiles)
	case cfg.AllowIndex:
		handler = index(cfg.Folder)
	default:
		handler = basic(cfg.Folder)
	}

	// Security headers applied to all responses.
	handler = withSecurityHeaders(handler)

	if cfg.HideDotFiles {
		handler = withHideDotFiles(handler)
	}

	if cfg.Compression {
		handler = withCompression(handler)
	}

	if len(cfg.CustomHeaders) > 0 {
		handler = withCustomHeaders(handler, cfg.CustomHeaders)
	}

	if cfg.CORS {
		handler = withCORS(handler)
	}

	if len(cfg.Referrers) > 0 {
		handler = withReferrer(handler, cfg.Referrers)
	}

	if cfg.AccessKey != "" {
		handler = withAccessKey(handler, cfg.AccessKey)
	}

	if cfg.URLPrefix != "" {
		handler = withPrefix(handler, cfg.URLPrefix)
	}

	if cfg.Debug {
		handler = withLogging(handler, cfg.LogFormat)
	}

	// Wrap with health check and optional metrics (bypass all middleware)
	handler = withHealthz(handler)

	if cfg.Metrics {
		handler = withMetrics(handler)
	}

	return handler
}

// withSecurityHeaders adds default security headers to all responses.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// withHealthz adds a /healthz endpoint that bypasses all middleware.
func withHealthz(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withCustomHeaders adds custom response headers to all responses.
func withCustomHeaders(next http.Handler, headers map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		next.ServeHTTP(w, r)
	})
}

// resolveAbsFolder computes the absolute path once at handler construction time.
func resolveAbsFolder(folder string) string {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return folder
	}
	return abs
}

// safePath resolves a URL path against the folder root, preventing directory traversal.
// absFolder must be pre-computed via resolveAbsFolder at handler creation time.
func safePath(folder, absFolder, urlPath string) (string, error) {
	cleaned := filepath.Clean("/" + urlPath)
	fullPath := filepath.Join(folder, cleaned)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	// Ensure the resolved path is within the folder root.
	if !strings.HasPrefix(absPath, absFolder) {
		return "", fmt.Errorf("path traversal attempt: %s", urlPath)
	}

	return fullPath, nil
}

// basic serves files only. Directories return 404.
func basic(folder string) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, absFolder, r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fpath)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// index serves files and index.html for directories. No directory listing.
func index(folder string) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, absFolder, r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fpath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if info.IsDir() {
			indexPath := filepath.Join(fpath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// listing serves files and directory listings. Does not prefer index.html.
func listing(folder string, hideDot bool) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, absFolder, r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fpath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Batch download: POST ?batch=zip
		if r.URL.Query().Get("batch") == "zip" && info.IsDir() {
			handleBatchDownload(w, r, fpath, hideDot)
			return
		}

		if info.IsDir() {
			renderListing(w, r, fpath, r.URL.Path, hideDot)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// listingAndIndex serves files, prefers index.html for directories, falls back
// to directory listing.
func listingAndIndex(folder string, hideDot bool) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, absFolder, r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fpath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Batch download: POST ?batch=zip
		if r.URL.Query().Get("batch") == "zip" && info.IsDir() {
			handleBatchDownload(w, r, fpath, hideDot)
			return
		}

		if info.IsDir() {
			indexPath := filepath.Join(fpath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			renderListing(w, r, fpath, r.URL.Path, hideDot)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// spa serves files if they exist, otherwise falls back to /index.html.
// Designed for single-page applications (React, Vue, Angular).
func spa(folder string) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, absFolder, r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fpath)
		if err != nil || info.IsDir() {
			// Fallback to root index.html for SPA routing.
			indexPath := filepath.Join(folder, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// withHideDotFiles rejects requests for dot files/directories (e.g. .env, .git).
func withHideDotFiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, segment := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(segment, ".") && segment != "." && segment != ".." {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// withCORS adds CORS headers to all responses and handles preflight OPTIONS requests.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withReferrer validates the Referer header against a whitelist.
func withReferrer(next http.Handler, referrers []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer := r.Header.Get("Referer")

		for _, allowed := range referrers {
			// Empty string in list allows requests without Referer header.
			if allowed == "" && referer == "" {
				next.ServeHTTP(w, r)
				return
			}
			if allowed != "" && strings.HasPrefix(referer, allowed) {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

// withAccessKey validates URL parameter access key or SHA-256 code.
func withAccessKey(next http.Handler, accessKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Direct key match: ?key=<access_key> (timing-safe comparison)
		if key := query.Get("key"); subtle.ConstantTimeCompare([]byte(key), []byte(accessKey)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		// SHA-256 code match: ?code=<SHA256(path + key)>
		if code := query.Get("code"); code != "" {
			expected := fmt.Sprintf("%X", sha256.Sum256([]byte(r.URL.Path+accessKey)))
			if strings.EqualFold(code, expected) {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.NotFound(w, r)
	})
}

// withPrefix strips a URL prefix and returns 404 if the prefix doesn't match.
func withPrefix(next http.Handler, prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		if r.URL.RawPath != "" {
			r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
			if r.URL.RawPath == "" {
				r.URL.RawPath = "/"
			}
		}
		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps http.ResponseWriter to capture the status code and bytes written.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytes += int64(n)
	return n, err
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// logEntry holds structured log data for JSON output.
type logEntry struct {
	Time       string `json:"time"`
	Remote     string `json:"remote"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Proto      string `json:"proto"`
	Host       string `json:"host"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Referer    string `json:"referer,omitempty"`
}

// withLogging logs each request with response status and elapsed time to stderr.
func withLogging(next http.Handler, format string) http.Handler {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	jsonLogger := log.New(os.Stderr, "", 0)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)

		if format == "json" {
			entry := logEntry{
				Time:       start.UTC().Format(time.RFC3339),
				Remote:     r.RemoteAddr,
				Method:     r.Method,
				Path:       r.URL.Path,
				Proto:      r.Proto,
				Host:       r.Host,
				Status:     rec.status,
				DurationMs: elapsed.Milliseconds(),
				Referer:    r.Referer(),
			}
			if data, err := json.Marshal(entry); err == nil {
				jsonLogger.Println(string(data))
			}
		} else {
			logger.Printf("%s %s %s %s %s %d %s referer=%q",
				r.RemoteAddr, r.Method, r.Proto, r.Host, r.URL.Path,
				rec.status, elapsed, r.Referer())
		}
	})
}
