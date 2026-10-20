package handler

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// compressibleType reports whether a Content-Type is text-like enough to gain from gzip.
func compressibleType(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	if strings.HasPrefix(mt, "text/") || strings.HasSuffix(mt, "+json") || strings.HasSuffix(mt, "+xml") {
		return true
	}
	switch mt {
	case "application/json", "application/javascript", "application/x-javascript",
		"application/xml", "application/manifest+json", "image/svg+xml":
		return true
	}
	return false
}

// gzipResponseWriter decides on compression when the status is written, from the final
// status, method and Content-Type, so binaries, HEAD and partial responses pass through.
type gzipResponseWriter struct {
	http.ResponseWriter
	method      string
	acceptsGzip bool
	gz          *gzip.Writer
	wroteHeader bool
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		g.ResponseWriter.WriteHeader(code) // informational: the final status is still to come
		return
	}
	if g.wroteHeader {
		g.ResponseWriter.WriteHeader(code)
		return
	}
	g.wroteHeader = true
	h := g.Header()
	if code == http.StatusNotModified {
		// writeNotModified drops Content-Type, so vary unconditionally as the 200 may have.
		h.Add("Vary", "Accept-Encoding")
	}
	if code == http.StatusOK && h.Get("Content-Encoding") == "" && compressibleType(h.Get("Content-Type")) {
		h.Add("Vary", "Accept-Encoding")
		// HEAD stays uncompressed: there is no body, and an empty gzip stream would
		// report a 20-byte Content-Length.
		if g.acceptsGzip && g.method != http.MethodHead {
			h.Del("Content-Length")
			h.Set("Content-Encoding", "gzip")
			g.gz = gzipPool.Get().(*gzip.Writer)
			g.gz.Reset(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		// Sniff as net/http would, before the type decides on compression.
		h := g.Header()
		if _, set := h["Content-Type"]; !set && h.Get("Content-Encoding") == "" && len(b) > 0 {
			h.Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.gz != nil {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// ReadFrom keeps http.ServeContent's sendfile path for responses left uncompressed.
func (g *gzipResponseWriter) ReadFrom(src io.Reader) (int64, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if rf, ok := g.ResponseWriter.(io.ReaderFrom); ok && g.gz == nil {
		return rf.ReadFrom(src)
	}
	return io.Copy(writerOnly{g}, src)
}

// Flush implements http.Flusher by flushing the gzip writer and the underlying ResponseWriter.
func (g *gzipResponseWriter) Flush() {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.gz != nil {
		// The response is already in flight; a failed flush cannot be signalled.
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying connection.
func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

func (g *gzipResponseWriter) close() {
	if g.gz == nil {
		return
	}
	// Headers went out long ago, so a failed final flush has no reply path.
	_ = g.gz.Close()
	gzipPool.Put(g.gz)
	g.gz = nil
}

// acceptsGzip reports whether an Accept-Encoding header allows gzip; "gzip;q=0" refuses it.
func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		q := strings.ReplaceAll(strings.TrimSpace(params), " ", "")
		return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
	}
	return false
}

// writerOnly hides gzipResponseWriter's ReadFrom so io.Copy does not recurse into it.
type writerOnly struct{ io.Writer }

// withCompression gzips text-like 200 responses for clients that accept gzip. Range
// requests are never compressed: partial downloads must keep their byte offsets.
func withCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		grw := &gzipResponseWriter{
			ResponseWriter: w,
			method:         r.Method,
			acceptsGzip:    acceptsGzip(r.Header.Get("Accept-Encoding")),
		}
		defer grw.close()
		next.ServeHTTP(grw, r)
	})
}
