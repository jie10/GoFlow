# GoFlow

A high-performance, feature-rich HTTP router for Go web applications with support for named parameters, wildcards, route
groups, and extensive middleware capabilities. Based on alexedwards/flow but enhanced with additional features and
optimizations.

## Features

- ⚡ High Performance: Optimized route matching with radix tree implementation
- 🎯 Zero External Dependencies: Pure Go implementation
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

## Route Patterns

### Basic Routes

```go
mux.Handle("/", handler, "GET")
mux.Handle("/about", handler, "GET", "POST")
```

### Named Parameters

```go
// Match: /users/123
mux.Handle("/users/:id", handler, "GET")

// Access parameter in handler
func handler(w http.ResponseWriter, r *http.Request) {
id := GoFlow.Param(r.Context(), "id")
}
```

### Regex Validation

```go
// Only match numeric IDs
mux.Handle("/users/:id|^\\d+$", handler, "GET")

// Custom regex pattern
mux.Handle("/posts/:slug|^[a-z0-9-]+$", handler, "GET")
```

### Wildcard Routes

```go
// Match any path after /static/
mux.Handle("/static/...", handler, "GET")
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

## Custom Middleware

```go
func customMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
// Do something before
next.ServeHTTP(w, r)
// Do something after
})
}

mux.Use(customMiddleware)
```

## Route Groups

```go
mux.Group(func (m *GoFlow.Mux) {
// Group middleware
m.Use(authMiddleware)

// Group routes
m.Handle("/admin", adminHandler, "GET")

// Nested group
m.Group(func (m *GoFlow.Mux) {
m.Use(superAdminMiddleware)
m.Handle("/admin/settings", settingsHandler, "GET")
})
})
```

## Error Handling

### Custom Not Found Handler

```go
mux := GoFlow.New()
mux.NotFound = http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusNotFound)
fmt.Fprintf(w, "Custom 404 - Page not found")
})
```

### Custom Method Not Allowed Handler

```go
mux.MethodNotAllowed = http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusMethodNotAllowed)
fmt.Fprintf(w, "Method not allowed")
})
```

## Performance Optimizations

GoFlow includes several performance optimizations:

- Radix tree-based routing for O(1) route matching
- Route parameter pooling to reduce GC pressure
- Pre-compiled regex patterns
- Efficient string building for headers
- Minimal allocations in hot paths

## Benchmarks

Run the benchmarks:

```bash
go test -bench=. -benchmem
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

Based on the excellent [alexedwards/flow](https://github.com/alexedwards/flow) router, with additional features and
optimizations.