package s3cache

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// metrics tracks cache endpoint activity without external dependencies.
type metrics struct {
	mu       sync.Mutex
	requests map[string]int64 // "method:status" -> count

	receivedBytes atomic.Int64
	sentBytes     atomic.Int64
	evictedTTL    atomic.Int64
	evictedSize   atomic.Int64
	objects       atomic.Int64 // as of the last sweep
	storedBytes   atomic.Int64 // as of the last sweep
}

func newMetrics() *metrics {
	return &metrics{requests: map[string]int64{}}
}

func (m *metrics) recordRequest(method string, status int, sent int64) {
	m.mu.Lock()
	m.requests[fmt.Sprintf("%s:%d", method, status)]++
	m.mu.Unlock()
	m.sentBytes.Add(sent)
}

func (m *metrics) serve(w http.ResponseWriter, maxTotal int64) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	m.mu.Lock()
	keys := make([]string, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	counts := make([]int64, len(keys))
	for i, k := range keys {
		counts[i] = m.requests[k]
	}
	m.mu.Unlock()

	fmt.Fprintln(w, "# HELP static_file_server_s3_requests_total Cache endpoint requests by method and status.")
	fmt.Fprintln(w, "# TYPE static_file_server_s3_requests_total counter")
	for i, k := range keys {
		method, status, _ := strings.Cut(k, ":")
		fmt.Fprintf(w, "static_file_server_s3_requests_total{method=%q,status=%q} %d\n", method, status, counts[i])
	}

	counter := func(name, help string, v int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, v)
	}
	gauge := func(name, help string, v int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, v)
	}
	counter("static_file_server_s3_received_bytes_total", "Bytes stored by successful uploads.", m.receivedBytes.Load())
	counter("static_file_server_s3_sent_bytes_total", "Bytes sent in responses.", m.sentBytes.Load())

	fmt.Fprintln(w, "# HELP static_file_server_s3_evicted_objects_total Objects deleted by the janitor, by reason.")
	fmt.Fprintln(w, "# TYPE static_file_server_s3_evicted_objects_total counter")
	fmt.Fprintf(w, "static_file_server_s3_evicted_objects_total{reason=\"ttl\"} %d\n", m.evictedTTL.Load())
	fmt.Fprintf(w, "static_file_server_s3_evicted_objects_total{reason=\"size\"} %d\n", m.evictedSize.Load())

	gauge("static_file_server_s3_objects", "Stored objects as of the last janitor sweep.", m.objects.Load())
	gauge("static_file_server_s3_stored_bytes", "Stored object bytes as of the last janitor sweep.", m.storedBytes.Load())
	gauge("static_file_server_s3_max_total_bytes", "Configured S3_MAX_TOTAL_SIZE (0 = unlimited).", maxTotal)
}
