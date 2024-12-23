package GoFlow

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// Common HTTP methods to avoid repeated string literals
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

// AllMethods is a pre-initialized slice containing all HTTP methods
var AllMethods = []string{
	MethodGet, MethodHead, MethodPost, MethodPut,
	MethodPatch, MethodDelete, MethodConnect,
	MethodOptions, MethodTrace,
}

// routeTree represents a radix tree node for faster route matching
type routeTree struct {
	segment    string
	handlers   map[string]http.Handler
	children   map[string]*routeTree // Changed from slice to map
	paramChild *routeTree            // Separate parameter child
	paramName  string
	isWildcard bool
	rxPattern  *regexp.Regexp
}

// Mux is the main router struct
type Mux struct {
	root             *routeTree
	NotFound         http.Handler
	MethodNotAllowed http.Handler
	Options          http.Handler
	middlewares      []func(http.Handler) http.Handler
	rxCache          sync.Map
}

type contextKey string

// Pool for route parameters to reduce allocations
var paramsPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]string, 8)
	},
}

// Pre-allocate builder for allowed methods
var methodsBuilder strings.Builder

func newRouteTree() *routeTree {
	return &routeTree{
		handlers: make(map[string]http.Handler),
		children: make(map[string]*routeTree),
	}
}

func New() *Mux {
	return &Mux{
		root:     newRouteTree(),
		NotFound: http.NotFoundHandler(),
		MethodNotAllowed: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}),
		Options: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}
}

func acquireParams() map[string]string {
	return paramsPool.Get().(map[string]string)
}

func releaseParams(params map[string]string) {
	clear(params)
	paramsPool.Put(params)
}

func (m *Mux) Handle(pattern string, handler http.Handler, methods ...string) {
	if len(methods) == 0 {
		methods = AllMethods
	}

	if contains(methods, MethodGet) && !contains(methods, MethodHead) {
		methods = append(methods, MethodHead)
	}

	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	wrappedHandler := m.wrap(handler)

	for _, method := range methods {
		m.addRoute(segments, strings.ToUpper(method), wrappedHandler)
	}
}

func (m *Mux) addRoute(segments []string, method string, handler http.Handler) {
	current := m.root

	for i, segment := range segments {
		if segment == "..." {
			current.isWildcard = true
			current.handlers[method] = handler
			return
		}

		var child *routeTree
		if strings.HasPrefix(segment, ":") {
			paramName, rxPattern, hasRx := strings.Cut(strings.TrimPrefix(segment, ":"), "|")
			child = m.findOrCreateChild(current, "", paramName)

			if hasRx {
				rx, _ := m.rxCache.LoadOrStore(rxPattern, regexp.MustCompile(rxPattern))
				child.rxPattern = rx.(*regexp.Regexp)
			}
		} else {
			child = m.findOrCreateChild(current, segment, "")
		}

		if i == len(segments)-1 {
			child.handlers[method] = handler
		}
		current = child
	}
}

func (m *Mux) findOrCreateChild(node *routeTree, segment, paramName string) *routeTree {
	if paramName != "" {
		if node.paramChild == nil {
			node.paramChild = newRouteTree()
			node.paramChild.paramName = paramName
		}
		return node.paramChild
	}

	child, exists := node.children[segment]
	if !exists {
		child = newRouteTree()
		child.segment = segment
		node.children[segment] = child
	}
	return child
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.EscapedPath(), "/")
	segments := strings.Split(path, "/")

	params := acquireParams()
	defer releaseParams(params)

	handler, params, allowed := m.findHandler(m.root, segments, params)

	if handler != nil && allowed[r.Method] {
		ctx := r.Context()
		for k, v := range params {
			ctx = context.WithValue(ctx, contextKey(k), v)
		}
		handler[r.Method].ServeHTTP(w, r.WithContext(ctx))
		return
	}

	if len(allowed) > 0 {
		methods := makeAllowedMethodsHeader(allowed)
		w.Header().Set("Allow", methods)
		if r.Method == MethodOptions {
			m.wrap(m.Options).ServeHTTP(w, r)
		} else {
			m.wrap(m.MethodNotAllowed).ServeHTTP(w, r)
		}
		return
	}

	m.wrap(m.NotFound).ServeHTTP(w, r)
}

func (m *Mux) findHandler(node *routeTree, segments []string, params map[string]string) (map[string]http.Handler, map[string]string, map[string]bool) {
	if len(segments) == 0 {
		if len(node.handlers) > 0 {
			return node.handlers, params, makeAllowedMethodsMap(node.handlers)
		}
		return nil, nil, nil
	}

	if node.isWildcard {
		params["..."] = strings.Join(segments, "/")
		return node.handlers, params, makeAllowedMethodsMap(node.handlers)
	}

	segment := segments[0]
	remainingSegments := segments[1:]

	// Try exact match first - O(1) lookup
	if child, exists := node.children[segment]; exists {
		if h, p, a := m.findHandler(child, remainingSegments, params); h != nil {
			return h, p, a
		}
	}

	// Try parameter match
	if node.paramChild != nil {
		if node.paramChild.rxPattern != nil && !node.paramChild.rxPattern.MatchString(segment) {
			return nil, nil, nil
		}

		newParams := copyParams(params)
		newParams[node.paramChild.paramName] = segment

		if h, p, a := m.findHandler(node.paramChild, remainingSegments, newParams); h != nil {
			return h, p, a
		}
	}

	return nil, nil, nil
}

func (m *Mux) Use(mw ...func(http.Handler) http.Handler) {
	m.middlewares = append(m.middlewares, mw...)
}

func (m *Mux) Group(fn func(*Mux)) {
	subMux := &Mux{
		root:        m.root,
		middlewares: make([]func(http.Handler) http.Handler, len(m.middlewares)),
	}
	copy(subMux.middlewares, m.middlewares)
	fn(subMux)
}

func (m *Mux) wrap(handler http.Handler) http.Handler {
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		handler = m.middlewares[i](handler)
	}
	return handler
}

func makeAllowedMethodsHeader(allowed map[string]bool) string {
	methodsBuilder.Reset()
	first := true

	for method := range allowed {
		if !first {
			methodsBuilder.WriteString(", ")
		}
		methodsBuilder.WriteString(method)
		first = false
	}

	if len(allowed) > 0 {
		if !first {
			methodsBuilder.WriteString(", ")
		}
		methodsBuilder.WriteString(MethodOptions)
	}

	return methodsBuilder.String()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func copyParams(params map[string]string) map[string]string {
	newParams := make(map[string]string, len(params))
	for k, v := range params {
		newParams[k] = v
	}
	return newParams
}

func makeAllowedMethodsMap(handlers map[string]http.Handler) map[string]bool {
	allowed := make(map[string]bool)
	for method := range handlers {
		allowed[method] = true
	}
	return allowed
}

func Param(ctx context.Context, param string) string {
	if v, ok := ctx.Value(contextKey(param)).(string); ok {
		return v
	}
	return ""
}
