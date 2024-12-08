package GoFlow

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
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
		}

		for _, tt := range tests {
			capturedID = ""
			w := httptest.NewRecorder()
			r := httptest.NewRequest(MethodGet, tt.path, nil)
			mux.ServeHTTP(w, r)

			if tt.shouldMatch && capturedID != tt.expectedID {
				t.Errorf("Path %s: Expected id '%s', got '%s'", tt.path, tt.expectedID, capturedID)
			}
			if !tt.shouldMatch && capturedID != "" {
				t.Errorf("Path %s: Should not have matched", tt.path)
			}
		}
	})

	t.Run("Wildcard Routes", func(t *testing.T) {
		mux := New()
		var capturedPath string

		mux.Handle("/static/...", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = Param(r.Context(), "...")
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/static/css/styles.css", nil)

		mux.ServeHTTP(w, r)

		expectedPath := "css/styles.css"
		if capturedPath != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, capturedPath)
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
		expectedMethods := []string{"GET", "HEAD", "OPTIONS"}

		if !equalMethodLists(allowedMethods, expectedMethods) {
			t.Errorf("Allow header methods don't match. Got %v, want %v", allowedMethods, expectedMethods)
		}
	})

	t.Run("Middleware", func(t *testing.T) {
		mux := New()
		middlewareCalled := false
		handlerCalled := false

		mux.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				middlewareCalled = true
				next.ServeHTTP(w, r)
			})
		})

		mux.Handle("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		}), MethodGet)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/test", nil)

		mux.ServeHTTP(w, r)

		if !middlewareCalled {
			t.Error("Middleware was not called")
		}
		if !handlerCalled {
			t.Error("Handler was not called")
		}
	})

	t.Run("Groups", func(t *testing.T) {
		mux := New()
		var groupMiddlewareCalled bool
		var handlerCalled bool

		mux.Group(func(m *Mux) {
			m.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					groupMiddlewareCalled = true
					next.ServeHTTP(w, r)
				})
			})

			m.Handle("/grouped", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
			}), MethodGet)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/grouped", nil)

		mux.ServeHTTP(w, r)

		if !groupMiddlewareCalled {
			t.Error("Group middleware was not called")
		}
		if !handlerCalled {
			t.Error("Group handler was not called")
		}
	})

	t.Run("Not Found", func(t *testing.T) {
		mux := New()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodGet, "/nonexistent", nil)

		mux.ServeHTTP(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("Options Request", func(t *testing.T) {
		mux := New()

		mux.Handle("/resource", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), MethodGet, MethodPost)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(MethodOptions, "/resource", nil)

		mux.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
		}

		allowedMethods := strings.Split(w.Header().Get("Allow"), ", ")
		expectedMethods := []string{"GET", "HEAD", "POST", "OPTIONS"}

		if !equalMethodLists(allowedMethods, expectedMethods) {
			t.Errorf("Allow header methods don't match. Got %v, want %v", allowedMethods, expectedMethods)
		}
	})
}

// equalMethodLists compares two slices of HTTP methods, ignoring order
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
