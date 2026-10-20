// Package s3cache serves the subset of the S3 API that the GitLab Runner
// distributed cache uses: presigned (SigV4 query) GET, HEAD and PUT on
// path-style object URLs. It is not a general-purpose S3 implementation.
package s3cache

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/somaz94/static-file-server/internal/httpx"
)

// Options configures the S3 cache endpoint.
type Options struct {
	Folder        string
	AccessKey     string
	SecretKey     string
	Buckets       []string      // empty = any valid bucket name
	MaxObjectSize int64         // 0 = unlimited
	MaxTotalSize  int64         // 0 = unlimited; enforced by the janitor, least recently used first
	TTL           time.Duration // since the last upload or read
	Debug         bool
	Metrics       bool // serve /metrics on the cache listener
}

// Server is the S3 cache HTTP handler.
type Server struct {
	opts    Options
	store   *store
	metrics *metrics
	now     func() time.Time
}

// New returns a Server rooted at opts.Folder.
func New(opts Options) *Server {
	return &Server{opts: opts, store: &store{root: opts.Folder}, metrics: newMetrics(), now: time.Now}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && len(r.URL.Query()) == 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	if s.opts.Metrics && r.URL.Path == "/metrics" && len(r.URL.Query()) == 0 {
		s.metrics.serve(w, s.opts.MaxTotalSize)
		return
	}

	rec := httpx.NewRecorder(w)
	start := time.Now()
	s.serve(rec, r)
	s.metrics.recordRequest(httpx.MetricMethod(r.Method), rec.Status, rec.Bytes)
	if s.opts.Debug {
		log.Printf("s3cache %s %s %d %s", r.Method, r.URL.Path, rec.Status, time.Since(start))
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")

	if e := s.authorize(r); e != nil {
		log.Printf("s3cache: denied %s bucket=%q: %s", r.Method, bucket, e.Message)
		e.write(w, r)
		return
	}

	if key == "" {
		errNotImplemented().write(w, r)
		return
	}
	if len(s.opts.Buckets) > 0 && !slices.Contains(s.opts.Buckets, bucket) {
		errNoSuchBucket().write(w, r)
		return
	}
	if !validName(bucket, key) {
		errInvalidRequest("invalid bucket name or object key").write(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.get(w, r, bucket, key)
	case http.MethodPut:
		s.put(w, r, bucket, key)
	default:
		errNotImplemented().write(w, r)
	}
}

func (s *Server) authorize(r *http.Request) *s3Error {
	now := s.now()
	e := verifyPresigned(r, r.Method, s.opts.AccessKey, s.opts.SecretKey, now)
	// The runner may probe with HEAD on a URL presigned for GET; it exposes nothing GET would not.
	if e != nil && e.Code == "SignatureDoesNotMatch" && r.Method == http.MethodHead {
		e = verifyPresigned(r, http.MethodGet, s.opts.AccessKey, s.opts.SecretKey, now)
	}
	return e
}

func (s *Server) get(w http.ResponseWriter, r *http.Request, bucket, key string) {
	f, meta, fi, err := s.store.open(bucket, key)
	if err != nil {
		if errors.Is(err, errNotFound) {
			errNoSuchKey().write(w, r)
			return
		}
		log.Printf("s3cache: open %s/%s: %v", bucket, key, err)
		errInternal().write(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	s.store.touch(bucket, key, s.now())

	h := w.Header()
	ct := meta.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	h.Set("Content-Type", ct)
	if meta.ETag != "" {
		h.Set("ETag", meta.ETag)
	}
	for k, v := range meta.UserMeta {
		h.Set("X-Amz-Meta-"+k, v)
	}
	h.Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *Server) put(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if r.ContentLength < 0 {
		errMissingContentLength().write(w, r)
		return
	}
	if (s.opts.MaxObjectSize > 0 && r.ContentLength > s.opts.MaxObjectSize) ||
		(s.opts.MaxTotalSize > 0 && r.ContentLength > s.opts.MaxTotalSize) {
		errEntityTooLarge().write(w, r)
		return
	}

	meta := objectMeta{ContentType: r.Header.Get("Content-Type"), UserMeta: userMeta(r.Header)}
	etag, err := s.store.put(bucket, key, r.Body, r.ContentLength, meta)
	switch {
	case err == nil:
		s.metrics.receivedBytes.Add(r.ContentLength)
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, errShortBody):
		errIncompleteBody().write(w, r)
	case errors.Is(err, errConflict):
		errKeyConflict().write(w, r)
	default:
		log.Printf("s3cache: put %s/%s: %v", bucket, key, err)
		errInternal().write(w, r)
	}
}

func userMeta(h http.Header) map[string]string {
	var m map[string]string
	for k := range h {
		lk := strings.ToLower(k)
		if name, ok := strings.CutPrefix(lk, "x-amz-meta-"); ok && name != "" {
			if m == nil {
				m = map[string]string{}
			}
			m[name] = h.Get(k)
		}
	}
	return m
}
