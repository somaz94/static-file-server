// Package handler provides HTTP handler middleware and file serving logic.
package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/httpx"
)

// Build creates the complete HTTP handler chain from configuration.
// Middleware order (outer to inner): logging → prefix → accessKey → referrer → CORS → customHeaders → compression → dotFiles → securityHeaders → fileHandler
func Build(cfg *config.Config, logger *slog.Logger) http.Handler {
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
		handler = withLogging(handler, logger)
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

	// Ensure the resolved path is within the folder root. The separator guard
	// prevents a sibling like /webfoo from matching the /web root via a bare
	// prefix check (consistent with the batch download validation).
	if absPath != absFolder && !strings.HasPrefix(absPath, absFolder+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal attempt: %s", urlPath)
	}

	return fullPath, nil
}

// resolve maps the request to a path under folder and stats it. When it returns false
// it has already answered 404.
func resolve(w http.ResponseWriter, r *http.Request, folder, absFolder string) (string, os.FileInfo, bool) {
	fpath, err := safePath(folder, absFolder, r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return "", nil, false
	}
	info, err := os.Stat(fpath)
	if err != nil {
		http.NotFound(w, r)
		return "", nil, false
	}
	return fpath, info, true
}

// serveBatch answers a directory's POST ?batch=zip with a zip archive and reports whether it did.
func serveBatch(w http.ResponseWriter, r *http.Request, fpath string, info os.FileInfo, hideDot bool) bool {
	if r.URL.Query().Get("batch") != "zip" || !info.IsDir() {
		return false
	}
	handleBatchDownload(w, r, fpath, hideDot)
	return true
}

// basic serves files only. Directories return 404.
func basic(folder string) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, info, ok := resolve(w, r, folder, absFolder)
		if !ok {
			return
		}
		if info.IsDir() {
			http.NotFound(w, r)
			return
		}

		serveFile(w, r, fpath)
	}
}

// index serves files and index.html for directories. No directory listing.
func index(folder string) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, info, ok := resolve(w, r, folder, absFolder)
		if !ok {
			return
		}

		if info.IsDir() {
			indexPath := filepath.Join(fpath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				if redirectToSlash(w, r) {
					return
				}
				serveFile(w, r, indexPath)
				return
			}
			http.NotFound(w, r)
			return
		}

		serveFile(w, r, fpath)
	}
}

// listing serves files and directory listings. Does not prefer index.html.
func listing(folder string, hideDot bool) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, info, ok := resolve(w, r, folder, absFolder)
		if !ok {
			return
		}

		if serveBatch(w, r, fpath, info, hideDot) {
			return
		}

		if info.IsDir() {
			if redirectToSlash(w, r) {
				return
			}
			renderListing(w, r, fpath, r.URL.Path, hideDot)
			return
		}

		serveFile(w, r, fpath)
	}
}

// listingAndIndex serves files, prefers index.html for directories, falls back
// to directory listing.
func listingAndIndex(folder string, hideDot bool) http.HandlerFunc {
	absFolder := resolveAbsFolder(folder)
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, info, ok := resolve(w, r, folder, absFolder)
		if !ok {
			return
		}

		if serveBatch(w, r, fpath, info, hideDot) {
			return
		}

		if info.IsDir() {
			if redirectToSlash(w, r) {
				return
			}
			// A JSON listing request bypasses index.html and returns the listing.
			if wantsJSON(r) {
				renderListing(w, r, fpath, r.URL.Path, hideDot)
				return
			}
			indexPath := filepath.Join(fpath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				serveFile(w, r, indexPath)
				return
			}
			renderListing(w, r, fpath, r.URL.Path, hideDot)
			return
		}

		serveFile(w, r, fpath)
	}
}

// serveFile serves a regular file. http.ServeFile is avoided because it redirects every
// request path ending in /index.html to ./, which 404s in basic mode and loops back to
// the listing in listing mode.
func serveFile(w http.ResponseWriter, r *http.Request, fpath string) {
	f, err := os.Open(fpath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// redirectToSlash sends a directory request without a trailing slash to the
// slash form, because the listing's relative hrefs resolve against it. The
// Location is relative so a stripped --prefix survives the redirect.
func redirectToSlash(w http.ResponseWriter, r *http.Request) bool {
	if strings.HasSuffix(r.URL.Path, "/") {
		return false
	}
	target := url.URL{Path: path.Base(r.URL.Path) + "/", RawQuery: r.URL.RawQuery}
	w.Header().Set("Location", target.String())
	w.WriteHeader(http.StatusMovedPermanently)
	return true
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
				serveFile(w, r, indexPath)
				return
			}
			http.NotFound(w, r)
			return
		}

		serveFile(w, r, fpath)
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
		rest, ok := strings.CutPrefix(r.URL.Path, prefix)
		// Without the boundary check, prefix /api would also strip /apifile.txt.
		if !ok || (rest != "" && rest[0] != '/') {
			http.NotFound(w, r)
			return
		}
		r.URL.Path = rest
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

// withLogging logs each request with its response status and elapsed time.
func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := httpx.NewRecorder(w)
		next.ServeHTTP(rec, r)

		if !logger.Enabled(r.Context(), slog.LevelInfo) {
			return
		}
		// Stamped with the start time, as the pre-slog request log was.
		rec2 := slog.NewRecord(start, slog.LevelInfo, "request", 0)
		rec2.AddAttrs(
			slog.String("remote", r.RemoteAddr),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("proto", r.Proto),
			slog.String("host", r.Host),
			slog.Int("status", rec.Status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		if ref := r.Referer(); ref != "" {
			rec2.AddAttrs(slog.String("referer", ref))
		}
		_ = logger.Handler().Handle(r.Context(), rec2)
	})
}
