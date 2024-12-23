package GoFlow

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// Common HTTP methods
const (
	MethodGet     = "GET"
	MethodPost    = "POST"
	MethodPut     = "PUT"
	MethodDelete  = "DELETE"
	MethodPatch   = "PATCH"
	MethodHead    = "HEAD"
	MethodOptions = "OPTIONS"
	MethodTrace   = "TRACE"
	MethodConnect = "CONNECT"
)

// Method bitset constants
const (
	methodGet uint16 = 1 << iota
	methodPost
	methodPut
	methodDelete
	methodPatch
	methodHead
	methodOptions
	methodTrace
	methodConnect
)

// AllMethods contains all supported HTTP methods
var AllMethods = []string{
	MethodGet, MethodHead, MethodPost, MethodPut,
	MethodPatch, MethodDelete, MethodConnect,
	MethodOptions, MethodTrace,
}

var methodMap = map[string]uint16{
	MethodGet:     methodGet,
	MethodPost:    methodPost,
	MethodPut:     methodPut,
	MethodDelete:  methodDelete,
	MethodPatch:   methodPatch,
	MethodHead:    methodHead,
	MethodOptions: methodOptions,
	MethodTrace:   methodTrace,
	MethodConnect: methodConnect,
}

// Pools for various objects
var (
	paramsPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]string, 8)
		},
	}

	builderPool = sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}

	segmentsPool = sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 8)
		},
	}
)

type (
	contextKey      struct{}
	paramContextKey struct{}
)

// methodHandler manages HTTP method handling
type methodHandler struct {
	handlers    map[string]http.Handler
	allowedSet  uint16
	allowedList string
}

// routeTree represents a node in the routing tree
type routeTree struct {
	segment     string
	methods     *methodHandler
	children    map[string]*routeTree
	paramChild  *routeTree
	paramName   string
	isWildcard  bool
	rxPattern   *regexp.Regexp
	staticPaths map[string]*routeTree
}

// Mux is the main router struct
type Mux struct {
	root             *routeTree
	NotFound         http.Handler
	MethodNotAllowed http.Handler
	Options          http.Handler
	middlewares      []func(http.Handler) http.Handler
	rxCache          sync.Map
	optimized        bool
}

// New creates a new Mux instance
func New() *Mux {
	return &Mux{
		root: &routeTree{
			children:    make(map[string]*routeTree),
			staticPaths: make(map[string]*routeTree),
		},
		NotFound: http.NotFoundHandler(),
		MethodNotAllowed: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}),
		Options: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}
}

// Handle registers a new route with its handlers
func (m *Mux) Handle(pattern string, handler http.Handler, methods ...string) {
	if len(methods) == 0 {
		methods = AllMethods
	}

	if contains(methods, MethodGet) && !contains(methods, MethodHead) {
		methods = append(methods, MethodHead)
	}

	wrappedHandler := m.wrap(handler)
	for _, method := range methods {
		m.addRoute(pattern, strings.ToUpper(method), wrappedHandler)
	}

	// Pre-compute static paths after adding new routes
	if m.optimized {
		m.precomputeStaticPaths()
	}
}

// ServeHTTP implements the http.Handler interface
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	// Get segments from pool
	segments := splitPath(path)
	if segments == nil {
		segments = segmentsPool.Get().([]string)[:0]
	}
	defer segmentsPool.Put(segments)

	// Get params from pool
	params := paramsPool.Get().(map[string]string)
	defer func() {
		clear(params)
		paramsPool.Put(params)
	}()

	methods, foundParams, found := m.findHandler(m.root, segments, params)

	if found && methods != nil {
		if handler, ok := methods.handlers[r.Method]; ok {
			ctx := r.Context()
			if len(foundParams) > 0 {
				ctx = context.WithValue(ctx, paramContextKey{}, foundParams)
			}
			handler.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Method not allowed
		w.Header().Set("Allow", methods.allowedList)
		if r.Method == MethodOptions {
			m.wrap(m.Options).ServeHTTP(w, r)
		} else {
			m.wrap(m.MethodNotAllowed).ServeHTTP(w, r)
		}
		return
	}

	m.wrap(m.NotFound).ServeHTTP(w, r)
}

// Use adds middleware to the router
func (m *Mux) Use(mw ...func(http.Handler) http.Handler) {
	m.middlewares = append(m.middlewares, mw...)
}

// Group creates a new route group
func (m *Mux) Group(fn func(*Mux)) {
	subMux := &Mux{
		root:        m.root,
		middlewares: make([]func(http.Handler) http.Handler, len(m.middlewares)),
	}
	copy(subMux.middlewares, m.middlewares)
	fn(subMux)
}

// Optimize applies performance optimizations
func (m *Mux) Optimize() {
	if !m.optimized {
		m.precomputeStaticPaths()
		m.optimized = true
	}
}

// Param gets a route parameter from the context
func Param(ctx context.Context, param string) string {
	if params, ok := ctx.Value(paramContextKey{}).(map[string]string); ok {
		return params[param]
	}
	return ""
}
