// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"reflect"
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestRouteNamedParam(t *testing.T) {
	s := New()
	s.Get("/hello/:name", func(c *Context) any { return "hi " + c.ParamString("name") })
	st, _, body := get(s, "/hello/world", "")
	if st != 200 || body != "hi world" {
		t.Errorf("got %d %q", st, body)
	}
}

func TestRouteFirstMatchWins(t *testing.T) {
	s := New()
	s.Get("/x", func(c *Context) any { return "first" })
	s.Get("/x", func(c *Context) any { return "second" })
	_, _, body := get(s, "/x", "")
	if body != "first" {
		t.Errorf("first-match: %q", body)
	}
}

func TestRouteSplatArray(t *testing.T) {
	s := New()
	s.Get("/say/*/to/*", func(c *Context) any {
		v, _ := c.Param("splat")
		arr := v.([]string)
		return arr[0] + "," + arr[1]
	})
	_, _, body := get(s, "/say/hi/to/you", "")
	if body != "hi,you" {
		t.Errorf("splat: %q", body)
	}
}

func TestRouteParamWinsOverQuery(t *testing.T) {
	s := New()
	s.Get("/m/:x", func(c *Context) any { return c.ParamString("x") })
	_, _, body := get(s, "/m/route", "x=query")
	if body != "route" {
		t.Errorf("merge: %q", body)
	}
}

func TestRouteQueryParamVisible(t *testing.T) {
	s := New()
	s.Get("/q", func(c *Context) any { return c.ParamString("a") + c.ParamString("b") })
	_, _, body := get(s, "/q", "a=1&b=2")
	if body != "12" {
		t.Errorf("query: %q", body)
	}
}

func TestRegexpRoute(t *testing.T) {
	s := New()
	p, err := CompileRegexp(`/regex/(\d+)`)
	if err != nil {
		t.Fatal(err)
	}
	s.RoutePattern(rack.MethodGet, p, func(c *Context) any {
		v, _ := c.Param("captures")
		return v.([]string)[0]
	})
	_, _, body := get(s, "/regex/123", "")
	if body != "123" {
		t.Errorf("regexp route: %q", body)
	}
}

func TestBeforeFilter(t *testing.T) {
	s := New()
	order := ""
	s.Before("", func(c *Context) any { order += "B"; return nil })
	s.Before("/f", func(c *Context) any { order += "F"; return nil })
	s.Get("/f", func(c *Context) any { order += "A"; return order })
	s.Get("/g", func(c *Context) any { order += "A"; return order })
	_, _, body := get(s, "/f", "")
	if body != "BFA" {
		t.Errorf("filtered order = %q", body)
	}
	order = ""
	_, _, body = get(s, "/g", "")
	if body != "BA" {
		t.Errorf("unfiltered order = %q", body)
	}
}

func TestAfterFilter(t *testing.T) {
	s := New()
	s.After("", func(c *Context) any { c.Header("X-After", "yes"); return nil })
	s.After("/only", func(c *Context) any { c.Header("X-Only", "1"); return nil })
	s.Get("/p", func(c *Context) any { return "p" })
	s.Get("/only", func(c *Context) any { return "o" })
	_, h, _ := get(s, "/p", "")
	if h.Get("X-After") != "yes" {
		t.Error("after filter did not run")
	}
	if h.Has("X-Only") {
		t.Error("path-scoped after ran on wrong path")
	}
	_, h, _ = get(s, "/only", "")
	if h.Get("X-Only") != "1" {
		t.Error("path-scoped after did not run")
	}
}

func TestAfterRunsOnHalt(t *testing.T) {
	s := New()
	s.After("", func(c *Context) any { c.Header("X-After", "yes"); return nil })
	s.Get("/h", func(c *Context) any { c.Halt(500, "boom"); return nil })
	st, h, body := get(s, "/h", "")
	if st != 500 || body != "boom" || h.Get("X-After") != "yes" {
		t.Errorf("after-on-halt: %d %q %v", st, body, h.Get("X-After"))
	}
}

