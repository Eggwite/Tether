package middleware

import (
	"net/http"
	"time"

	"tether/src/utils"
)

var apiLatency utils.LatencyRing

// APILatencyMiddleware measures request duration and records it for percentile stats.
func APILatencyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			apiLatency.Record(time.Since(start))
		})
	}
}

// APIP99 returns the 99th percentile of the last 100 recorded request latencies.
func APIP99() time.Duration {
	return apiLatency.P99()
}
