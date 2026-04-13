package handler

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// metricsCollector tracks request metrics without external dependencies.
type metricsCollector struct {
	mu                 sync.RWMutex
	requestCounts      map[string]*atomic.Int64 // "method:status" -> count
	responseBytesTotal atomic.Int64
	durationBuckets    []*durationBucket
	durationSum        atomic.Uint64 // float64 bits via math.Float64bits/Float64frombits
	durationCount      atomic.Int64
}

type durationBucket struct {
	le    float64 // upper bound in seconds
	count atomic.Int64
}

var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func newMetricsCollector() *metricsCollector {
	buckets := make([]*durationBucket, len(defaultBuckets))
	for i, le := range defaultBuckets {
		buckets[i] = &durationBucket{le: le}
	}
	return &metricsCollector{
		requestCounts:   make(map[string]*atomic.Int64),
		durationBuckets: buckets,
	}
}

func (m *metricsCollector) record(method string, status int, duration time.Duration, bytes int64) {
	key := fmt.Sprintf("%s:%d", method, status)
	m.mu.RLock()
	counter, ok := m.requestCounts[key]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		counter, ok = m.requestCounts[key]
		if !ok {
			counter = &atomic.Int64{}
			m.requestCounts[key] = counter
		}
		m.mu.Unlock()
	}

	counter.Add(1)
	m.responseBytesTotal.Add(bytes)
	m.durationCount.Add(1)

	// Atomic float64 add via CAS loop.
	secs := duration.Seconds()
	for {
		old := m.durationSum.Load()
		new := math.Float64bits(math.Float64frombits(old) + secs)
		if m.durationSum.CompareAndSwap(old, new) {
			break
		}
	}

	for _, b := range m.durationBuckets {
		if secs <= b.le {
			b.count.Add(1)
			break
		}
	}
}

func (m *metricsCollector) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Request counts
		m.mu.RLock()
		keys := make([]string, 0, len(m.requestCounts))
		for k := range m.requestCounts {
			keys = append(keys, k)
		}
		m.mu.RUnlock()
		sort.Strings(keys)

		fmt.Fprintln(w, "# HELP static_file_server_requests_total Total number of HTTP requests.")
		fmt.Fprintln(w, "# TYPE static_file_server_requests_total counter")
		for _, key := range keys {
			m.mu.RLock()
			counter := m.requestCounts[key]
			m.mu.RUnlock()
			method, statusStr, _ := strings.Cut(key, ":")
			fmt.Fprintf(w, "static_file_server_requests_total{method=%q,status=%q} %d\n",
				method, statusStr, counter.Load())
		}

		// Response bytes
		fmt.Fprintln(w, "# HELP static_file_server_response_bytes_total Total bytes sent in responses.")
		fmt.Fprintln(w, "# TYPE static_file_server_response_bytes_total counter")
		fmt.Fprintf(w, "static_file_server_response_bytes_total %d\n", m.responseBytesTotal.Load())

		// Duration histogram (cumulative buckets per Prometheus convention)
		fmt.Fprintln(w, "# HELP static_file_server_request_duration_seconds Request duration histogram.")
		fmt.Fprintln(w, "# TYPE static_file_server_request_duration_seconds histogram")
		var cumulative int64
		for _, b := range m.durationBuckets {
			cumulative += b.count.Load()
			fmt.Fprintf(w, "static_file_server_request_duration_seconds_bucket{le=\"%.3f\"} %d\n",
				b.le, cumulative)
		}
		total := m.durationCount.Load()
		fmt.Fprintf(w, "static_file_server_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", total)
		fmt.Fprintf(w, "static_file_server_request_duration_seconds_sum %f\n",
			math.Float64frombits(m.durationSum.Load()))
		fmt.Fprintf(w, "static_file_server_request_duration_seconds_count %d\n", total)
	}
}

func (m *metricsCollector) totalRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total int64
	for _, c := range m.requestCounts {
		total += c.Load()
	}
	return total
}

// withMetrics wraps a handler to collect request metrics and serves /metrics.
func withMetrics(next http.Handler) http.Handler {
	collector := newMetricsCollector()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			collector.handler().ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		collector.record(r.Method, rec.status, time.Since(start), rec.bytes)
	})
}
