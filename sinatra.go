// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"github.com/go-ruby-rack/rack"
)

// route is one entry in the per-verb route table: a compiled pattern and the
// action body to run on a match.
type route struct {
	pattern *Pattern
	action  Action
}

// filter is a before/after filter: an optional pattern (nil = every request)
// and the action to run.
type filter struct {
	pattern *Pattern // nil means "all paths"
	action  Action
}

// Sinatra is an application: the route table per HTTP verb, the before/after
// filters, the not_found and error handlers, and the settings registry. Build
// one with [New], register routes with Get/Post/…, then serve a Rack env with
// [Sinatra.Call].
type Sinatra struct {
	routes      map[string][]route
	beforeFltrs []filter
	afterFltrs  []filter
	notFound    Action
	// errorHandlers maps an HTTP status to its handler (Sinatra's error(code)).
	errorHandlers map[int]Action
	settings      *Settings
}

// New returns an empty application with Sinatra's defaults.
func New() *Sinatra {
	return &Sinatra{
		routes:        map[string][]route{},
		errorHandlers: map[int]Action{},
		settings:      newSettings(),
	}
}

// Settings returns the application settings registry (set/enable/disable).
func (s *Sinatra) Settings() *Settings { return s.settings }

// route registration ---------------------------------------------------------

// addRoute compiles pat (a Mustermann string) for verb and appends it.
func (s *Sinatra) addRoute(verb, pat string, action Action) {
	s.routes[verb] = append(s.routes[verb], route{pattern: Compile(pat), action: action})
}

// Route registers an action for an arbitrary HTTP verb and Mustermann pattern.
func (s *Sinatra) Route(verb, pat string, action Action) { s.addRoute(verb, pat, action) }

// RoutePattern registers an action for verb against an already-compiled
// [Pattern] (e.g. a regexp route from [CompileRegexp]).
func (s *Sinatra) RoutePattern(verb string, p *Pattern, action Action) {
	s.routes[verb] = append(s.routes[verb], route{pattern: p, action: action})
}

// Get registers a GET (and, by Sinatra convention, HEAD) route.
func (s *Sinatra) Get(pat string, action Action) {
	s.addRoute(rack.MethodGet, pat, action)
	s.addRoute(rack.MethodHead, pat, action)
}

// Post registers a POST route.
func (s *Sinatra) Post(pat string, action Action) { s.addRoute(rack.MethodPost, pat, action) }

// Put registers a PUT route.
func (s *Sinatra) Put(pat string, action Action) { s.addRoute(rack.MethodPut, pat, action) }

// Delete registers a DELETE route.
func (s *Sinatra) Delete(pat string, action Action) { s.addRoute(rack.MethodDelete, pat, action) }

// Patch registers a PATCH route.
func (s *Sinatra) Patch(pat string, action Action) { s.addRoute(rack.MethodPatch, pat, action) }

// Options registers an OPTIONS route.
func (s *Sinatra) Options(pat string, action Action) { s.addRoute(rack.MethodOptions, pat, action) }

// Head registers a HEAD route explicitly.
func (s *Sinatra) Head(pat string, action Action) { s.addRoute(rack.MethodHead, pat, action) }

// filters and handlers -------------------------------------------------------

// Before registers a before-filter. With an empty pattern it runs on every
// request; otherwise only when pat matches the path (and its captures merge
// into params, like Sinatra).
func (s *Sinatra) Before(pat string, action Action) { s.addFilter(&s.beforeFltrs, pat, action) }

// After registers an after-filter, run after the action (and after a halt).
func (s *Sinatra) After(pat string, action Action) { s.addFilter(&s.afterFltrs, pat, action) }

func (s *Sinatra) addFilter(dst *[]filter, pat string, action Action) {
	var p *Pattern
	if pat != "" {
		p = Compile(pat)
	}
	*dst = append(*dst, filter{pattern: p, action: action})
}

// NotFound registers the handler for an unmatched request (Sinatra's
// not_found), run with status 404.
func (s *Sinatra) NotFound(action Action) { s.notFound = action }

// Error registers a handler for a specific HTTP status (Sinatra's error(code)).
func (s *Sinatra) Error(status int, action Action) { s.errorHandlers[status] = action }

// dispatch -------------------------------------------------------------------

// Call serves a Rack environment, returning the [status, headers, body] tuple
// as a rack.Response. It runs the before filters, finds the first matching
// route for the request method (honouring pass), runs the action, runs the
// after filters, and applies the not_found / error handlers.
func (s *Sinatra) Call(env rack.Env) *rack.Response {
	req := rack.NewRequest(env)
	resp := rack.NewResponse(nil, 200, rack.NewHeaders())
	// Default content type, like Sinatra.
	resp.SetContentType(s.defaultContentType())

	c := &Context{
		app:      s,
		request:  req,
		response: resp,
		params:   newOrderedParams(),
	}
	if sess, ok := env[rack.RackSession]; ok {
		c.session = sess
	}
	// Sinatra seeds params from the request (query + form) before filters run;
	// route captures then override on collision.
	s.seedQueryParams(c)

	path := req.PathInfo()
	if path == "" {
		path = "/"
	}
	method := req.RequestMethod()

	s.invoke(c, func() {
		s.runBefore(c, path)
		s.route(c, method, path)
	})
	s.runAfter(c, path)
	s.finalizeBody(c)
	return c.response
}

