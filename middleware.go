package GoFlow

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// Recovery middleware to handle panics
func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("panic: %v\n%s", err, debug.Stack())
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout middleware to add context timeout
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Logger middleware for request logging
func Logger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			log.Printf(
				"%s %s %d %s",
				r.Method,
				r.RequestURI,
				sw.status,
				time.Since(start),
			)
		})
	}
}

// RateLimit middleware for basic rate limiting
func RateLimit(requests int, duration time.Duration) func(http.Handler) http.Handler {
	type client struct {
		count    int
		lastSeen time.Time
	}

	clients := make(map[string]*client)
	var mu sync.Mutex

	// Clean up old entries periodically
	go func() {
		for range time.Tick(duration) {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > duration {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			if c, exists := clients[ip]; exists {
				if c.count >= requests && time.Since(c.lastSeen) <= duration {
					mu.Unlock()
					http.Error(w, "Too many requests", http.StatusTooManyRequests)
					return
				}
				c.count++
				c.lastSeen = time.Now()
			} else {
				clients[ip] = &client{count: 1, lastSeen: time.Now()}
			}
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