func TestHaltStatusAndBody(t *testing.T) {
	s := New()
	s.Get("/a", func(c *Context) any { c.Halt(201, "halted"); return "never" })
	s.Get("/b", func(c *Context) any { c.Halt("just-body"); return "never" })
	s.Get("/c", func(c *Context) any { c.Halt(204); return "never" })
	s.Get("/d", func(c *Context) any { c.Halt([]string{"x", "y"}); return "never" })
	s.Get("/e", func(c *Context) any { c.Halt(); return "kept" })
	s.Get("/f", func(c *Context) any { c.Halt(1.5); return "ignored-float" })

	st, _, body := get(s, "/a", "")
	if st != 201 || body != "halted" {
		t.Errorf("halt(int,str) = %d %q", st, body)
	}
	st, _, body = get(s, "/b", "")
	if st != 200 || body != "just-body" {
		t.Errorf("halt(str) = %d %q", st, body)
	}
	st, _, _ = get(s, "/c", "")
	if st != 204 {
		t.Errorf("halt(int) = %d", st)
	}
	_, _, body = get(s, "/d", "")
	if body != "xy" {
		t.Errorf("halt([]string) = %q", body)
	}
	st, _, _ = get(s, "/e", "")
	if st != 200 {
		t.Errorf("bare halt status = %d", st)
	}
	st, _, _ = get(s, "/f", "")
	if st != 200 {
		t.Errorf("halt(unknown-type) status = %d", st)
	}
}

func TestPass(t *testing.T) {
	s := New()
	s.Get("/p", func(c *Context) any {
		if c.ParamString("skip") == "1" {
			c.Pass()
		}
		return "first"
	})
	s.Get("/p", func(c *Context) any { return "second" })
	_, _, body := get(s, "/p", "skip=1")
	if body != "second" {
		t.Errorf("pass -> %q", body)
	}
	_, _, body = get(s, "/p", "")
	if body != "first" {
		t.Errorf("no-pass -> %q", body)
	}
}

func TestPassToNotFound(t *testing.T) {
	s := New()
	s.Get("/only", func(c *Context) any { c.Pass(); return "x" })
	s.NotFound(func(c *Context) any { return "nf" })
	st, _, body := get(s, "/only", "")
	if st != 404 || body != "nf" {
		t.Errorf("pass-to-nf: %d %q", st, body)
	}
}

func TestRedirect(t *testing.T) {
	s := New()
	s.Get("/r", func(c *Context) any { c.Redirect("/target"); return nil })
	s.Get("/rp", func(c *Context) any { c.Redirect("/target", 301); return nil })
	s.Get("/abs", func(c *Context) any { c.Redirect("http://other/x"); return nil })

	st, h, _ := get(s, "/r", "")
	if st != 302 || h.Get("location") != "http://example.org/target" {
		t.Errorf("redirect: %d %v", st, h.Get("location"))
	}
	st, _, _ = get(s, "/rp", "")
	if st != 301 {
		t.Errorf("redirect 301: %d", st)
	}
	_, h, _ = get(s, "/abs", "")
	if h.Get("location") != "http://other/x" {
		t.Errorf("abs redirect: %v", h.Get("location"))
	}
}

func TestStatusAndBodyHelpers(t *testing.T) {
	s := New()
	s.Get("/s", func(c *Context) any {
		c.Status(418)
		c.Body("teapot")
		return nil
	})
	st, _, body := get(s, "/s", "")
	if st != 418 || body != "teapot" {
		t.Errorf("status/body: %d %q", st, body)
	}
}

func TestContentType(t *testing.T) {
	s := New()
	s.Get("/json", func(c *Context) any { c.ContentType("json"); return "{}" })
	s.Get("/xml", func(c *Context) any { c.ContentType("xml"); return "<x/>" })
	s.Get("/png", func(c *Context) any { c.ContentType("png"); return "" })
	s.Get("/text", func(c *Context) any { c.ContentType("text/plain"); return "x" })
	s.Get("/cs", func(c *Context) any { c.ContentTypeCharset("json", "iso-8859-1"); return "{}" })
	s.Get("/full", func(c *Context) any { c.ContentType("application/json"); return "{}" })
	s.Get("/default", func(c *Context) any { return "x" })

	cases := []struct{ path, want string }{
		{"/json", "application/json"},
		{"/xml", "application/xml;charset=utf-8"},
		{"/png", "image/png"},
		{"/text", "text/plain;charset=utf-8"},
		{"/cs", "application/json;charset=iso-8859-1"},
		{"/full", "application/json"},
		{"/default", "text/html;charset=utf-8"},
	}
	for _, c := range cases {
		_, h, _ := get(s, c.path, "")
		if got := h.Get("content-type"); got != c.want {
			t.Errorf("%s content-type = %v want %q", c.path, got, c.want)
		}
	}
}

func TestContentTypeUnknownPanics(t *testing.T) {
	s := New()
	s.Get("/u", func(c *Context) any { c.ContentType("nope"); return "x" })
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown media type")
		} else if e, ok := r.(*UnknownMediaType); !ok || e.Error() == "" {
			t.Errorf("wrong panic: %v", r)
		}
	}()
	get(s, "/u", "")
}

func TestNotFoundDefault(t *testing.T) {
	s := New()
	s.Get("/exists", func(c *Context) any { return "ok" })
	st, _, body := get(s, "/missing", "")
	if st != 404 {
		t.Errorf("missing status = %d", st)
	}
	if body != "" {
		t.Errorf("default 404 body should be empty (host renders): %q", body)
	}
}

