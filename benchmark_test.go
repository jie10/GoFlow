package GoFlow

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRouteMatch(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Static route
	b.Run("Static", func(b *testing.B) {
		mux.Handle("/users/profile", handler, "GET")
		r := httptest.NewRequest("GET", "/users/profile", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})

	// Parameter route
	b.Run("Parameter", func(b *testing.B) {
		mux.Handle("/users/:id", handler, "GET")
		r := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})

	// Regex parameter route
	b.Run("RegexParameter", func(b *testing.B) {
		mux.Handle("/users/:id|^\\d+$", handler, "GET")
		r := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})

	// Wildcard route
	b.Run("Wildcard", func(b *testing.B) {
		mux.Handle("/static/...", handler, "GET")
		r := httptest.NewRequest("GET", "/static/css/style.css", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})
}

func BenchmarkMiddleware(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Single middleware
	b.Run("SingleMiddleware", func(b *testing.B) {
		mux.Use(Logger())
		mux.Handle("/test", handler, "GET")
		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})

	// Multiple middleware
	b.Run("MultipleMiddleware", func(b *testing.B) {
		mux := New()
		mux.Use(Logger(), Recovery(), Timeout(30))
		mux.Handle("/test", handler, "GET")
		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})
}

func BenchmarkRouteGroups(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Simple group
	b.Run("SimpleGroup", func(b *testing.B) {
		mux.Group(func(m *Mux) {
			m.Handle("/admin/users", handler, "GET")
		})
		r := httptest.NewRequest("GET", "/admin/users", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})

	// Nested groups
	b.Run("NestedGroups", func(b *testing.B) {
		mux.Group(func(m *Mux) {
			m.Use(Logger())
			m.Group(func(m *Mux) {
				m.Use(Recovery())
				m.Handle("/admin/settings/users", handler, "GET")
			})
		})
		r := httptest.NewRequest("GET", "/admin/settings/users", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mux.ServeHTTP(w, r)
		}
	})
}

func BenchmarkParallelRequests(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Setup routes
	mux.Handle("/static", handler, "GET")
	mux.Handle("/users/:id", handler, "GET")
	mux.Handle("/api/v1/...", handler, "GET")

	b.Run("ParallelStaticRoute", func(b *testing.B) {
		r := httptest.NewRequest("GET", "/static", nil)
		w := httptest.NewRecorder()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mux.ServeHTTP(w, r)
			}
		})
	})

	b.Run("ParallelParameterRoute", func(b *testing.B) {
		r := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mux.ServeHTTP(w, r)
			}
		})
	})

	b.Run("ParallelWildcardRoute", func(b *testing.B) {
		r := httptest.NewRequest("GET", "/api/v1/users/123/profile", nil)
		w := httptest.NewRecorder()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mux.ServeHTTP(w, r)
			}
		})
	})
}

func BenchmarkMethodNotAllowed(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	mux.Handle("/resource", handler, "GET", "POST")
	r := httptest.NewRequest("PUT", "/resource", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, r)
	}
}

func BenchmarkNotFound(b *testing.B) {
	mux := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	mux.Handle("/exists", handler, "GET")
	r := httptest.NewRequest("GET", "/does-not-exist", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, r)
	}
}
