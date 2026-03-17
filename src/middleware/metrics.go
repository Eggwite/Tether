package middleware

import (
	"net/http"
	"sync/atomic"
	"time"

	"tether/src/utils"
)

var apiLatency utils.LatencyRing
var totalRequests atomic.Int64

// APILatencyMiddleware measures request duration, increments the request counter,
// and records the latency sample.
func APILatencyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			totalRequests.Add(1)
			start := time.Now()
			next.ServeHTTP(w, r)
			apiLatency.Record(time.Since(start))
		})
	}
}

// APIP99 returns the p99 of the last 100 recorded request latencies.
func APIP99() time.Duration {
	return apiLatency.P99()
}

// APIRequestCount returns the total number of HTTP requests handled since startup.
func APIRequestCount() int64 {
	return totalRequests.Load()
}
