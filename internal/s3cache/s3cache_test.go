package s3cache

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, mutate func(*Options)) *Server {
	t.Helper()
	opts := Options{Folder: t.TempDir(), AccessKey: testAK, SecretKey: testSK}
	if mutate != nil {
		mutate(&opts)
	}
	s := New(opts)
	s.now = func() time.Time { return testNow }
	return s
}

func do(t *testing.T, s *Server, method, path string, signMethod string, headers http.Header, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	signed := http.Header{}
	for k, v := range headers {
		if k == "Content-Type" || strings.HasPrefix(k, "X-Amz-Meta-") {
			signed[k] = v
		}
	}
	target := presign(t, signMethod, path, signed, testNow, 3600)
	r := httptest.NewRequest(method, "http://"+testHost+target, bytes.NewReader(body))
	for k, v := range headers {
		r.Header[k] = v
	}
	if body == nil {
		r.ContentLength = 0
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func putObject(t *testing.T, s *Server, path string, body []byte, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	if headers == nil {
		headers = http.Header{}
	}
	return do(t, s, http.MethodPut, path, http.MethodPut, headers, body)
}

func TestPutGetRoundTrip(t *testing.T) {
	s := newTestServer(t, nil)
	body := []byte("cache archive bytes")
	headers := http.Header{
		"Content-Type":        {"application/zip"},
		"X-Amz-Meta-Cachekey": {"default-protected"},
		"Last-Modified":       {"Mon, 01 Jan 2024 00:00:00 GMT"},
	}

	w := putObject(t, s, "/runner-cache/runner/abc/project/1/default", body, headers)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	if etag != `"602f3854a57ba156364aab4e49bbea17"` {
		t.Fatalf("unexpected ETag %q", etag)
	}

	w = do(t, s, http.MethodGet, "/runner-cache/runner/abc/project/1/default", http.MethodGet, nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != string(body) {
		t.Fatalf("GET: %d %q", w.Code, w.Body.String())
	}
	h := w.Header()
	if h.Get("ETag") != etag || h.Get("Content-Type") != "application/zip" ||
		h.Get("X-Amz-Meta-Cachekey") != "default-protected" || h.Get("Last-Modified") == "" ||
		h.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("unexpected GET headers: %v", h)
	}
}

func TestGetRangeProbe(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("0123456789"), nil)

	w := do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, http.Header{"Range": {"bytes=0-0"}}, nil)
	if w.Code != http.StatusPartialContent || w.Header().Get("Content-Range") != "bytes 0-0/10" || w.Body.String() != "0" {
		t.Fatalf("range probe: %d %q %q", w.Code, w.Header().Get("Content-Range"), w.Body.String())
	}

	w = do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, http.Header{"Range": {"bytes=4-7"}}, nil)
	if w.Code != http.StatusPartialContent || w.Body.String() != "4567" {
		t.Fatalf("range chunk: %d %q", w.Code, w.Body.String())
	}
}

func TestHead(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("abc"), nil)

	for _, signMethod := range []string{http.MethodHead, http.MethodGet} {
		w := do(t, s, http.MethodHead, "/b-1/k", signMethod, nil, nil)
		if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Last-Modified") == "" ||
			w.Header().Get("Content-Length") != "3" || w.Header().Get("Content-Type") != "application/octet-stream" {
			t.Fatalf("HEAD signed for %s: %d len=%d %v", signMethod, w.Code, w.Body.Len(), w.Header())
		}
	}

	w := do(t, s, http.MethodHead, "/b-1/missing", http.MethodHead, nil, nil)
	if w.Code != http.StatusNotFound || w.Body.Len() != 0 {
		t.Fatalf("HEAD missing: %d %q", w.Code, w.Body.String())
	}

	// A PUT-signed URL must not authorize HEAD.
	w = do(t, s, http.MethodHead, "/b-1/k", http.MethodPut, nil, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("HEAD with PUT signature: %d", w.Code)
	}
}

func TestErrors(t *testing.T) {
	s := newTestServer(t, func(o *Options) {
		o.Buckets = []string{"allowed"}
		o.MaxObjectSize = 5
	})

	tests := []struct {
		name       string
		method     string
		path       string
		signMethod string
		body       []byte
		wantStatus int
		wantCode   string
	}{
		{"missing key", http.MethodGet, "/allowed/nope", http.MethodGet, nil, 404, "NoSuchKey"},
		{"bucket not allowed", http.MethodGet, "/other/k", http.MethodGet, nil, 404, "NoSuchBucket"},
		{"bucket-level request", http.MethodGet, "/allowed", http.MethodGet, nil, 501, "NotImplemented"},
		{"delete unsupported", http.MethodDelete, "/allowed/k", http.MethodDelete, nil, 501, "NotImplemented"},
		{"dot-dot key", http.MethodGet, "/allowed/a/../b", http.MethodGet, nil, 400, "InvalidRequest"},
		{"empty segment", http.MethodGet, "/allowed/a//b", http.MethodGet, nil, 400, "InvalidRequest"},
		{"too large", http.MethodPut, "/allowed/k", http.MethodPut, []byte("123456"), 400, "EntityTooLarge"},
		{"wrong signature method", http.MethodPut, "/allowed/k", http.MethodGet, []byte("1"), 403, "SignatureDoesNotMatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := do(t, s, tt.method, tt.path, tt.signMethod, nil, tt.body)
			if w.Code != tt.wantStatus || !strings.Contains(w.Body.String(), "<Code>"+tt.wantCode+"</Code>") {
				t.Fatalf("got %d %s", w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
				t.Fatalf("error Content-Type %q", ct)
			}
		})
	}
}

