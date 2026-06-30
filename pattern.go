// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"regexp"
	"strings"
)

// Pattern is a compiled Sinatra route pattern. It is produced by [Compile]
// from a Mustermann-style string ("/hello/:name", "/say/*/to/*", "/posts/:id?")
// or wrapped around a raw regexp by [CompileRegexp].
//
// Matching a path yields the ordered named captures and the ordered list of
// splat ("*") values, mirroring how Sinatra populates params: a :name token
// becomes params["name"], every * (and a regexp group) feeds the
// params["splat"] / params["captures"] array, and all captured values are
// URI-decoded.
type Pattern struct {
	re *regexp.Regexp
	// caps lists the capture groups of re in order. Each entry names the
	// param the group feeds: a real name for ":name", or "" for a "*" splat.
	caps []capture
	// isRegexp marks a pattern built from a raw regexp; its numbered captures
	// feed params["captures"] rather than params["splat"].
	isRegexp bool
}

type capture struct {
	name  string // param name; "" means a splat ("*")
	group string // the unique group name inside re
}

// MatchResult holds the outcome of matching a path against a [Pattern].
type MatchResult struct {
	// Named maps each :name token to its URI-decoded captured value, in the
	// order the tokens appear. A nil capture (an absent optional :name?) is
	// omitted, matching Sinatra dropping the key.
	Named *orderedParams
	// Splat is the ordered list of URI-decoded "*" captures (or, for a regexp
	// pattern, its numbered captures).
	Splat []string
	// FromRegexp reports whether Splat came from a raw regexp pattern, so the
	// dispatcher knows to file it under params["captures"] not params["splat"].
	FromRegexp bool
}

// Compile turns a Mustermann-style Sinatra pattern into a [Pattern]. The
// grammar handled is the Sinatra default: ":name" named captures, "*" splats,
// a trailing "?" making the preceding token optional, and literal text (with
// regexp metacharacters escaped). It mirrors Mustermann's compiled regexp:
// :name -> (?P<n>[^/?#]+), :name? -> (?:(?P<n>[^/?#]+))? and * -> (.*?), all
// anchored with \A…\z.
//
// Every token in the grammar emits a valid regexp fragment — names are
// [A-Za-z0-9_], literals are quoted, and the structural pieces are fixed — so
// the assembled regexp always compiles; Compile therefore never returns a
// pattern error. A raw user regexp, which can be malformed, goes through
// [CompileRegexp] instead.
func Compile(pat string) *Pattern {
	var b strings.Builder
	b.WriteString(`\A`)
	caps := []capture{}
	splatN := 0
	nameN := 0
	i := 0
	for i < len(pat) {
		c := pat[i]
		switch c {
		case ':':
			// Read the name: word chars following the colon.
			j := i + 1
			for j < len(pat) && isNameChar(pat[j]) {
				j++
			}
			if j == i+1 {
				// A bare ':' with no name is a literal colon.
				b.WriteString(regexp.QuoteMeta(":"))
				i++
				continue
			}
			name := pat[i+1 : j]
			group := uniqueName("p", &nameN)
			optional := j < len(pat) && pat[j] == '?'
			if optional {
				b.WriteString(`(?:(?P<` + group + `>[^/?#]+))?`)
				j++
			} else {
				b.WriteString(`(?P<` + group + `>[^/?#]+)`)
			}
			caps = append(caps, capture{name: name, group: group})
			i = j
		case '*':
			group := uniqueName("s", &splatN)
			optional := i+1 < len(pat) && pat[i+1] == '?'
			if optional {
				b.WriteString(`(?:(?P<` + group + `>.*?))?`)
				i++
			} else {
				b.WriteString(`(?P<` + group + `>.*?)`)
			}
			caps = append(caps, capture{name: "", group: group})
			i++
		case '?':
			// A '?' not consumed by a preceding token makes the previous
			// literal character optional, like Mustermann. With no preceding
			// character it is a literal '?'.
			b.WriteString(`?`)
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	b.WriteString(`\z`)
	re := regexp.MustCompile(b.String())
	return &Pattern{re: re, caps: caps}
}

// CompileRegexp wraps a raw regexp source as a [Pattern]. Its numbered
// captures feed params["captures"], matching Sinatra's `get %r{…}` form. The
// source is anchored with \A…\z if not already.
func CompileRegexp(src string) (*Pattern, error) {
	re, err := regexp.Compile(`\A(?:` + src + `)\z`)
	if err != nil {
		return nil, err
	}
	caps := make([]capture, 0, re.NumSubexp())
	for i := 1; i <= re.NumSubexp(); i++ {
		caps = append(caps, capture{name: "", group: ""})
	}
	return &Pattern{re: re, caps: caps, isRegexp: true}, nil
}

// String returns the compiled regexp source, useful for debugging.
func (p *Pattern) String() string { return p.re.String() }

// Match tests path against the pattern. It returns the extracted captures and
// true on a match, or a zero [MatchResult] and false otherwise. Captured
// values are URI-decoded (Mustermann/Sinatra decode each segment).
func (p *Pattern) Match(path string) (MatchResult, bool) {
	loc := p.re.FindStringSubmatchIndex(path)
	if loc == nil {
		return MatchResult{}, false
	}
	res := MatchResult{Named: newOrderedParams(), FromRegexp: p.isRegexp}
	if p.isRegexp {
		// Numbered captures (skip the whole match at index 0) feed "captures".
		for i := 1; i <= p.re.NumSubexp(); i++ {
			res.Splat = append(res.Splat, decodeCapture(submatch(path, loc, i)))
		}
		return res, true
	}
	// Map group name -> capture index for quick lookup.
	idx := map[string]int{}
	for i, n := range p.re.SubexpNames() {
		if n != "" {
			idx[n] = i
		}
	}
	for _, c := range p.caps {
		gi := idx[c.group]
		// A skipped optional group reports a -1 start index.
		participated := loc[2*gi] >= 0
		if c.name == "" {
			if participated { // an absent optional splat is dropped
				res.Splat = append(res.Splat, decodeCapture(submatch(path, loc, gi)))
			}
			continue
		}
		if !participated {
			// An absent optional :name? still yields the key with a nil value
			// (Mustermann's named_captures reports nil); Sinatra's merge then
			// keeps any existing query value via `v2 || v1`.
			res.Named.set(c.name, nil)
			continue
		}
		res.Named.set(c.name, decodeCapture(submatch(path, loc, gi)))
	}
	return res, true
}

// submatch returns the i-th submatch string from a FindStringSubmatchIndex
// result, or "" when the group did not participate.
func submatch(path string, loc []int, i int) string {
	start, end := loc[2*i], loc[2*i+1]
	if start < 0 {
		return ""
	}
	return path[start:end]
}

func isNameChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func uniqueName(prefix string, n *int) string {
	*n++
	return prefix + itoa(*n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
