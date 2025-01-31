package GoFlow

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
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
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout adds a timeout to the request context
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
		})
	}
}

// Logger logs request information
func Logger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			// Get real IP from proxy headers if available
			ip := r.Header.Get("X-Real-IP")
			if ip == "" {
				ip = r.Header.Get("X-Forwarded-For")
				if ip == "" {
					ip = r.RemoteAddr
				}
			}

			log.Printf(
				"[%s] %s %s %d %s %d bytes %s",
				ip,
				r.Method,
				r.URL.Path,
				sw.status,
				duration,
				sw.size,
				r.UserAgent(),
			)
		})
	}
}

type RateLimiter struct {
	buckets   sync.Map
	maxSize   int
	evictList *list.List // For LRU eviction
	mu        sync.Mutex
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	for rl.evictList.Len() > rl.maxSize {
		if elem := rl.evictList.Back(); elem != nil {
			rl.buckets.Delete(elem.Value)
			rl.evictList.Remove(elem)
		}
	}
	rl.mu.Unlock()
}

// RateLimit implements a token bucket rate limiting middleware
func RateLimit(requests int, duration time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		tokens   int
		lastSeen time.Time
		mu       sync.Mutex
	}

	buckets := sync.Map{}

	// Clean up old buckets periodically
	go func() {
		for range time.Tick(duration) {
			buckets.Range(func(key, value interface{}) bool {
				b := value.(*bucket)
				b.mu.Lock()
				if time.Since(b.lastSeen) > duration*2 {
					buckets.Delete(key)
				}
				b.mu.Unlock()
				return true
			})
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.Header.Get("X-Real-IP")
			if ip == "" {
				ip = r.Header.Get("X-Forwarded-For")
				if ip == "" {
					ip = r.RemoteAddr
				}
			}

			var b *bucket
			if v, ok := buckets.Load(ip); ok {
				b = v.(*bucket)
			} else {
				b = &bucket{tokens: requests}
				buckets.Store(ip, b)
			}

			b.mu.Lock()
			defer b.mu.Unlock()

			// Replenish tokens based on time passed
			elapsed := time.Since(b.lastSeen)
			newTokens := int(elapsed.Seconds() * float64(requests) / duration.Seconds())
			b.tokens = min(requests, b.tokens+newTokens)
			b.lastSeen = time.Now()

			if b.tokens <= 0 {
				w.Header().Set("X-RateLimit-Limit", toString(requests))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", toString(int(duration.Seconds())))
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			b.tokens--
			w.Header().Set("X-RateLimit-Limit", toString(requests))
			w.Header().Set("X-RateLimit-Remaining", toString(b.tokens))
			w.Header().Set("X-RateLimit-Reset", toString(int(duration.Seconds())))

			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware adds Cross-Origin Resource Sharing headers
func CORS(allowedOrigins []string, allowedMethods []string, allowedHeaders []string) func(http.Handler) http.Handler {
	allowedOriginsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		allowedOriginsMap[origin] = true
	}

	allowedMethodsStr := strings.Join(allowedMethods, ", ")
	allowedHeadersStr := strings.Join(allowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" {
				if allowedOriginsMap["*"] {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if allowedOriginsMap[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", allowedMethodsStr)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeadersStr)
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Compression middleware for response compression
func Compression() func(http.Handler) http.Handler {
	pool := sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(nil)
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gz := pool.Get().(*gzip.Writer)
			defer pool.Put(gz)

			gz.Reset(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")

			gzw := &gzipResponseWriter{
				ResponseWriter: w,
				Writer:         gz,
			}
			next.ServeHTTP(gzw, r)
		})
	}
}

// Cache middleware for response caching
func Cache(duration time.Duration) func(http.Handler) http.Handler {
	cache := sync.Map{}

	// Clean up expired entries periodically
	go func() {
		for range time.Tick(duration) {
			cache.Range(func(key, value interface{}) bool {
				if entry := value.(*cacheEntry); entry.expired() {
					cache.Delete(key)
				}
				return true
			})
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache GET requests
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			key := r.URL.String()
			if cached, ok := cache.Load(key); ok {
				entry := cached.(*cacheEntry)
				if !entry.expired() {
					for k, values := range entry.headers {
						for _, v := range values {
							w.Header().Add(k, v)
						}
					}
					w.Write(entry.data)
					return
				}
				cache.Delete(key)
			}

			cw := &cacheWriter{
				ResponseWriter: w,
				headers:        make(http.Header),
			}
			next.ServeHTTP(cw, r)

			if cw.status == http.StatusOK {
				cache.Store(key, &cacheEntry{
					data:    cw.data.Bytes(),
					headers: cw.headers.Clone(),
					expires: time.Now().Add(duration),
				})
			}
		})
	}
}

var responseWriterPool = sync.Pool{
	New: func() interface{} {
		return &statusWriter{
			headers: make(http.Header),
		}
	},
}

// Helper types
type statusWriter struct {
	http.ResponseWriter
	status  int
	size    int64
	headers http.Header
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
	w.Writer.Flush()
}

type cacheEntry struct {
	data    []byte
	headers http.Header
	expires time.Time
}

func (c *cacheEntry) expired() bool {
	return time.Now().After(c.expires)
}

type cacheWriter struct {
	http.ResponseWriter
	status  int
	headers http.Header
	data    bytes.Buffer
}

func (w *cacheWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *cacheWriter) Write(b []byte) (int, error) {
	w.data.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheWriter) Header() http.Header {
	return w.headers
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func toString(n int) string {
	return strconv.Itoa(n)
}
