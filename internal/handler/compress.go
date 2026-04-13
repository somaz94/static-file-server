package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// skipCompressionExts contains file extensions that are already compressed.
var skipCompressionExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".webm": true,
	".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true,
	".zip": true, ".gz": true, ".bz2": true, ".xz": true,
	".rar": true, ".7z": true, ".tgz": true,
	".apk": true, ".ipa": true,
	".woff": true, ".woff2": true,
	".pdf": true,
	".exe": true, ".bin": true, ".dll": true, ".so": true, ".dylib": true, ".wasm": true,
}

var gzipPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipResponseWriter wraps http.ResponseWriter with gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.writer.Write(b)
}

// Flush implements http.Flusher by flushing the gzip writer and the underlying ResponseWriter.
func (g *gzipResponseWriter) Flush() {
	g.writer.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withCompression adds gzip compression for text-based responses.
// It skips compression for Range requests and already-compressed file types.
func withCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if client doesn't accept gzip.
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for Range requests (partial downloads must not be compressed).
		if r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for already-compressed file extensions.
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		if skipCompressionExts[ext] {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		defer gzipPool.Put(gz)
		gz.Reset(w)

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.Header().Set("Vary", "Accept-Encoding")

		grw := &gzipResponseWriter{ResponseWriter: w, writer: gz}
		next.ServeHTTP(grw, r)
		gz.Close()
	})
}
