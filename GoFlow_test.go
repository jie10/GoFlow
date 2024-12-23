package GoFlow

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMux(t *testing.T) {
	t.Run("Basic Route Matching", func(t *testing.T) {
		mux := New()
		called := false

		mux.Handle("/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/hello", nil)

		mux.ServeHTTP(w, r)

		if !called {
			t.Error("Handler was not called")
		}
	})

	t.Run("URL Parameters", func(t *testing.T) {
		mux := New()
		var capturedID string

		mux.Handle("/users/:id", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = Param(r.Context(), "id")
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/users/123", nil)

		mux.ServeHTTP(w, r)

		if capturedID != "123" {
			t.Errorf("Expected id '123', got '%s'", capturedID)
		}
	})

	t.Run("Multiple URL Parameters", func(t *testing.T) {
		mux := New()
		var userID, action string

		mux.Handle("/users/:id/:action", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID = Param(r.Context(), "id")
			action = Param(r.Context(), "action")
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/users/123/edit", nil)

		mux.ServeHTTP(w, r)

		if userID != "123" || action != "edit" {
			t.Errorf("Expected id '123' and action 'edit', got id '%s' and action '%s'", userID, action)
		}
	})

	t.Run("Regex Parameters", func(t *testing.T) {
		mux := New()
		var capturedID string

		mux.Handle("/users/:id|^\\d+$", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = Param(r.Context(), "id")
		}), MethodGet)

		tests := []struct {
			path        string
			shouldMatch bool
			expectedID  string
		}{
			{"/users/123", true, "123"},
			{"/users/abc", false, ""},
			{"/users/123abc", false, ""},
			{"/users/", false, ""},
		}

		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				capturedID = ""
				w := httptest.NewRecorder()
				r := httptest.NewRequest(MethodGet, tt.path, nil)
				mux.ServeHTTP(w, r)

				if tt.shouldMatch {
					if capturedID != tt.expectedID {
						t.Errorf("Path %s: Expected id '%s', got '%s'", tt.path, tt.expectedID, capturedID)
					}
					if w.Code != http.StatusOK {
						t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
					}
				} else {
					if w.Code != http.StatusNotFound {
						t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
					}
				}
			})
		}
	})

	t.Run("Wildcard Routes", func(t *testing.T) {
		mux := New()
		var capturedPath string

		mux.Handle("/static/...", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = Param(r.Context(), "...")
		}), MethodGet)

		tests := []struct {
			path     string
			expected string
		}{
			{"/static/css/styles.css", "css/styles.css"},
			{"/static/js/app.js", "js/app.js"},
			{"/static/img/logo.png", "img/logo.png"},
			{"/static/", ""},
		}

		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				capturedPath = ""
				w := httptest.NewRecorder()
				r := httptest.NewRequest(MethodGet, tt.path, nil)
				mux.ServeHTTP(w, r)

				if capturedPath != tt.expected {
					t.Errorf("Expected path '%s', got '%s'", tt.expected, capturedPath)
				}
			})
		}
	})

	t.Run("Method Not Allowed", func(t *testing.T) {
		mux := New()

		mux.Handle("/resource", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodPost, "/resource", nil)

		mux.ServeHTTP(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}

		allowedMethods := strings.Split(w.Header().Get("Allow"), ", ")
		expectedMethods := []string{MethodGet, MethodHead, MethodOptions}

		if !equalMethodLists(allowedMethods, expectedMethods) {
			t.Errorf("Allow header methods don't match. Got %v, want %v", allowedMethods, expectedMethods)
		}
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		mux := New()
		var mu sync.Mutex
		results := make(map[string]bool)

		// Add multiple routes
		routes := []string{"/route1", "/route2", "/route3"}
		for _, route := range routes {
			routeCopy := route
			mux.Handle(routeCopy, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				results[routeCopy] = true
				mu.Unlock()
			}), MethodGet)
		}

		// Concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				route := routes[i%len(routes)]
				w := httptest.NewRecorder()
				r := httptest.NewRequest(MethodGet, route, nil)
				mux.ServeHTTP(w, r)
			}(i)
		}
		wg.Wait()

		// Check results
		for _, route := range routes {
			if !results[route] {
				t.Errorf("Route %s was not called", route)
			}
		}
	})

	t.Run("Middleware Chain", func(t *testing.T) {
		mux := New()
		var order []string

		middleware1 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "m1_before")
				next.ServeHTTP(w, r)
				order = append(order, "m1_after")
			})
		}

		middleware2 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "m2_before")
				next.ServeHTTP(w, r)
				order = append(order, "m2_after")
			})
		}

		mux.Use(middleware1, middleware2)
		mux.Handle("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/test", nil)
		mux.ServeHTTP(w, r)

		expected := []string{"m1_before", "m2_before", "handler", "m2_after", "m1_after"}
		if !equalSlices(order, expected) {
			t.Errorf("Middleware execution order incorrect. Got %v, want %v", order, expected)
		}
	})

	t.Run("Route Groups with Middleware", func(t *testing.T) {
		mux := New()
		var calls []string

		// Global middleware
		mux.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, "global")
				next.ServeHTTP(w, r)
			})
		})

		// Group with additional middleware
		mux.Group(func(m *Mux) {
			m.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls = append(calls, "group")
					next.ServeHTTP(w, r)
				})
			})

			m.Handle("/admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, "handler")
			}), MethodGet)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/admin", nil)
		mux.ServeHTTP(w, r)

		expected := []string{"global", "group", "handler"}
		if !equalSlices(calls, expected) {
			t.Errorf("Expected calls %v, got %v", expected, calls)
		}
	})

	t.Run("Timeout Middleware", func(t *testing.T) {
		mux := New()
		mux.Use(Timeout(50 * time.Millisecond))

		mux.Handle("/slow", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				// Context should be canceled
				return
			case <-time.After(100 * time.Millisecond):
				// Should not reach here
				t.Error("Handler exceeded timeout")
			}
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/slow", nil)
		mux.ServeHTTP(w, r)
	})

	t.Run("Recovery Middleware", func(t *testing.T) {
		mux := New()
		mux.Use(Recovery())

		mux.Handle("/panic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/panic", nil)

		// Should not panic
		mux.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

// Helper functions
func equalMethodLists(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))

	copy(aCopy, a)
	copy(bCopy, b)

	sort.Strings(aCopy)
	sort.Strings(bCopy)

	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}

	return true
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
