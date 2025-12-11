package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware limits requests per IP using a non-blocking token bucket.
// Exceeding requests are rejected immediately with 429 and a Retry-After header.
func RateLimitMiddleware(requestsPerSecond int, behindProxy bool) func(http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Cleanup routine for stale clients
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	burst := requestsPerSecond // allow short bursts up to the per-second rate

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r, behindProxy)

			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				c = &client{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst)}
				clients[ip] = c
			}
			c.lastSeen = time.Now()
			mu.Unlock()

			// Non-blocking: reserve a token and reject if it would require waiting.
			res := c.limiter.Reserve()
			if !res.OK() {
				writeRateLimited(w, requestsPerSecond, time.Second)
				return
			}

			if delay := res.Delay(); delay > 0 {
				res.Cancel() // do not consume the token if we're rejecting
				writeRateLimited(w, requestsPerSecond, delay)
				return
			}

			// Token consumed, proceed.
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the real client IP, checking proxy headers if behindProxy is true
func getClientIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		// Check Cloudflare-specific header (most reliable)
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}

		// Check X-Forwarded-For (take first IP in the chain)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return ip
		}
	}

	// Fallback to RemoteAddr (strip port, handle IPv6)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// writeRateLimited writes a 429 with Retry-After and basic rate-limit headers.
func writeRateLimited(w http.ResponseWriter, limit int, delay time.Duration) {
	retryAfterSeconds := max(int(math.Ceil(delay.Seconds())), 1)
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Duration(retryAfterSeconds)*time.Second).Unix(), 10))
	http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
}
