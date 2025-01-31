# GoFlow

A high-performance, feature-rich HTTP router for Go web applications with support for named parameters, wildcards, route
groups, and extensive middleware capabilities. Based on alexedwards/flow but enhanced with additional features and
optimizations.

## Features

- ⚡ High Performance:
    - 46.8M requests/second for static routes
    - 6.48M requests/second for parameter routes
    - 36.9M requests/second for wildcard routes
- 🎯 Zero External Dependencies
- 🔒 Thread-Safe: Concurrent request handling
- 🛣️ Flexible Routing:
    - Named parameters with regex validation
    - Wildcard routes
    - Route groups
- 🔄 Middleware System:
    - Built-in middleware (Recovery, Logging, Timeout, Rate Limiting)
    - Custom middleware support
    - Per-group middleware
- 🛡️ Built-in Protection:
    - Panic recovery
    - Request timeouts
    - Advanced rate limiting with burst support
    - CORS support with configurable options
    - Compression with gzip
    - Response caching
    - CSRF protection
    - Security headers (HSTS, CSP, etc.)
- 🎮 Easy to Use API
- 📊 Extensive Testing & Benchmarks

## Benchmark Results

```
BenchmarkParallelRequests/ParallelStaticRoute-16     46836762   30.13 ns/op  (46.8M req/sec)
BenchmarkParallelRequests/ParallelParameterRoute-16   6481224  183.60 ns/op  (6.48M req/sec)
BenchmarkParallelRequests/ParallelWildcardRoute-16   36884716   33.03 ns/op  (36.9M req/sec)
BenchmarkMethodNotAllowed-16                          5940390  199.20 ns/op  (5.94M req/sec)
BenchmarkNotFound-16                                 10605184  112.30 ns/op  (10.6M req/sec)
```

## Installation

```bash
go get github.com/jie10/GoFlow
```

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/jie10/GoFlow"
	"log"
	"net/http"
	"time"
)