// invoke runs fn, recovering a halt (stop now) but letting a pass propagate to
// the router. Any non-control panic is re-raised.
func (s *Sinatra) invoke(_ *Context, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(haltSignal); ok {
				return
			}
			panic(r)
		}
	}()
	fn()
}

// route finds and runs the first matching route for method/path, applying
// params and honouring pass. With no match (or all routes passed) it runs the
// not_found path.
func (s *Sinatra) route(c *Context, method, path string) {
	rs := s.routes[method]
	for i := 0; i < len(rs); i++ {
		mr, ok := rs[i].pattern.Match(path)
		if !ok {
			continue
		}
		passed := s.runRoute(c, rs[i].action, mr, path)
		if passed {
			continue // pass: try the next matching route
		}
		return // matched and ran (or halted)
	}
	s.handleNotFound(c)
}

// runRoute installs the route's params, runs the action, and reports whether
// the action called pass. A halt inside the action stops via the invoke
// recover at Call; here we recover only pass.
func (s *Sinatra) runRoute(c *Context, action Action, mr MatchResult, path string) (passed bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(passSignal); ok {
				passed = true
				return
			}
			panic(r) // halt or a real error propagates
		}
	}()
	s.applyMatch(c, mr)
	out := action(c)
	s.applyActionResult(c, out)
	return false
}

// runBefore runs every before filter whose pattern matches (a nil pattern
// always runs), merging its captures into params first.
func (s *Sinatra) runBefore(c *Context, path string) {
	for _, f := range s.beforeFltrs {
		if f.pattern == nil {
			f.action(c)
			continue
		}
		if mr, ok := f.pattern.Match(path); ok {
			s.applyMatch(c, mr)
			f.action(c)
		}
	}
}

// runAfter runs the after filters. They run even after a halt (Sinatra runs
// after filters in an ensure), so this is called unconditionally and is itself
// halt-safe.
func (s *Sinatra) runAfter(c *Context, path string) {
	s.invoke(c, func() {
		for _, f := range s.afterFltrs {
			if f.pattern == nil {
				f.action(c)
				continue
			}
			if mr, ok := f.pattern.Match(path); ok {
				s.applyMatch(c, mr)
				f.action(c)
			}
		}
	})
}

// handleNotFound sets status 404 and runs the not_found handler (or leaves an
// empty body for the host to render the default page).
func (s *Sinatra) handleNotFound(c *Context) {
	c.response.SetStatus(404)
	if h, ok := s.errorHandlers[404]; ok {
		out := h(c)
		s.applyActionResult(c, out)
		return
	}
	if s.notFound != nil {
		out := s.notFound(c)
		s.applyActionResult(c, out)
	}
}

// applyMatch merges a route/filter match into the context params, in Sinatra's
// order: existing query params are already in place from Params; route captures
// overwrite them (route wins), then the splat/captures array is appended.
func (s *Sinatra) applyMatch(c *Context, mr MatchResult) {
	if mr.Named != nil {
		c.params.mergeFrom(mr.Named)
	}
	if len(mr.Splat) > 0 {
		key := "splat"
		if mr.FromRegexp {
			key = "captures"
		}
		if existing, ok := c.params.get(key); ok {
			if arr, ok := existing.([]string); ok {
				c.params.set(key, append(append([]string{}, arr...), mr.Splat...))
			} else {
				c.params.set(key, mr.Splat)
			}
		} else {
			c.params.set(key, mr.Splat)
		}
	}
}

// seedQueryParams loads the request's query (and form) params into the context
// params. Route params then override on collision (Sinatra merges route over
// request params).
func (s *Sinatra) seedQueryParams(c *Context) {
	p, err := c.request.Params()
	if err != nil || p == nil {
		return
	}
	p.Each(func(k string, v any) bool {
		c.params.set(k, v)
		return true
	})
}

// applyActionResult turns the value an action returned into the response body,
// mirroring Sinatra's body coercion: a string is the body; a []string is the
// body parts; an int is a status code (with no body change); nil leaves the
// body as set by helpers.
func (s *Sinatra) applyActionResult(c *Context, out any) {
	switch v := out.(type) {
	case nil:
		// leave body as-is (set via body/halt or empty)
	case string:
		c.setBody(v)
	case []string:
		c.setBodyParts(v)
	case int:
		c.response.SetStatus(v)
	case []byte:
		c.setBody(string(v))
	}
}

// finalizeBody applies the error handler for the final status if one is
// registered and the body is still empty, matching Sinatra dispatching
// error(code) for a bare status with no body.
func (s *Sinatra) finalizeBody(c *Context) {
	st := c.response.Status()
	if st == 404 {
		return // handled in handleNotFound
	}
	if c.response.Empty() {
		if h, ok := s.errorHandlers[st]; ok {
			s.invoke(c, func() {
				out := h(c)
				s.applyActionResult(c, out)
			})
		}
	}
}

func (s *Sinatra) defaultContentType() string {
	mt := s.settings.String("default_content_type")
	enc := s.settings.String("default_encoding")
	ct, ok := buildContentType(mt, enc, "")
	if !ok {
		return mt
	}
	return ct
}

// StatusText returns the canonical reason phrase for an HTTP status code, from
// rack's status table (e.g. 404 -> "Not Found"). It is "" for an unknown code.
func StatusText(code int) string {
	return rack.HTTPStatusCodes[code]
}

// CallTuple is a convenience returning the SPEC [status, headers, body] tuple
// directly, the form a Rack server consumes.
func (s *Sinatra) CallTuple(env rack.Env) (int, *rack.Headers, []string) {
	return s.Call(env).Finish()
}
