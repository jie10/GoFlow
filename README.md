# GoFlow

A high-performance, feature-rich HTTP router for Go web applications with support for named parameters, wildcards, route
groups, and extensive middleware capabilities. Based on alexedwards/flow but enhanced with additional features and
optimizations.

## Features

- ⚡ High Performance:
    - 40.5M requests/second for static routes
    - 6.4M requests/second for parameter routes
    - 33.2M requests/second for wildcard routes
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
    - Rate limiting
    - CORS support
    - Compression
    - Response caching
- 🎮 Easy to Use API
- 📊 Extensive Testing & Benchmarks

## Benchmark Results

```
BenchmarkParallelRequests/ParallelStaticRoute-16      40536432    31.00 ns/op  (40.5M req/sec)   0 allocs/op
BenchmarkParallelRequests/ParallelParameterRoute-16    6398863   183.90 ns/op  (6.4M req/sec)    2 allocs/op
BenchmarkParallelRequests/ParallelWildcardRoute-16    33238326    34.92 ns/op  (33.2M req/sec)   1 allocs/op
BenchmarkMethodNotAllowed-16                           5843803   206.10 ns/op                    3 allocs/op
BenchmarkNotFound-16                                   9652524   124.30 ns/op                    2 allocs/op
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
		GoFlow.Recovery(),                  // Panic recovery
		GoFlow.Logger(),                    // Request logging
		GoFlow.Timeout(30*time.Second),     // Request timeout
		GoFlow.RateLimit(100, time.Minute), // Rate limiting
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

func userHandler(w http.ResponseWriter, r *http.Request) {
	id := GoFlow.Param(r.Context(), "id")
	fmt.Fprintf(w, "User ID: %s", id)
}
```

## Built-in Middleware

```go
// Recovery
mux.Use(GoFlow.Recovery())

// Logging
mux.Use(GoFlow.Logger())

// Timeout
mux.Use(GoFlow.Timeout(30 * time.Second))

// Rate Limiting
mux.Use(GoFlow.RateLimit(100, time.Minute))

// CORS
mux.Use(GoFlow.CORS([]string{"*"}, []string{"GET", "POST"}, []string{"Content-Type"}))

// Compression
mux.Use(GoFlow.Compression())

// Caching
mux.Use(GoFlow.Cache(5 * time.Minute))
```

### Recovery Middleware

```go
mux.Use(GoFlow.Recovery())
```

### Logging Middleware

```go
mux.Use(GoFlow.Logger())
```

### Timeout Middleware

```go
mux.Use(GoFlow.Timeout(30 * time.Second))
```

### Rate Limiting Middleware

```go
mux.Use(GoFlow.RateLimit(100, time.Minute)) // 100 requests per minute
```

### CORS Middleware

```go
mux.Use(GoFlow.CORS([]string{"*"}, []string{"GET", "POST"}, []string{"Content-Type"}))
```

### Compression Middleware

```go
mux.Use(GoFlow.Compression())
```

### Cache Middleware

```go
mux.Use(GoFlow.Cache(5 * time.Minute))
```

## Performance Optimizations

GoFlow includes several performance optimizations:

- Radix tree-based routing with O(1) lookup
- Object pooling to reduce GC pressure
- Pre-compiled regex patterns
- Efficient string building
- Minimal allocations in hot paths
- Smart middleware chaining
- Memory pooling for common operations

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

Based on the excellent [alexedwards/flow](https://github.com/alexedwards/flow) router, with additional features and
optimizations.