// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"reflect"
	"testing"
)

func namedMap(mr MatchResult) map[string]any {
	out := map[string]any{}
	if mr.Named == nil {
		return out
	}
	for _, k := range mr.Named.keys {
		out[k] = mr.Named.values[k]
	}
	return out
}

func TestCompileNamed(t *testing.T) {
	p := Compile("/hello/:name")
	mr, ok := p.Match("/hello/world")
	if !ok {
		t.Fatal("expected match")
	}
	if got := namedMap(mr); !reflect.DeepEqual(got, map[string]any{"name": "world"}) {
		t.Errorf("named = %v", got)
	}
	if _, ok := p.Match("/hello/world/x"); ok {
		t.Error("should not match extra segment")
	}
	if _, ok := p.Match("/hello"); ok {
		t.Error("should not match missing segment")
	}
}

func TestCompileSplat(t *testing.T) {
	p := Compile("/say/*/to/*")
	mr, ok := p.Match("/say/hello/to/world")
	if !ok {
		t.Fatal("no match")
	}
	if !reflect.DeepEqual(mr.Splat, []string{"hello", "world"}) {
		t.Errorf("splat = %v", mr.Splat)
	}
}

func TestCompileSplatDot(t *testing.T) {
	p := Compile("/download/*.*")
	mr, ok := p.Match("/download/path/to/file.xml")
	if !ok {
		t.Fatal("no match")
	}
	if !reflect.DeepEqual(mr.Splat, []string{"path/to/file", "xml"}) {
		t.Errorf("splat = %v", mr.Splat)
	}
}

func TestCompileOptional(t *testing.T) {
	p := Compile("/posts/:id?")
	// Bare /posts does not match (the literal '/' is required).
	if _, ok := p.Match("/posts"); ok {
		t.Error("/posts should not match /posts/:id?")
	}
	// Trailing slash matches with a nil id (the key is present, value nil).
	mr, ok := p.Match("/posts/")
	if !ok {
		t.Fatal("/posts/ should match")
	}
	if got := namedMap(mr); !reflect.DeepEqual(got, map[string]any{"id": nil}) {
		t.Errorf("named = %v want {id:nil}", got)
	}
	mr, _ = p.Match("/posts/5")
	if got := namedMap(mr); !reflect.DeepEqual(got, map[string]any{"id": "5"}) {
		t.Errorf("named = %v", got)
	}
}

func TestCompileMultiOptional(t *testing.T) {
	p := Compile("/opt/:x?/:y?")
	for _, path := range []string{"/opt", "/opt/1"} {
		if _, ok := p.Match(path); ok {
			t.Errorf("%s should not match /opt/:x?/:y?", path)
		}
	}
	mr, ok := p.Match("/opt/1/2")
	if !ok {
		t.Fatal("/opt/1/2 should match")
	}
	if got := namedMap(mr); !reflect.DeepEqual(got, map[string]any{"x": "1", "y": "2"}) {
		t.Errorf("named = %v", got)
	}
}

func TestCompileTwoNames(t *testing.T) {
	p := Compile("/:foo.:bar")
	mr, ok := p.Match("/article.html")
	if !ok {
		t.Fatal("no match")
	}
	if got := namedMap(mr); !reflect.DeepEqual(got, map[string]any{"foo": "article", "bar": "html"}) {
		t.Errorf("named = %v", got)
	}
}

func TestCompileDecode(t *testing.T) {
	p := Compile("/d/:name")
	mr, _ := p.Match("/d/foo%20bar")
	if mr.Named.values["name"] != "foo bar" {
		t.Errorf("decode = %v", mr.Named.values["name"])
	}
	// An invalid escape is left unchanged.
	mr, _ = p.Match("/d/foo%zzbar")
	if mr.Named.values["name"] != "foo%zzbar" {
		t.Errorf("bad-escape = %v", mr.Named.values["name"])
	}
}

func TestCompileOptionalSplat(t *testing.T) {
	p := Compile("/files/*?")
	// Mustermann: "/files/" -> splat [""], "/files/a" -> ["a"], "/files" -> no.
	mr, ok := p.Match("/files/")
	if !ok {
		t.Fatal("/files/ should match optional splat")
	}
	if !reflect.DeepEqual(mr.Splat, []string{""}) {
		t.Errorf("splat for /files/ = %v want [\"\"]", mr.Splat)
	}
	mr, _ = p.Match("/files/a")
	if !reflect.DeepEqual(mr.Splat, []string{"a"}) {
		t.Errorf("splat for /files/a = %v", mr.Splat)
	}
	if _, ok := p.Match("/files"); ok {
		t.Error("/files should not match /files/*?")
	}
}

func TestCompileLiteralColonAndQuestion(t *testing.T) {
	// A bare ':' with no following name is a literal colon.
	p := Compile("/a:b")
	if _, ok := p.Match("/a:b"); !ok {
		t.Error("literal colon should match")
	}
	// A leading '?' with nothing before it is a literal '?'.
	p2 := Compile("?x")
	if _, ok := p2.Match("?x"); !ok {
		t.Error("literal question mark should match")
	}
}

func TestCompileRegexpError(t *testing.T) {
	// The Mustermann grammar always emits a valid regexp, so Compile cannot
	// fail; a raw user regexp can, and that error surfaces via CompileRegexp.
	if _, err := CompileRegexp("("); err == nil {
		t.Error("expected regexp compile error")
	}
}

func TestCompileRegexpMatch(t *testing.T) {
	p, err := CompileRegexp(`/regex/(\d+)`)
	if err != nil {
		t.Fatal(err)
	}
	mr, ok := p.Match("/regex/123")
	if !ok {
		t.Fatal("no match")
	}
	if !mr.FromRegexp {
		t.Error("FromRegexp should be true")
	}
	if len(mr.Splat) != 1 || mr.Splat[0] != "123" {
		t.Errorf("captures = %v", mr.Splat)
	}
	if _, ok := p.Match("/regex/abc"); ok {
		t.Error("should not match non-digits")
	}
}

func TestPatternString(t *testing.T) {
	p := Compile("/x")
	if p.String() == "" {
		t.Error("String() empty")
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 42: "42", 100: "100"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q want %q", in, got, want)
		}
	}
}
