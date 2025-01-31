package GoFlow

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

func (m *Mux) addRoute(pattern string, method string, handler http.Handler) {
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	current := m.root

	for i, segment := range segments {
		if segment == "..." {
			current.isWildcard = true
			if current.methods == nil {
				current.methods = newMethodHandler()
			}
			current.methods.addHandler(method, handler)
			return
		}

		var child *routeTree
		if strings.HasPrefix(segment, ":") {
			paramName, rxPattern, hasRx := strings.Cut(strings.TrimPrefix(segment, ":"), "|")
			child = m.findOrCreateChild(current, "", paramName)

			if hasRx {
				child.rxPattern = m.compilePattern(rxPattern)
			}
		} else {
			child = m.findOrCreateChild(current, segment, "")
		}

		if i == len(segments)-1 {
			if child.methods == nil {
				child.methods = newMethodHandler()
			}
			child.methods.addHandler(method, handler)
		}
		current = child
	}
}

// Replace existing findHandler with this
func (m *Mux) findHandler(node *routeTree, segments []string, params map[string]string) (*methodHandler, map[string]string, bool) {
	if len(segments) == 0 {
		return node.methods, params, true
	}

	segment := segments[0]
	remaining := segments[1:]

	// Static route lookup (most common case)
	if child := node.children[segment]; child != nil {
		return m.findHandler(child, remaining, params)
	}

	// Parameter matching with fast path for non-regex
	if pc := node.paramChild; pc != nil {
		if pc.rxPattern == nil {
			params[pc.paramName] = segment
			return m.findHandler(pc, remaining, params)
		}
		if pc.rxPattern.MatchString(segment) {
			params[pc.paramName] = segment
			return m.findHandler(pc, remaining, params)
		}
	}

	return nil, nil, false
}

// Add this new method
func (m *Mux) findHandlerInternal(node *routeTree, segments []string, params map[string]string) (*methodHandler, map[string]string, bool) {
	if len(segments) == 0 {
		return node.methods, params, true
	}

	segment := segments[0]
	remaining := segments[1:]

	// Static route matching with string compare
	if child := node.children[segment]; child != nil {
		if methods, p, found := m.findHandlerInternal(child, remaining, params); found {
			return methods, p, true
		}
	}

	// Parameter matching with early return
	if pc := node.paramChild; pc != nil && (pc.rxPattern == nil || len(segment) < 20 && pc.rxPattern.MatchString(segment)) {
		params[pc.paramName] = segment
		if methods, p, found := m.findHandlerInternal(pc, remaining, params); found {
			return methods, p, true
		}
		delete(params, pc.paramName)
	}

	return nil, nil, false
}

func (m *Mux) findOrCreateChild(node *routeTree, segment, paramName string) *routeTree {
	if paramName != "" {
		if node.paramChild == nil {
			node.paramChild = &routeTree{
				paramName: paramName,
				children:  make(map[string]*routeTree),
			}
		}
		return node.paramChild
	}

	child, exists := node.children[segment]
	if !exists {
		child = &routeTree{
			segment:  segment,
			children: make(map[string]*routeTree),
		}
		node.children[segment] = child
	}
	return child
}

func (m *Mux) precomputeStaticPaths() {
	m.root.staticHandlers = make(map[string]routeNode)
	m.buildStaticPaths(m.root, "")
}

func (m *Mux) buildStaticPaths(node *routeTree, prefix string) {
	if node.paramChild != nil || node.isWildcard {
		return
	}

	if node.methods != nil {
		m.root.staticHandlers[prefix] = routeNode{
			methods: node.methods,
			get:     node.methods.handlers[MethodGet],
		}
	}

	for segment, child := range node.children {
		newPrefix := prefix
		if newPrefix != "" {
			newPrefix += "/"
		}
		newPrefix += segment
		m.buildStaticPaths(child, newPrefix)
	}
}

// Replace existing wrap method
func (m *Mux) wrap(handler http.Handler) http.Handler {
	if m.middlewareChain.cached != nil {
		return m.middlewareChain.cached
	}

	h := handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		h = m.middlewares[i](h)
	}
	m.middlewareChain.cached = h
	return h
}

func newMethodHandler() *methodHandler {
	return &methodHandler{
		handlers: make(map[string]http.Handler),
	}
}

func (mh *methodHandler) addHandler(method string, handler http.Handler) {
	mh.handlers[method] = handler
	if bit, ok := methodMap[method]; ok {
		mh.allowedSet |= bit
	}
	mh.updateAllowedList()
}

func (mh *methodHandler) updateAllowedList() {
	var methods []string
	for method := range mh.handlers {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	mh.allowedList = strings.Join(append(methods, MethodOptions), ", ")
}

var pathSegmentPool = sync.Pool{
	New: func() interface{} {
		return make([]string, 0, 16)
	},
}

func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}

	segments := pathSegmentPool.Get().([]string)
	segments = segments[:0]

	var start int
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			segments = append(segments, path[start+1:i])
			start = i
		}
	}
	if start < len(path)-1 {
		segments = append(segments, path[start+1:])
	}

	return segments
}

func (m *Mux) compilePattern(pattern string) *regexp.Regexp {
	if rx, ok := m.rxCache.Load(pattern); ok {
		return rx.(*regexp.Regexp)
	}

	rx := regexp.MustCompile(pattern)
	m.rxCache.Store(pattern, rx)
	return rx
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