func TestNotFoundHandler(t *testing.T) {
	s := New()
	s.NotFound(func(c *Context) any { return "custom-nf" })
	st, _, body := get(s, "/missing", "")
	if st != 404 || body != "custom-nf" {
		t.Errorf("nf handler: %d %q", st, body)
	}
}

func TestErrorHandlerByStatus(t *testing.T) {
	s := New()
	s.Get("/forbid", func(c *Context) any { c.Halt(403); return nil })
	s.Error(403, func(c *Context) any { return "forbidden!" })
	st, _, body := get(s, "/forbid", "")
	if st != 403 || body != "forbidden!" {
		t.Errorf("error handler: %d %q", st, body)
	}
}

func TestError404HandlerWinsOverNotFound(t *testing.T) {
	s := New()
	s.Error(404, func(c *Context) any { return "err404" })
	s.NotFound(func(c *Context) any { return "nf" })
	_, _, body := get(s, "/x", "")
	if body != "err404" {
		t.Errorf("error(404) should win: %q", body)
	}
}

func TestErrorHandlerNotInvokedWhenBodySet(t *testing.T) {
	s := New()
	s.Get("/h", func(c *Context) any { c.Halt(500, "explicit"); return nil })
	s.Error(500, func(c *Context) any { return "handler-body" })
	st, _, body := get(s, "/h", "")
	if st != 500 || body != "explicit" {
		t.Errorf("error handler should not overwrite body: %d %q", st, body)
	}
}

func TestVerbs(t *testing.T) {
	s := New()
	s.Post("/p", func(c *Context) any { return "post" })
	s.Put("/p", func(c *Context) any { return "put" })
	s.Delete("/p", func(c *Context) any { return "delete" })
	s.Patch("/p", func(c *Context) any { return "patch" })
	s.Options("/p", func(c *Context) any { return "options" })
	s.Head("/h", func(c *Context) any { return "head" })
	s.Route("LINK", "/l", func(c *Context) any { return "link" })

	cases := map[string]string{
		rack.MethodPost: "post", rack.MethodPut: "put", rack.MethodDelete: "delete",
		rack.MethodPatch: "patch", rack.MethodOptions: "options",
	}
	for m, want := range cases {
		_, _, body := call(s, mkEnv(m, "/p", ""))
		if body != want {
			t.Errorf("%s -> %q want %q", m, body, want)
		}
	}
	_, _, body := call(s, mkEnv(rack.MethodHead, "/h", ""))
	if body != "head" {
		t.Errorf("HEAD -> %q", body)
	}
	_, _, body = call(s, mkEnv("LINK", "/l", ""))
	if body != "link" {
		t.Errorf("LINK -> %q", body)
	}
}

func TestGetRegistersHead(t *testing.T) {
	s := New()
	s.Get("/g", func(c *Context) any { return "g" })
	_, _, body := call(s, mkEnv(rack.MethodHead, "/g", ""))
	if body != "g" {
		t.Errorf("HEAD of GET route -> %q", body)
	}
}

func TestEmptyPathDefaultsToRoot(t *testing.T) {
	s := New()
	s.Get("/", func(c *Context) any { return "root" })
	_, _, body := get(s, "", "")
	if body != "root" {
		t.Errorf("empty path -> %q", body)
	}
}

func TestUnknownMethodNotFound(t *testing.T) {
	s := New()
	s.Get("/g", func(c *Context) any { return "g" })
	s.NotFound(func(c *Context) any { return "nf" })
	_, _, body := call(s, mkEnv("PURGE", "/g", ""))
	if body != "nf" {
		t.Errorf("unknown method -> %q", body)
	}
}

func TestActionResultTypes(t *testing.T) {
	s := New()
	s.Get("/str", func(c *Context) any { return "s" })
	s.Get("/arr", func(c *Context) any { return []string{"a", "b"} })
	s.Get("/int", func(c *Context) any { c.Body("x"); return 503 })
	s.Get("/bytes", func(c *Context) any { return []byte("bb") })
	s.Get("/nil", func(c *Context) any { c.Body("kept"); return nil })
	s.Get("/other", func(c *Context) any { c.Body("kept2"); return 1.5 })

	_, _, body := get(s, "/str", "")
	if body != "s" {
		t.Errorf("str: %q", body)
	}
	_, _, body = get(s, "/arr", "")
	if body != "ab" {
		t.Errorf("arr: %q", body)
	}
	st, _, body := get(s, "/int", "")
	if st != 503 || body != "x" {
		t.Errorf("int: %d %q", st, body)
	}
	_, _, body = get(s, "/bytes", "")
	if body != "bb" {
		t.Errorf("bytes: %q", body)
	}
	_, _, body = get(s, "/nil", "")
	if body != "kept" {
		t.Errorf("nil: %q", body)
	}
	_, _, body = get(s, "/other", "")
	if body != "kept2" {
		t.Errorf("other type: %q", body)
	}
}

