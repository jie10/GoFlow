# GoFlow

A high-performance, feature-rich HTTP router for Go web applications with support for named parameters, wildcards, route
groups, and extensive middleware capabilities. Based on alexedwards/flow but enhanced with additional features and
optimizations.

## Features

- ⚡ High Performance:
    - 9.3M requests/second for static routes
    - 6.7M requests/second for parameter routes
    - Optimized memory usage and allocations
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
- 🎮 Easy to Use API
- 📊 Extensive Testing & Benchmarks

## Benchmark Results

```
BenchmarkParallelRequests/ParallelStaticRoute-16          9300190    127.4 ns/op    344 B/op    2 allocs/op
BenchmarkParallelRequests/ParallelParameterRoute-16       6721171    189.4 ns/op    392 B/op    3 allocs/op
BenchmarkParallelRequests/ParallelWildcardRoute-16        5221261    227.2 ns/op    416 B/op    4 allocs/op
BenchmarkMethodNotAllowed-16                              2229696    520.2 ns/op    148 B/op    5 allocs/op
BenchmarkNotFound-16                                      2727050    436.8 ns/op    121 B/op    4 allocs/op
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
	"log"
	"net/http"
	"time"

	"github.com/jie10/GoFlow"
)

func main() {
	// Create a new router
	mux := GoFlow.New()

	// Add global middleware
	mux.Use(
		GoFlow.Recovery(),              // Panic recovery
		GoFlow.Logger(),                // Request logging
		GoFlow.Timeout(30*time.Second), // Request timeout
	)

	// Basic routes
	mux.Handle("/", http.HandlerFunc(homeHandler), "GET")
	mux.Handle("/about", http.HandlerFunc(aboutHandler), "GET")

	// Route with named parameter
	mux.Handle("/users/:id", http.HandlerFunc(userHandler), "GET")

	// Route with regex validation
	mux.Handle("/products/:id|^\\d+$", http.HandlerFunc(productHandler), "GET")

	// Wildcard route for static files
	mux.Handle("/static/...", http.HandlerFunc(staticHandler), "GET")

	// Route group with middleware
	mux.Group(func(m *GoFlow.Mux) {
		// Add group-specific middleware
		m.Use(authMiddleware)

		// Group routes
		m.Handle("/admin", adminHandler, "GET")
		m.Handle("/admin/users", adminUsersHandler, "GET", "POST")
	})

	// Start server
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// Handler functions
func userHandler(w http.ResponseWriter, r *http.Request) {
	// Get route parameter
	id := GoFlow.Param(r.Context(), "id")
	fmt.Fprintf(w, "User ID: %s", id)
}
```

## Built-in Middleware

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

- Radix tree-based routing for O(1) route matching
- Route parameter pooling to reduce GC pressure
- Pre-compiled regex patterns
- Efficient string building for headers
- Minimal allocations in hot paths
- Smart middleware chaining
- Memory pooling for common operations

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

Based on the excellent [alexedwards/flow](https://github.com/alexedwards/flow) router, with additional features and
optimizations.