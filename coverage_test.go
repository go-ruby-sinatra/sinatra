// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"reflect"
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestMimeTypeEdgeCases(t *testing.T) {
	if MimeType("") != "" {
		t.Errorf("empty -> %q", MimeType(""))
	}
	// A full media type (containing '/') is returned unchanged.
	if MimeType("application/custom") != "application/custom" {
		t.Errorf("full type -> %q", MimeType("application/custom"))
	}
	if MimeType("nope") != "" {
		t.Errorf("unknown -> %q", MimeType("nope"))
	}
	if MimeType(".json") != "application/json" {
		t.Errorf("ext -> %q", MimeType(".json"))
	}
}

func TestCompileBareColonLiteral(t *testing.T) {
	// A ':' not followed by a name character is a literal colon.
	p := Compile("/a:/b")
	if _, ok := p.Match("/a:/b"); !ok {
		t.Errorf("literal colon should match: %s", p.String())
	}
	if _, ok := p.Match("/aX/b"); ok {
		t.Error("literal colon must not act as a wildcard")
	}
}

func TestSubmatchNonParticipatingRegexpGroup(t *testing.T) {
	// A regexp route with an optional group that does not participate yields an
	// empty capture (submatch's start<0 branch).
	p, err := CompileRegexp(`/x(a)?(b)`)
	if err != nil {
		t.Fatal(err)
	}
	mr, ok := p.Match("/xb")
	if !ok {
		t.Fatal("no match")
	}
	// Two numbered groups; the first did not participate -> "".
	if !reflect.DeepEqual(mr.Splat, []string{"", "b"}) {
		t.Errorf("captures = %v want [\"\" \"b\"]", mr.Splat)
	}
}

func TestSeedQueryParamsBadEscapeIgnored(t *testing.T) {
	// An invalid %-escape in the query string makes rack's parser error;
	// seedQueryParams swallows it so the route still runs with no query params.
	s := New()
	s.Get("/q", func(c *Context) any {
		if c.ParamString("a") != "" {
			t.Error("bad query should yield no params")
		}
		return "ok"
	})
	_, _, body := get(s, "/q", "%zz=1")
	if body != "ok" {
		t.Errorf("bad-escape query -> %q", body)
	}
}

func TestDefaultContentTypeUnknownFallback(t *testing.T) {
	// When default_content_type is an unknown media type, the raw value is the
	// fallback (the !ok branch of defaultContentType).
	s := New()
	// A bare symbol with no '/' that is not a known mime resolves to "" in
	// MimeType, so buildContentType reports !ok and the raw value is used.
	s.Settings().Set("default_content_type", "totallybogus")
	s.Get("/d", func(c *Context) any { return "x" })
	_, h, _ := get(s, "/d", "")
	if h.Get("content-type") != "totallybogus" {
		t.Errorf("fallback content-type = %v", h.Get("content-type"))
	}
}

func TestSplatArrayAppendBranch(t *testing.T) {
	// A before-filter splat then a route splat: applyMatch hits the
	// existing-[]string append branch.
	s := New()
	s.Before("/pre/*", func(c *Context) any { return nil })
	s.Get("/pre/*", func(c *Context) any {
		v, _ := c.Param("splat")
		return v.([]string)
	})
	_, _, body := get(s, "/pre/abc", "")
	// filter splat "abc" + route splat "abc" accumulate.
	if body != "abcabc" {
		t.Errorf("accumulated splat body = %q", body)
	}
}

func TestSplatExistingNonArrayReplaced(t *testing.T) {
	// If "splat" is already set to a non-[]string (a query param named splat),
	// the route's splat replaces it (the else branch).
	s := New()
	s.Get("/s/*", func(c *Context) any {
		v, _ := c.Param("splat")
		return v
	})
	_, _, body := get(s, "/s/x", "splat=qq")
	// The route splat array wins; body is the single-element array joined.
	if body != "x" {
		t.Errorf("splat replace body = %q", body)
	}
}

func TestCallReturnsResponse(t *testing.T) {
	s := New()
	s.Get("/x", func(c *Context) any { return "ok" })
	resp := s.Call(mkEnv(rack.MethodGet, "/x", ""))
	if resp.Status() != 200 {
		t.Errorf("Call status = %d", resp.Status())
	}
}
