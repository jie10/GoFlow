package GoFlow

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// SecurityOptions configures security middleware
type SecurityOptions struct {
	// CORS options
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int

	// Rate limiting options
	RateLimit RateLimitOptions

	// Security headers
	HSTS                  bool
	HSTSMaxAge            int
	HSTSPreload           bool
	HSTSIncludeSubdomains bool

	// XSS Protection
	XSSProtection bool

	// Content Security Policy
	CSP string

	// Trusted proxies
	TrustedProxies []string

	// CSRF Protection
	CSRFEnabled bool
	CSRFKey     string
}

type RateLimitOptions struct {
	Requests   int
	Duration   time.Duration
	TrustedIPs []string
	BurstSize  int
}

var (
	// Precompiled regex for origin validation
	originRegex = regexp.MustCompile(`^https?://[\w\-\.]+(:\d+)?$`)

	// Precompiled regex for IP validation
	ipRegex = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
)

// Security middleware that combines multiple security features
func Security(opts SecurityOptions) func(http.Handler) http.Handler {
	if opts.HSTSMaxAge == 0 {
		opts.HSTSMaxAge = 31536000 // 1 year
	}

	// Set default burst size if not specified
	if opts.RateLimit.BurstSize == 0 {
		opts.RateLimit.BurstSize = opts.RateLimit.Requests / 10 // Default to 10% of base rate
	}

	trustedProxies := make(map[string]struct{})
	for _, ip := range opts.TrustedProxies {
		trustedProxies[ip] = struct{}{}
	}

	// Initialize rate limiter with burst parameter
	rateLimiter := NewRateLimiter(
		opts.RateLimit.Requests,
		opts.RateLimit.Duration,
		opts.RateLimit.BurstSize,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setSecurityHeaders(w, opts)

			if !handleCORS(w, r, opts) {
				http.Error(w, "Invalid CORS request", http.StatusForbidden)
				return
			}

			clientIP := getRealIP(r, trustedProxies)

			if !rateLimiter.Allow(clientIP) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			if opts.CSRFEnabled && !validateCSRF(r, opts.CSRFKey) {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setSecurityHeaders(w http.ResponseWriter, opts SecurityOptions) {
	// HSTS
	if opts.HSTS {
		hstsValue := fmt.Sprintf("max-age=%d", opts.HSTSMaxAge)
		if opts.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if opts.HSTSPreload {
			hstsValue += "; preload"
		}
		w.Header().Set("Strict-Transport-Security", hstsValue)
	}

	// XSS Protection
	if opts.XSSProtection {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	}

	// Basic security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Content Security Policy
	if opts.CSP != "" {
		w.Header().Set("Content-Security-Policy", opts.CSP)
	}
}

func handleCORS(w http.ResponseWriter, r *http.Request, opts SecurityOptions) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Same origin request
	}

	// Validate origin format
	if !originRegex.MatchString(origin) {
		return false
	}

	// Check if origin is allowed
	allowed := false
	for _, allowedOrigin := range opts.AllowedOrigins {
		if allowedOrigin == "*" {
			if !opts.AllowCredentials { // Don't allow wildcard with credentials
				w.Header().Set("Access-Control-Allow-Origin", "*")
				allowed = true
				break
			}
		} else if allowedOrigin == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			allowed = true
			break
		}
	}

	if !allowed {
		return false
	}

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowedHeaders, ", "))
		w.Header().Set("Access-Control-Max-Age", toString(opts.MaxAge))
		if opts.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}

	return true
}

func getRealIP(r *http.Request, trustedProxies map[string]struct{}) string {
	// Get immediate client IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// Only process X-Forwarded-For if from trusted proxy
	if _, trusted := trustedProxies[ip]; trusted {
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			ips := strings.Split(forwardedFor, ",")
			// Get the original client IP (first in the chain)
			clientIP := strings.TrimSpace(ips[0])
			if ipRegex.MatchString(clientIP) {
				return clientIP
			}
		}
	}

	return ip
}

func validateCSRF(r *http.Request, key string) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead ||
		r.Method == http.MethodOptions || r.Method == http.MethodTrace {
		return true // No CSRF check needed for safe methods
	}

	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrf_token")
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1
}

// Usage example:
/*
opts := SecurityOptions{
    AllowedOrigins: []string{"https://example.com"},
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders: []string{"Content-Type", "Authorization"},
    HSTS: true,
    HSTSMaxAge: 31536000,
    XSSProtection: true,
    CSP: "default-src 'self'",
    TrustedProxies: []string{"10.0.0.1", "10.0.0.2"},
    CSRFEnabled: true,
    CSRFKey: "your-csrf-key",
    RateLimit: RateLimitOptions{
        Requests: 100,
        Duration: time.Minute,
        BurstSize: 5,
    },
}

mux.Use(Security(opts))
*/
