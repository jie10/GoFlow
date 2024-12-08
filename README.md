# GoFlow
A lightweight and efficient HTTP router for Go web applications with support for named parameters, wildcards, and route groups.
This package is based on alexedwards/flow, a delightfully simple, readable, and tiny HTTP router for Go web applications.

## Attribution
Original work by Alex Edwards (@alexedwards). This is a modified version of the flow package with some adaptations and improvements.

### Features
- Simple and intuitive API
- Named URL parameters
- Optional regex pattern validation
- Wildcard routing
- Automatic handling of HEAD and OPTIONS requests
- Route grouping with middleware support
- Customizable NotFound and MethodNotAllowed handlers
- Zero external dependencies

### Installation
```
go get github.com/jie10/GoFlow
```

### Quick Start
```go
package main

import (
    "fmt"
    "log"
    "net/http"
    
    "github.com/jie10/GoFlow"
)

func main() {
    // Create a new router
    mux := GoFlow.New()
    
    // Add a simple route with named parameter
    mux.Handle("/greet/:name", http.HandlerFunc(greet), "GET")
    
    // Start the server
    log.Fatal(http.ListenAndServe(":2323", mux))
}

func greet(w http.ResponseWriter, r *http.Request) {
    // Get the value of the 'name' parameter
    name := GoFlow.Param(r.Context(), "name")
    fmt.Fprintf(w, "Hello %s", name)
}
```

## Basic Usage

### Simple Routes
```go
// Basic route
mux.Handle("/", http.HandlerFunc(homeHandler), "GET")

// Multiple HTTP methods
mux.Handle("/api/users", http.HandlerFunc(usersHandler), "GET", "POST")
```

### Named Parameters
```go
// Using named parameters
mux.Handle("/users/:id", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    userID := GoFlow.Param(r.Context(), "id")
    fmt.Fprintf(w, "User ID: %s", userID)
}), "GET")

// With regex pattern
mux.Handle("/users/:id|^[0-9]+$", http.HandlerFunc(userHandler), "GET")
```

### Wildcard Routes
```go
// Match any path after /static/
mux.Handle("/static/...", http.HandlerFunc(staticHandler), "GET")
```

### Route Groups
```go
mux.Group(func(m *GoFlow.Mux) {
    // Add middleware for this group
    m.Use(authMiddleware)
    
    // Group routes
    m.Handle("/admin", http.HandlerFunc(adminHandler), "GET")
    
    // Nested groups
    m.Group(func(m *GoFlow.Mux) {
        m.Use(superAdminMiddleware)
        m.Handle("/admin/settings", http.HandlerFunc(settingsHandler), "GET")
    })
})
```

### Middleware Example
```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

// Apply middleware
mux.Use(loggingMiddleware)
```

### Custom Error Handlers
```go
mux := GoFlow.New()

// Custom 404 handler
mux.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNotFound)
    fmt.Fprintf(w, "Custom 404 - Page not found")
})

// Custom method not allowed handler
mux.MethodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusMethodNotAllowed)
    fmt.Fprintf(w, "Method not allowed")
})
```

### Complete Example
```go
package main

import (
    "fmt"
    "log"
    "net/http"
    
    "github.com/jie10/GoFlow"
)

func main() {
    mux := GoFlow.New()
    
    // Global middleware
    mux.Use(loggingMiddleware)
    
    // Basic routes
    mux.Handle("/", http.HandlerFunc(homeHandler), "GET")
    mux.Handle("/about", http.HandlerFunc(aboutHandler), "GET")
    
    // Routes with parameters
    mux.Handle("/users/:id", http.HandlerFunc(userHandler), "GET")
    
    // Admin group
    mux.Group(func(m *GoFlow.Mux) {
        m.Use(authMiddleware)
        m.Handle("/admin", http.HandlerFunc(adminHandler), "GET")
    })
    
    log.Fatal(http.ListenAndServe(":2323", mux))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome to the home page!")
}

func userHandler(w http.ResponseWriter, r *http.Request) {
    userID := GoFlow.Param(r.Context(), "id")
    fmt.Fprintf(w, "User ID: %s", userID)
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("Request: %s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}
```

### Contributing
Contributions are welcome! Please feel free to submit a Pull Request.