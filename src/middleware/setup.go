package middleware

import (
	"github.com/go-chi/chi/v5"
	chi_mw "github.com/go-chi/chi/v5/middleware"
)

// Setup registers the global middleware stack on the router.
func Setup(r *chi.Mux, behindProxy bool) {
	// Recoverer should be the first middleware so it catches panics from
	// downstream handlers and converts them to 500 responses instead of
	// crashing the whole process.
	r.Use(chi_mw.Recoverer)
	// 10 req/s
	r.Use(APILatencyMiddleware())
	r.Use(RateLimitMiddleware(10, behindProxy))
}
