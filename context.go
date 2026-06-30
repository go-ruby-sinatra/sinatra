// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"github.com/go-ruby-rack/rack"
)

// Action is the body of a route (or a filter): the Ruby `do…end` block as a Go
// func. It receives the live *[Context] and returns the body. The embedding
// runtime (rbgo) binds a Ruby block to an Action; a Go caller supplies one
// directly. Control-flow helpers — Halt, Pass, Redirect — are invoked on the
// context and unwind via panic, exactly like Sinatra's throw :halt/:pass.
type Action func(c *Context) any

// Context carries the per-request dispatch state and exposes the Sinatra DSL
// helpers an action body uses: Params, Request, Response, Status, Body,
// ContentType, Headers, Redirect, Halt and Pass. It wraps go-ruby-rack's
// Request and Response.
type Context struct {
	app      *Sinatra
	request  *rack.Request
	response *rack.Response
	params   *orderedParams
	// session is the seam for the session store; rbgo supplies it. It is the
	// raw rack.session env value.
	session any
}

// Request returns the underlying rack.Request.
func (c *Context) Request() *rack.Request { return c.request }

// Response returns the underlying rack.Response being assembled.
func (c *Context) Response() *rack.Response { return c.response }

// App returns the owning application (for settings access).
func (c *Context) App() *Sinatra { return c.app }

// Settings returns the application settings, like the `settings` helper.
func (c *Context) Settings() *Settings { return c.app.settings }

// Session returns the session store seam (the rack.session env value). The
// store itself is supplied by the host; Sinatra only shapes access to it.
func (c *Context) Session() any { return c.session }

// Param returns the named param value and whether it is present. The value is a
// string for a scalar, or a []string for "splat"/"captures".
func (c *Context) Param(key string) (any, bool) { return c.params.get(key) }

// ParamString returns the string value of a scalar param, or "" if absent or
// not a string.
func (c *Context) ParamString(key string) string {
	v, ok := c.params.get(key)
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// ParamKeys returns the param keys in insertion order.
func (c *Context) ParamKeys() []string { return c.params.keysInOrder() }

// Params returns a snapshot of the params as an ordered key/value list,
// preserving Sinatra's deterministic ordering (route params, then splat, then
// query). The values are strings or []string.
func (c *Context) Params() []ParamPair {
	out := make([]ParamPair, 0, len(c.params.keys))
	for _, k := range c.params.keys {
		out = append(out, ParamPair{Key: k, Value: c.params.values[k]})
	}
	return out
}

// ParamPair is one key/value entry of the params hash.
type ParamPair struct {
	Key   string
	Value any
}

// Status sets the response status, like the `status` helper.
func (c *Context) Status(code int) { c.response.SetStatus(code) }

// CurrentStatus returns the response status.
func (c *Context) CurrentStatus() int { return c.response.Status() }

// Body sets the response body to a single string, like the `body` helper.
func (c *Context) Body(s string) { c.setBody(s) }

// Header sets a response header, like response.headers[k]=v.
func (c *Context) Header(key string, value string) { c.response.SetHeader(key, value) }

// Headers sets several response headers at once, like the `headers` helper.
func (c *Context) Headers(h map[string]string) {
	for k, v := range h {
		c.response.SetHeader(k, v)
	}
}

// ContentType sets the Content-Type, resolving a Sinatra mime symbol/extension
// and appending a charset per Sinatra's add_charset rule, like the
// `content_type` helper. It panics (HaltError 500-style) for an unknown type,
// matching Sinatra raising on an unknown media type — callers using rbgo see
// the raise; Go callers should pass known types or a full media type.
func (c *Context) ContentType(typeArg string) {
	c.ContentTypeCharset(typeArg, "")
}

// ContentTypeCharset is [Context.ContentType] with an explicit charset
// (content_type :json, charset: 'utf-8').
func (c *Context) ContentTypeCharset(typeArg, charset string) {
	enc := c.app.settings.String("default_encoding")
	ct, ok := buildContentType(typeArg, enc, charset)
	if !ok {
		panic(&UnknownMediaType{Type: typeArg})
	}
	c.response.SetContentType(ct)
}

// UnknownMediaType is raised by ContentType for an unrecognised media type,
// mirroring Sinatra's "Unknown media type" error.
type UnknownMediaType struct{ Type string }

func (e *UnknownMediaType) Error() string { return "Unknown media type: " + e.Type }

// Redirect performs a Sinatra redirect: it sets the Location header (resolving a
// relative target against the request URL) and halts with status 302 (or the
// given status). Like Sinatra, it unwinds the action immediately.
func (c *Context) Redirect(target string, status ...int) {
	code := 302
	if len(status) > 0 {
		code = status[0]
	}
	c.response.SetStatus(code)
	c.response.SetHeader("location", c.absoluteURI(target))
	panic(haltSignal{})
}

// Halt stops processing immediately, like Sinatra's `halt`. With no argument it
// keeps the current status; an int sets the status; a string sets the body;
// (int, string) sets both. Extra/!typed arguments are ignored.
func (c *Context) Halt(args ...any) {
	for _, a := range args {
		switch v := a.(type) {
		case int:
			c.response.SetStatus(v)
		case string:
			c.setBody(v)
		case []string:
			c.setBodyParts(v)
		}
	}
	panic(haltSignal{})
}

// Pass abandons the current route and tells the dispatcher to try the next
// matching route for the request method, like Sinatra's `pass`. If no further
// route matches, the not_found handler runs.
func (c *Context) Pass() { panic(passSignal{}) }

// URI builds an absolute URI for path, like Sinatra's url/uri helper. With an
// absolute target it is returned unchanged.
func (c *Context) URI(path string) string { return c.absoluteURI(path) }

// haltSignal and passSignal are the control-flow markers thrown by Halt/Redirect
// and Pass; the dispatcher recovers them.
type haltSignal struct{}
type passSignal struct{}

func (c *Context) setBody(s string) { c.setBodyParts([]string{s}) }

func (c *Context) setBodyParts(parts []string) {
	// Reset the rack.Response body to parts.
	resp := rack.NewResponse(parts, c.response.Status(), c.response.Headers())
	*c.response = *resp
}

// absoluteURI resolves target against the request's base URL, like Sinatra#uri.
// An absolute URL (with a scheme) or scheme-relative URL is returned as is.
func (c *Context) absoluteURI(target string) string {
	if hasScheme(target) || (len(target) >= 2 && target[0] == '/' && target[1] == '/') {
		return target
	}
	base := c.request.BaseURL()
	if len(target) > 0 && target[0] == '/' {
		return base + target
	}
	// Relative to the request path's directory.
	return base + "/" + target
}

func hasScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ':' {
			return i > 0
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.') {
			return false
		}
	}
	return false
}