func TestUnsignedRequestDenied(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("secret"), nil)

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://"+testHost+"/b-1/k", nil))
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("unsigned GET: %d %q", w.Code, w.Body.String())
	}
}

func TestPutBodyChecks(t *testing.T) {
	s := newTestServer(t, nil)

	target := presign(t, http.MethodPut, "/b-1/k", nil, testNow, 3600)

	r := httptest.NewRequest(http.MethodPut, "http://"+testHost+target, strings.NewReader("abc"))
	r.ContentLength = -1
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusLengthRequired {
		t.Fatalf("chunked PUT: %d", w.Code)
	}

	r = httptest.NewRequest(http.MethodPut, "http://"+testHost+target, strings.NewReader("abc"))
	r.ContentLength = 10
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "IncompleteBody") {
		t.Fatalf("short body: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(s.opts.Folder, "b-1", "k")); !os.IsNotExist(err) {
		t.Fatalf("short body left an object behind: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.opts.Folder, tmpDir)); len(entries) != 0 {
		t.Fatalf("short body left %d temp file(s)", len(entries))
	}
}

func TestPutOverwriteAndConflict(t *testing.T) {
	s := newTestServer(t, nil)

	putObject(t, s, "/b-1/k", []byte("v1"), http.Header{"X-Amz-Meta-Old": {"x"}})
	putObject(t, s, "/b-1/k", []byte("v2"), nil)
	w := do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, nil, nil)
	if w.Body.String() != "v2" || w.Header().Get("X-Amz-Meta-Old") != "" {
		t.Fatalf("overwrite: %q meta=%q", w.Body.String(), w.Header().Get("X-Amz-Meta-Old"))
	}

	// "k" is a file, so "k/child" needs it to be a directory.
	if w := putObject(t, s, "/b-1/k/child", []byte("x"), nil); w.Code != http.StatusConflict {
		t.Fatalf("file-as-dir conflict: %d %s", w.Code, w.Body.String())
	}

	putObject(t, s, "/b-1/dir/child", []byte("x"), nil)
	if w := putObject(t, s, "/b-1/dir", []byte("x"), nil); w.Code != http.StatusConflict {
		t.Fatalf("dir-as-file conflict: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, s, http.MethodGet, "/b-1/dir", http.MethodGet, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("GET on a directory: %d", w.Code)
	}
}

func TestMissingMetadataSidecar(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("abc"), nil)
	if err := os.RemoveAll(filepath.Join(s.opts.Folder, metaDir)); err != nil {
		t.Fatal(err)
	}
	w := do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("ETag") != "" || w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("GET without sidecar: %d %v", w.Code, w.Header())
	}
}

func TestStoreIOErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t, func(o *Options) { o.Folder = root })

	if w := putObject(t, s, "/b-1/k", []byte("x"), nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("PUT on broken root: %d", w.Code)
	}

	st := &store{root: root}
	if _, err := st.put("Bad_Bucket", "k", strings.NewReader(""), 0, objectMeta{}); err != errInvalidName {
		t.Fatalf("put invalid name: %v", err)
	}
	if _, _, _, err := st.open("b-1", ""); err != errInvalidName {
		t.Fatalf("open invalid name: %v", err)
	}
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t, nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	body, _ := io.ReadAll(w.Body)
	if w.Code != http.StatusOK || string(body) != "ok" {
		t.Fatalf("healthz: %d %q", w.Code, body)
	}
}

func TestDebugLogging(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.Debug = true })
	if w := do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("GET: %d", w.Code)
	}
}

func TestValidName(t *testing.T) {
	tests := []struct {
		bucket, key string
		want        bool
	}{
		{"runner-cache", "runner/a/project/1/k", true},
		{"ab", "k", false},          // bucket too short
		{"Upper", "k", false},       // uppercase bucket
		{".meta", "k", false},       // reserved prefix can never be a bucket
		{"b-1", "", false},          // empty key
		{"b-1", "a/./b", false},     // dot segment
		{"b-1", "a\x00b", false},    // NUL byte
		{"b-1", ".hidden/ok", true}, // dot-prefixed but not a dot segment
	}
	for _, tt := range tests {
		if got := validName(tt.bucket, tt.key); got != tt.want {
			t.Errorf("validName(%q, %q) = %v, want %v", tt.bucket, tt.key, got, tt.want)
		}
	}
}
