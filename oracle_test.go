// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/go-ruby-rack/rack"
)

// rubyBin locates a usable `ruby` with the sinatra gem once. The oracle tests
// skip themselves when ruby (or the gem) is absent — the Windows lane, the
// cross-arch qemu lanes, and any host without sinatra — so the deterministic
// suite alone drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	if err := exec.Command(path, "-rsinatra/base", "-e", "1").Run(); err != nil {
		t.Skip("sinatra gem not installed; skipping MRI oracle")
	}
	return path
}

// sinatraOracle runs a Sinatra app under MRI and returns "status\tbody\tct" for
// a single GET request to path?query. The app is built from the route DSL in
// dsl. $stdout is binmoded so Windows text-mode never reshapes the bytes (the
// go-ruby-erb lesson), though the oracle already skips on Windows.
func sinatraOracle(t *testing.T, bin, dsl, path, query string) (status, body, ct string) {
	t.Helper()
	script := `
$stdout.binmode
require 'sinatra/base'
require 'stringio'
class App < Sinatra::Base
  set :environment, :test
  set :show_exceptions, false
  set :raise_errors, false
` + dsl + `
end
env = {
  'REQUEST_METHOD' => 'GET',
  'PATH_INFO' => ` + rbStr(path) + `,
  'QUERY_STRING' => ` + rbStr(query) + `,
  'rack.input' => StringIO.new(''),
  'rack.errors' => $stderr,
  'SERVER_NAME' => 'example.org',
  'SERVER_PORT' => '80',
  'rack.url_scheme' => 'http',
}
st, h, b = App.call(env)
body = +''
b.each { |x| body << x }
$stdout.write([st.to_s, body, (h['content-type'] || '')].join("\x1f"))
`
	out, err := exec.Command(bin, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\noutput:\n%s", err, out)
	}
	parts := strings.SplitN(string(out), "\x1f", 3)
	if len(parts) != 3 {
		t.Fatalf("unexpected oracle output: %q", out)
	}
	return parts[0], parts[1], parts[2]
}

// rbStr renders s as a Ruby single-quoted string literal.
func rbStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

// goResult runs the equivalent Go app and returns the same triple.
func goResult(s *Sinatra, path, query string) (status, body, ct string) {
	st, h, b := s.CallTuple(mkEnv(rack.MethodGet, path, query))
	c := ""
	if v := h.Get(rack.ContentTypeKey); v != nil {
		c, _ = v.(string)
	}
	return itoa(st), strings.Join(b, ""), c
}

func assertParity(t *testing.T, bin string, s *Sinatra, dsl, path, query string) {
	t.Helper()
	wSt, wBody, wCt := sinatraOracle(t, bin, dsl, path, query)
	gSt, gBody, gCt := goResult(s, path, query)
	if gSt != wSt || gBody != wBody || gCt != wCt {
		t.Errorf("parity %s?%s:\n  go  = %s %q %q\n  ruby= %s %q %q",
			path, query, gSt, gBody, gCt, wSt, wBody, wCt)
	}
}

func TestOracleRouteParams(t *testing.T) {
	bin := rubyBin(t)
	dsl := `
  get('/hello/:name') { params['name'] }
  get('/say/*/to/*') { params['splat'].join(',') }
  get('/download/*.*') { params['splat'].join('|') }
  get('/posts/:id?') { "id=#{params['id'].inspect}" }
  get('/:foo.:bar') { "#{params['foo']}/#{params['bar']}" }
  get('/decode/:name') { params['name'] }
  get('/merge/:x') { params['x'] }
`
	s := New()
	s.Get("/hello/:name", func(c *Context) any { return c.ParamString("name") })
	s.Get("/say/*/to/*", func(c *Context) any { v, _ := c.Param("splat"); return strings.Join(v.([]string), ",") })
	s.Get("/download/*.*", func(c *Context) any { v, _ := c.Param("splat"); return strings.Join(v.([]string), "|") })
	s.Get("/posts/:id?", func(c *Context) any {
		v, ok := c.Param("id")
		if !ok || v == nil {
			return "id=nil"
		}
		return "id=" + rbInspect(v.(string))
	})
	s.Get("/:foo.:bar", func(c *Context) any { return c.ParamString("foo") + "/" + c.ParamString("bar") })
	s.Get("/decode/:name", func(c *Context) any { return c.ParamString("name") })
	s.Get("/merge/:x", func(c *Context) any { return c.ParamString("x") })

	cases := []struct{ path, query string }{
		{"/hello/world", ""},
		{"/say/hello/to/world", ""},
		{"/download/path/to/file.xml", ""},
		{"/posts/5", ""},
		{"/posts/", ""},
		{"/article.html", ""},
		{"/decode/foo%20bar", ""},
		{"/merge/route", "x=query"},
	}
	for _, c := range cases {
		assertParity(t, bin, s, dsl, c.path, c.query)
	}
}