func TestParamsSnapshot(t *testing.T) {
	s := New()
	var keys []string
	var pairs []ParamPair
	s.Get("/u/:id/*", func(c *Context) any {
		keys = c.ParamKeys()
		pairs = c.Params()
		return "ok"
	})
	get(s, "/u/7/extra", "q=1")
	if len(pairs) == 0 {
		t.Fatal("no params")
	}
	want := map[string]bool{"q": true, "id": true, "splat": true}
	for _, k := range keys {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("missing param keys: %v (have %v)", want, keys)
	}
}

func TestHeadersHelper(t *testing.T) {
	s := New()
	s.Get("/h", func(c *Context) any {
		c.Headers(map[string]string{"X-A": "1", "X-B": "2"})
		return "x"
	})
	_, h, _ := get(s, "/h", "")
	if h.Get("X-A") != "1" || h.Get("X-B") != "2" {
		t.Errorf("headers: %v %v", h.Get("X-A"), h.Get("X-B"))
	}
}

func TestParamAccessors(t *testing.T) {
	s := New()
	s.Get("/a/:x", func(c *Context) any {
		if _, ok := c.Param("missing"); ok {
			t.Error("missing param reported present")
		}
		if c.ParamString("missing") != "" {
			t.Error("missing ParamString not empty")
		}
		if v, ok := c.Param("x"); !ok || v.(string) != "5" {
			t.Errorf("x param = %v", v)
		}
		c.params.set("arr", []string{"z"})
		if c.ParamString("arr") != "" {
			t.Error("non-string ParamString should be empty")
		}
		return "ok"
	})
	get(s, "/a/5", "")
}

func TestContextAccessors(t *testing.T) {
	s := New()
	s.Get("/c", func(c *Context) any {
		if c.Request() == nil || c.Response() == nil || c.App() == nil || c.Settings() == nil {
			t.Error("nil accessor")
		}
		if c.CurrentStatus() != 200 {
			t.Errorf("status = %d", c.CurrentStatus())
		}
		if c.URI("/p") != "http://example.org/p" {
			t.Errorf("URI = %q", c.URI("/p"))
		}
		if c.URI("rel") != "http://example.org/rel" {
			t.Errorf("URI rel = %q", c.URI("rel"))
		}
		if c.URI("//cdn/x") != "//cdn/x" {
			t.Errorf("URI scheme-rel = %q", c.URI("//cdn/x"))
		}
		if c.URI("https://x/y") != "https://x/y" {
			t.Errorf("URI abs = %q", c.URI("https://x/y"))
		}
		return "ok"
	})
	get(s, "/c", "")
}

func TestSession(t *testing.T) {
	s := New()
	s.Get("/sess", func(c *Context) any {
		if c.Session() == nil {
			return "no-sess"
		}
		return c.Session().(string)
	})
	env := mkEnv(rack.MethodGet, "/sess", "")
	env[rack.RackSession] = "the-store"
	_, _, body := call(s, env)
	if body != "the-store" {
		t.Errorf("session = %q", body)
	}
	_, _, body = get(s, "/sess", "")
	if body != "no-sess" {
		t.Errorf("no session = %q", body)
	}
}

func TestRouteAlias(t *testing.T) {
	s := New()
	s.Route(rack.MethodGet, "/ok", func(c *Context) any { return "ok" })
	_, _, body := get(s, "/ok", "")
	if body != "ok" {
		t.Errorf("Route alias: %q", body)
	}
}

func TestStatusText(t *testing.T) {
	if StatusText(404) != "Not Found" {
		t.Errorf("404 -> %q", StatusText(404))
	}
	if StatusText(999) != "" {
		t.Errorf("unknown -> %q", StatusText(999))
	}
}

func TestSplatAccumulatesAcrossFilterAndRoute(t *testing.T) {
	s := New()
	s.Before("/pre/*", func(c *Context) any { return nil })
	s.Get("/pre/*", func(c *Context) any {
		v, _ := c.Param("splat")
		return v
	})
	_, _, _ = get(s, "/pre/x", "")

	s.Get("/multi/*/*", func(c *Context) any {
		v, _ := c.Param("splat")
		if !reflect.DeepEqual(v, []string{"a", "b"}) {
			t.Errorf("multi splat = %v", v)
		}
		return "ok"
	})
	get(s, "/multi/a/b", "")
}
