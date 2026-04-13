// Package handler provides HTTP handler middleware and file serving logic.
package handler

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/somaz94/static-file-server/internal/config"
)

// Build creates the complete HTTP handler chain from configuration.
// Middleware order (outer to inner): logging → prefix → accessKey → referrer → CORS → customHeaders → fileHandler
func Build(cfg *config.Config) http.Handler {
	var handler http.Handler

	// Base handler: file serving with index/listing control
	switch {
	case cfg.ShowListing && cfg.AllowIndex:
		handler = listingAndIndex(cfg.Folder)
	case cfg.ShowListing:
		handler = listing(cfg.Folder)
	case cfg.AllowIndex:
		handler = index(cfg.Folder)
	default:
		handler = basic(cfg.Folder)
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
		handler = withLogging(handler)
	}

	// Wrap with health check (bypasses all middleware)
	return withHealthz(handler)
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

// safePath resolves a URL path against the folder root, preventing directory traversal.
func safePath(folder, urlPath string) (string, error) {
	cleaned := filepath.Clean("/" + urlPath)
	fullPath := filepath.Join(folder, cleaned)

	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
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
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, r.URL.Path)
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
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, r.URL.Path)
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
func listing(folder string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, r.URL.Path)
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
			renderListing(w, r, fpath, r.URL.Path)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// listingAndIndex serves files, prefers index.html for directories, falls back
// to directory listing.
func listingAndIndex(folder string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fpath, err := safePath(folder, r.URL.Path)
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
			renderListing(w, r, fpath, r.URL.Path)
			return
		}

		http.ServeFile(w, r, fpath)
	}
}

// withCORS adds CORS headers to all responses.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
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

// withAccessKey validates URL parameter access key or MD5 code.
func withAccessKey(next http.Handler, accessKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Direct key match: ?key=<access_key>
		if key := query.Get("key"); key == accessKey {
			next.ServeHTTP(w, r)
			return
		}

		// MD5 code match: ?code=<MD5(path + key)>
		if code := query.Get("code"); code != "" {
			expected := fmt.Sprintf("%X", md5.Sum([]byte(r.URL.Path+accessKey)))
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
		next.ServeHTTP(w, r)
	})
}

// withLogging logs each request to stderr.
func withLogging(next http.Handler) http.Handler {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("%s %s %s %s %s referer=%q",
			r.RemoteAddr, r.Method, r.Proto, r.Host, r.URL.Path, r.Referer())
		next.ServeHTTP(w, r)
	})
}