// rbInspect renders a Go string the way Ruby's String#inspect does for the
// simple ASCII values used in these oracle cases (wrapping in double quotes).
func rbInspect(s string) string { return `"` + s + `"` }

func TestOracleFilterOrder(t *testing.T) {
	bin := rubyBin(t)
	dsl := `
  before { @log = 'B' }
  before('/f') { @log += 'F' }
  after { response.headers['X-After'] = 'yes' }
  get('/f') { @log }
  get('/g') { @log }
`
	s := New()
	var log string
	s.Before("", func(c *Context) any { log = "B"; return nil })
	s.Before("/f", func(c *Context) any { log += "F"; return nil })
	s.After("", func(c *Context) any { c.Header("X-After", "yes"); return nil })
	s.Get("/f", func(c *Context) any { return log })
	s.Get("/g", func(c *Context) any { return log })

	// Reset log via a fresh evaluation each call: the Go closure shares state,
	// so re-run "B" assignment happens in the before filter.
	assertParity(t, bin, s, dsl, "/f", "")
	assertParity(t, bin, s, dsl, "/g", "")
}

func TestOracleHaltPassRedirect(t *testing.T) {
	bin := rubyBin(t)
	dsl := `
  get('/halt') { halt 201, 'halted' }
  get('/halt-body') { halt 'just-body' }
  get('/halt-status') { halt 204 }
  get('/redir') { redirect '/target' }
  get('/redir-perm') { redirect '/target', 301 }
  get('/pass') { pass if params['p'] == '1'; 'first' }
  get('/pass') { 'second' }
`
	s := New()
	s.Get("/halt", func(c *Context) any { c.Halt(201, "halted"); return nil })
	s.Get("/halt-body", func(c *Context) any { c.Halt("just-body"); return nil })
	s.Get("/halt-status", func(c *Context) any { c.Halt(204); return nil })
	s.Get("/redir", func(c *Context) any { c.Redirect("/target"); return nil })
	s.Get("/redir-perm", func(c *Context) any { c.Redirect("/target", 301); return nil })
	s.Get("/pass", func(c *Context) any {
		if c.ParamString("p") == "1" {
			c.Pass()
		}
		return "first"
	})
	s.Get("/pass", func(c *Context) any { return "second" })

	for _, c := range []struct{ path, query string }{
		{"/halt", ""}, {"/halt-body", ""}, {"/halt-status", ""},
		{"/redir", ""}, {"/redir-perm", ""},
		{"/pass", "p=1"}, {"/pass", ""},
	} {
		assertParity(t, bin, s, dsl, c.path, c.query)
	}
	// The redirect Location header is set; check it matches MRI too.
	_, h, _ := s.CallTuple(mkEnv(rack.MethodGet, "/redir", ""))
	if h.Get("location") != "http://example.org/target" {
		t.Errorf("redirect Location = %v", h.Get("location"))
	}
}

func TestOracleContentType(t *testing.T) {
	bin := rubyBin(t)
	dsl := `
  get('/json') { content_type :json; '{}' }
  get('/xml') { content_type :xml; '<x/>' }
  get('/png') { content_type :png; 'x' }
  get('/text') { content_type 'text/plain'; 'x' }
  get('/svg') { content_type :svg; 'x' }
  get('/default') { 'x' }
`
	s := New()
	s.Get("/json", func(c *Context) any { c.ContentType("json"); return "{}" })
	s.Get("/xml", func(c *Context) any { c.ContentType("xml"); return "<x/>" })
	s.Get("/png", func(c *Context) any { c.ContentType("png"); return "x" })
	s.Get("/text", func(c *Context) any { c.ContentType("text/plain"); return "x" })
	s.Get("/svg", func(c *Context) any { c.ContentType("svg"); return "x" })
	s.Get("/default", func(c *Context) any { return "x" })

	for _, p := range []string{"/json", "/xml", "/png", "/text", "/svg", "/default"} {
		assertParity(t, bin, s, dsl, p, "")
	}
}

func TestOracleNotFoundStatus(t *testing.T) {
	bin := rubyBin(t)
	dsl := `
  get('/exists') { 'ok' }
`
	s := New()
	s.Get("/exists", func(c *Context) any { return "ok" })
	// Only compare the status for the missing route (the default 404 body is
	// host-rendered dev chrome and out of scope here).
	wSt, _, _ := sinatraOracle(t, bin, dsl, "/missing", "")
	gSt, _, _ := goResult(s, "/missing", "")
	if gSt != wSt {
		t.Errorf("404 status parity: go=%s ruby=%s", gSt, wSt)
	}
	// The matched route is full parity.
	assertParity(t, bin, s, dsl, "/exists", "")
}
