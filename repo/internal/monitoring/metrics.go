package monitoring

import (
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	"helios-backend/internal/db"

	"github.com/gin-gonic/gin"
)

const (
	serviceName     = "backend"
	samplerInterval = 30 * time.Second
)

type httpKey struct {
	Method string
	Path   string
	Status int
}

type httpBucket struct {
	count uint64
	sumMs uint64
}

var (
	httpMu    sync.Mutex
	httpStats = make(map[httpKey]*httpBucket)
	startTime = time.Now()
)

// RequestMetrics records per-request method/path/status counts and latency.
// Buckets are flushed to performance_metrics by the sampler.
func RequestMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		k := httpKey{c.Request.Method, path, c.Writer.Status()}
		ms := uint64(time.Since(start).Milliseconds())

		httpMu.Lock()
		b, ok := httpStats[k]
		if !ok {
			b = &httpBucket{}
			httpStats[k] = b
		}
		b.count++
		b.sumMs += ms
		httpMu.Unlock()
	}
}

func drainHTTP() map[httpKey]*httpBucket {
	httpMu.Lock()
	defer httpMu.Unlock()
	out := httpStats
	httpStats = make(map[httpKey]*httpBucket)
	return out
}

// StartSampler flushes HTTP deltas and runtime gauges every samplerInterval.
func StartSampler() {
	go func() {
		t := time.NewTicker(samplerInterval)
		defer t.Stop()
		for range t.C {
			safeSample()
		}
	}()
	log.Printf("monitoring sampler started (interval=%s)", samplerInterval)
}

func safeSample() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("monitoring sampler panic: %v", r)
		}
	}()
	sample()
}

func sample() {
	// 1. HTTP counters (flushed as delta per interval)
	stats := drainHTTP()
	for k, b := range stats {
		tags := tagJSON(map[string]any{
			"method": k.Method,
			"path":   k.Path,
			"status": k.Status,
		})
		writeRow("http_requests_total", float64(b.count), "count", tags)
		if b.count > 0 {
			writeRow("http_request_ms_avg", float64(b.sumMs)/float64(b.count), "ms", tags)
		}
	}

	// 2. Runtime gauges (current values)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeRow("goroutines", float64(runtime.NumGoroutine()), "count", nil)
	writeRow("heap_alloc_bytes", float64(m.HeapAlloc), "bytes", nil)
	writeRow("heap_objects", float64(m.HeapObjects), "count", nil)
	writeRow("gc_runs_total", float64(m.NumGC), "count", nil)
	writeRow("gc_pause_ns_total", float64(m.PauseTotalNs), "ns", nil)
	writeRow("uptime_seconds", time.Since(startTime).Seconds(), "s", nil)
}

func writeRow(name string, value float64, unit string, tags any) {
	if _, err := db.DB.Exec(
		`INSERT INTO performance_metrics (service, metric_name, metric_value, unit, tags)
		 VALUES (?, ?, ?, ?, ?)`,
		serviceName, name, value, unit, tags,
	); err != nil {
		log.Printf("perf metric %s: %v", name, err)
	}
}

func tagJSON(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return string(b)
}