func main() {
	// Create router
	mux := GoFlow.New()

	// Add global middleware
	mux.Use(
		GoFlow.Recovery(),                      // Panic recovery
		GoFlow.Logger(),                        // Request logging
		GoFlow.Timeout(30*time.Second),         // Request timeout
		GoFlow.RateLimit(100, time.Minute, 20), // Rate limiting with burst
	)

	// Basic routes
	mux.Handle("/", homeHandler, "GET")
	mux.Handle("/about", aboutHandler, "GET")

	// Named parameter
	mux.Handle("/users/:id", userHandler, "GET")

	// With regex validation
	mux.Handle("/products/:id|^\\d+$", productHandler, "GET")

	// Wildcard route
	mux.Handle("/static/...", staticHandler, "GET")

	// Route group with middleware
	mux.Group(func(m *GoFlow.Mux) {
		m.Use(authMiddleware)
		m.Handle("/admin", adminHandler, "GET")
		m.Handle("/admin/users", adminUsersHandler, "GET", "POST")
	})

	// Optimize routes (optional)
	mux.Optimize()

	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## Advanced Usage

### Comprehensive Security Configuration

```go
securityOpts := GoFlow.SecurityOptions{
// CORS Configuration
AllowedOrigins: []string{"https://example.com"},
AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
AllowedHeaders: []string{"Content-Type", "Authorization"},
ExposedHeaders: []string{"X-Request-ID"},
AllowCredentials: true,
MaxAge: 3600,

// Security Headers
HSTS: true,
HSTSMaxAge: 31536000,
HSTSPreload: true,
HSTSIncludeSubdomains: true,
XSSProtection: true,
CSP: "default-src 'self'",

// Rate Limiting
RateLimit: GoFlow.RateLimitOptions{
Requests: 100,
Duration: time.Minute,
BurstSize: 20,
TrustedIPs: []string{"10.0.0.1"},
},

// CSRF Protection
CSRFEnabled: true,
CSRFKey: "your-csrf-key",

// Trusted Proxies
TrustedProxies: []string{"10.0.0.1", "10.0.0.2"},
}

mux.Use(GoFlow.Security(securityOpts))
```

### Advanced Rate Limiting

```go
// Basic rate limiting
mux.Use(GoFlow.RateLimit(100, time.Minute, 20)) // 100 req/min with 20 burst

// Custom rate limiting per route
customLimiter := GoFlow.NewRateLimiter(500, time.Minute, 50)
mux.Handle("/api", handler, "GET").With(func (next http.Handler) http.Handler {
return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
if !customLimiter.Allow(r.RemoteAddr) {
http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
return
}
next.ServeHTTP(w, r)
})
})
```

### Route Groups with Nested Middleware

```go
mux.Group(func (m *GoFlow.Mux) {
// Group level middleware
m.Use(GoFlow.Logger())
m.Use(authMiddleware)

// Nested group
m.Group(func (api *GoFlow.Mux) {
api.Use(GoFlow.RateLimit(200, time.Minute, 20))

// API routes
api.Handle("/v1/users", usersHandler, "GET", "POST")
api.Handle("/v1/products", productsHandler, "GET")

// Further nesting
api.Group(func (admin *GoFlow.Mux) {
admin.Use(adminAuthMiddleware)
admin.Handle("/v1/admin/users", adminUsersHandler, "GET")
})
})
})
```

### Custom Middleware

```go
// Custom middleware example
func customMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
// Pre-processing
start := time.Now()

// Call the next handler
next.ServeHTTP(w, r)

// Post-processing
duration := time.Since(start)
log.Printf("Request took %v", duration)
})
}

// Apply middleware
mux.Use(customMiddleware)
```

### Response Caching

```go
// Cache responses for 5 minutes
mux.Use(GoFlow.Cache(5 * time.Minute))

// Custom cache configuration per route
mux.Handle("/cached", handler, "GET").With(GoFlow.Cache(time.Minute))
```

### Compression

```go
// Enable gzip compression
mux.Use(GoFlow.Compression())
```

### Secure Headers

```go
// Basic secure headers
mux.Use(GoFlow.Security(GoFlow.SecurityOptions{
HSTS: true,
XSSProtection: true,
CSP: "default-src 'self'",
}))
```

### Parameter Handling

```go
func userHandler(w http.ResponseWriter, r *http.Request) {
// Get route parameter
userID := GoFlow.Param(r.Context(), "id")

// Use the parameter
fmt.Fprintf(w, "User ID: %s", userID)
}

// With regex validation
mux.Handle("/users/:id|^\\d+$", userHandler, "GET")
```

### File Serving

```go
// Serve static files
mux.Handle("/static/...", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
```

### Error Handlers

```go
// Custom 404 handler
mux.NotFound = http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusNotFound)
fmt.Fprint(w, "Custom 404 page")
})

// Custom method not allowed handler
mux.MethodNotAllowed = http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusMethodNotAllowed)
fmt.Fprint(w, "Method not allowed")
})
```

## Performance Optimizations

GoFlow includes several performance optimizations:

1. Routing Optimizations:

- Radix tree-based routing with O(1) lookup for static routes
- Pre-compiled regex patterns for parameter validation
- Efficient string building and path matching

2. Memory Management:

- Object pooling to reduce GC pressure
- Minimal allocations in hot paths
- Memory pooling for common operations

3. Concurrency Optimizations:

- Sharded rate limiting for reduced lock contention
- Atomic operations for concurrent access
- Lock-free paths where possible
- Smart middleware chaining

4. Cache Optimizations:

- Response caching with efficient eviction
- Header caching
- Route tree caching

## Best Practices

1. Route Organization:

- Group related routes together
- Use meaningful parameter names
- Keep regex patterns simple
- Use route groups for common prefixes

2. Middleware Usage:

- Order middleware from most to least frequently used
- Use middleware at appropriate levels (global vs group vs route)
- Implement custom middleware for specific needs
- Be mindful of middleware overhead

3. Performance:

- Enable route optimization for production
- Use appropriate cache durations
- Monitor rate limiting configurations
- Profile your application under load

4. Security:

- Configure CORS appropriately
- Use HTTPS in production
- Enable security headers
- Set appropriate rate limits
- Validate route parameters
- Enable CSRF protection for forms

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. Here are some ways you can contribute:

- Report bugs
- Suggest new features
- Improve documentation
- Add test cases
- Optimize performance
- Fix issues

## License

MIT License

## Acknowledgments

Based on the excellent [alexedwards/flow](https://github.com/alexedwards/flow) router, with additional features and
optimizations.

## Support

If you find this project useful, please consider giving it a star ⭐ on GitHub. For issues, questions, or contributions,
please visit the GitHub repository.