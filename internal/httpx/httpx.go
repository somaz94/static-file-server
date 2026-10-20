// Package httpx holds HTTP helpers shared by the file server and the S3 cache endpoint.
package httpx

import (
	"io"
	"net/http"
)

// MetricMethod maps a request method to a bounded metric label. net/http accepts any
// method token, so labelling by r.Method lets a client mint unlimited series.
func MetricMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodOptions, http.MethodPatch:
		return method
	}
	return "OTHER"
}

// Recorder wraps an http.ResponseWriter to capture the status code and bytes written.
type Recorder struct {
	http.ResponseWriter
	Status int
	Bytes  int64
}

// NewRecorder returns a Recorder whose status defaults to 200, as net/http's does.
func NewRecorder(w http.ResponseWriter) *Recorder {
	return &Recorder{ResponseWriter: w, Status: http.StatusOK}
}

func (r *Recorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *Recorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.Bytes += int64(n)
	return n, err
}

// ReadFrom forwards to the wrapped writer so http.ServeContent keeps its sendfile path.
func (r *Recorder) ReadFrom(src io.Reader) (int64, error) {
	if rf, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(src)
		r.Bytes += n
		return n, err
	}
	return io.Copy(writerOnly{r}, src)
}

// Flush implements http.Flusher when the wrapped writer does.
func (r *Recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying connection.
func (r *Recorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// writerOnly hides Recorder's ReadFrom so io.Copy does not recurse into it.
type writerOnly struct{ io.Writer }
