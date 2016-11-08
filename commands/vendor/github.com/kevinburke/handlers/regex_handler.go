package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/kevinburke/rest"
)

type route struct {
	pattern *regexp.Regexp
	methods []string
	handler http.Handler
}

// A RegexpHandler is a simple http.Handler that can match regular expressions
// for routes.
type Regexp struct {
	routes []*route
}

// Handle calls the provided handler for requests whose URL matches the given
// pattern and HTTP method. The first matching route will get called.
func (h *Regexp) Handle(pattern *regexp.Regexp, methods []string, handler http.Handler) {
	h.routes = append(h.routes, &route{
		pattern: pattern,
		methods: methods,
		handler: handler,
	})
}

// HandleFunc calls the provided HandlerFunc for requests whose URL matches the
// given pattern and HTTP method. The first matching route will get called.
func (h *Regexp) HandleFunc(pattern *regexp.Regexp, methods []string, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{
		pattern: pattern,
		methods: methods,
		handler: http.HandlerFunc(handler),
	})
}

// ServeHTTP checks all registered routes in turn for a match, and
// Scalls erveHTTP on the first matching handler. If no routes match,
// StatusMethodNotAllowed will be rendered.
func (h *Regexp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		if route.pattern.MatchString(r.URL.Path) {
			upperMethod := strings.ToUpper(r.Method)
			for _, method := range route.methods {
				if strings.ToUpper(method) == upperMethod {
					route.handler.ServeHTTP(w, r)
					return
				}
			}
			if upperMethod == "OPTIONS" {
				methods := strings.Join(append(route.methods, "OPTIONS"), ", ")
				w.Header().Set("Allow", methods)
			} else {
				rest.NotAllowed(w, r)
				return
			}
			return
		}
	}
	rest.NotFound(w, r)
}
