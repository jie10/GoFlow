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

type routeNode struct {
	get     http.Handler
	methods *methodHandler
}

// routeTree represents a node in the routing tree
type routeTree struct {
	segment        string
	methods        *methodHandler
	children       map[string]*routeTree
	paramChild     *routeTree
	paramName      string
	isWildcard     bool
	rxPattern      *regexp.Regexp
	staticHandlers map[string]routeNode
}

// Add after existing type definitions
type routeCacheEntry struct {
	methods *methodHandler
	params  map[string]string
}

type MiddlewareChain struct {
	handlers []func(http.Handler) http.Handler
	cached   http.Handler
}

// Update Mux struct
type Mux struct {
	root             *routeTree
	NotFound         http.Handler
	MethodNotAllowed http.Handler
	Options          http.Handler
	middlewares      []func(http.Handler) http.Handler
	middlewareChain  MiddlewareChain // Add this
	rxCache          sync.Map
	pathCache        sync.Map // Add this
	optimized        bool
}

// New creates a new Mux instance
func New() *Mux {
	return &Mux{
		root: &routeTree{
			children:       make(map[string]*routeTree),
			staticHandlers: make(map[string]routeNode),
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

	// Fast path for GET requests
	if r.Method == MethodGet {
		if route, ok := m.root.staticHandlers[path[1:]]; ok && route.get != nil {
			route.get.ServeHTTP(w, r)
			return
		}
	}

	// Optimize struct pooling
	sw := responseWriterPool.Get().(*statusWriter)
	sw.ResponseWriter = w
	sw.status = 0
	sw.size = 0
	clear(sw.headers)
	defer responseWriterPool.Put(sw)

	// Get segments from pool
	segments := m.getPathSegments(path)
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
			if len(foundParams) > 0 {
				ctx := context.WithValue(r.Context(), paramContextKey{}, foundParams)
				handler.ServeHTTP(sw, r.WithContext(ctx))
				return
			}
			handler.ServeHTTP(sw, r)
			return
		}
		sw.Header().Set("Allow", methods.allowedList)
		if r.Method == MethodOptions {
			m.wrap(m.Options).ServeHTTP(sw, r)
		} else {
			m.wrap(m.MethodNotAllowed).ServeHTTP(sw, r)
		}
		return
	}

	m.wrap(m.NotFound).ServeHTTP(sw, r)
}

func (m *Mux) getPathSegments(path string) []string {
	if path == "" || path == "/" {
		return nil
	}

	segments := segmentsPool.Get().([]string)
	segments = segments[:0]

	var start int
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			segments = append(segments, path[start+1:i])
			start = i
		} else if i == len(path)-1 {
			segments = append(segments, path[start+1:])
		}
	}

	return segments
}

func (m *Mux) getStaticHandler(path string, method string) http.Handler {
	if route, ok := m.root.staticHandlers[strings.TrimPrefix(path, "/")]; ok {
		if method == MethodGet {
			return route.get
		}
		if route.methods != nil {
			return route.methods.handlers[method]
		}
	}
	return nil
}

func (m *Mux) lookupStaticRoute(path string) *routeTree {
	if !m.optimized {
		return nil
	}
	if pathLen := len(path); pathLen > 1 && path[pathLen-1] == '/' {
		path = path[:pathLen-1]
	}
	if route, ok := m.root.staticHandlers[path[1:]]; ok {
		return &routeTree{methods: route.methods}
	}
	return nil
}

// Use adds middleware to the router
func (m *Mux) Use(mw ...func(http.Handler) http.Handler) {
	m.middlewares = append(m.middlewares, mw...)
	// Reset middleware chain cache
	m.middlewareChain.cached = nil
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
