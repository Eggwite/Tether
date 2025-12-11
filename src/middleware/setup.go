package middleware

import (
	"github.com/go-chi/chi/v5"
)

// Setup registers the global middleware stack on the router.
func Setup(r *chi.Mux, behindProxy bool) {
	// 10 req/s
	r.Use(APILatencyMiddleware())
	r.Use(RateLimitMiddleware(10, behindProxy))
}
