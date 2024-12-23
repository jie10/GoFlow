package GoFlow

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
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

func (m *Mux) findHandler(node *routeTree, segments []string, params map[string]string) (*methodHandler, map[string]string, bool) {
	if len(segments) == 0 {
		return node.methods, params, true
	}

	if node.isWildcard {
		params["..."] = strings.Join(segments, "/")
		return node.methods, params, true
	}

	segment := segments[0]
	remaining := segments[1:]

	// Try exact match first
	if child, ok := node.children[segment]; ok {
		if methods, p, found := m.findHandler(child, remaining, params); found {
			return methods, p, true
		}
	}

	// Try parameter match
	if node.paramChild != nil {
		if node.paramChild.rxPattern != nil && !node.paramChild.rxPattern.MatchString(segment) {
			return nil, nil, false
		}

		params[node.paramChild.paramName] = segment
		if methods, p, found := m.findHandler(node.paramChild, remaining, params); found {
			return methods, p, true
		}
		delete(params, node.paramChild.paramName)
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
	m.root.staticPaths = make(map[string]*routeTree)
	m.buildStaticPaths(m.root, "")
}

func (m *Mux) buildStaticPaths(node *routeTree, prefix string) {
	if node.paramChild != nil || node.isWildcard {
		return
	}

	if node.methods != nil {
		m.root.staticPaths[prefix] = node
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

func (m *Mux) wrap(handler http.Handler) http.Handler {
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		handler = m.middlewares[i](handler)
	}
	return handler
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

func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}

	segments := segmentsPool.Get().([]string)
	segments = segments[:0]

	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if start < i {
				segments = append(segments, path[start:i])
			}
			start = i + 1
		}
	}

	if start < len(path) {
		segments = append(segments, path[start:])
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
