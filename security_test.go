package GoFlow

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkSecurityMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	opts := SecurityOptions{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		HSTS:           true,
		XSSProtection:  true,
		CSP:            "default-src 'self'",
		CSRFEnabled:    true,
		CSRFKey:        "test-key",
		RateLimit: RateLimitOptions{
			Requests:  1000,
			Duration:  time.Minute,
			BurstSize: 100,
		},
	}

	secureHandler := Security(opts)(handler)

	b.Run("ParallelSecurityStack", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			r := httptest.NewRequest("GET", "/test", nil)
			r.Header.Set("Origin", "https://example.com")
			r.Header.Set("X-Real-IP", "192.168.1.1")

			for pb.Next() {
				w := httptest.NewRecorder()
				secureHandler.ServeHTTP(w, r)
			}
		})
	})
}
