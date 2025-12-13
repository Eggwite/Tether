package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tether/src/middleware"

	"github.com/go-chi/chi/v5"
)

func TestRateLimitMiddleware(t *testing.T) {
	r := chi.NewRouter()
	middleware.Setup(r, false) // false = not behind proxy for tests

	// Simple test handler
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	t.Run("allows requests under limit", func(t *testing.T) {
		// Should allow first request
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("rate limits excessive requests", func(t *testing.T) {
		// Send many requests rapidly from same IP
		ip := "192.168.1.2:54321"
		successCount := 0
		rateLimitedCount := 0

		// Try 100 requests in quick succession
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			switch w.Code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				rateLimitedCount++
			}
		}

		// Should have some rate limited requests
		if rateLimitedCount == 0 {
			t.Error("expected some requests to be rate limited")
		}

		// Should have allowed some requests through
		if successCount == 0 {
			t.Error("expected some requests to succeed")
		}

		t.Logf("Success: %d, Rate Limited: %d", successCount, rateLimitedCount)
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		// Two different IPs should have independent rate limits
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.1.3:12345"
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.1.4:12345"
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w1.Code != http.StatusOK {
			t.Errorf("IP1 expected 200, got %d", w1.Code)
		}
		if w2.Code != http.StatusOK {
			t.Errorf("IP2 expected 200, got %d", w2.Code)
		}
	})

	t.Run("limit resets over time", func(t *testing.T) {
		// This test would need to wait for the rate limit window to reset
		// Skip in CI/quick tests, but useful for local verification
		if testing.Short() {
			t.Skip("skipping time-dependent test in short mode")
		}

		ip := "192.168.1.5:12345"

		// Exhaust the rate limit
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}

		// Wait for rate limit window to reset (adjust based on your config)
		time.Sleep(2 * time.Second)

		// Should be allowed again
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 after reset, got %d", w.Code)
		}
	})
}
