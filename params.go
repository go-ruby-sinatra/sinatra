// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import "net/url"

// orderedParams is a small insertion-ordered string->any map used to build the
// params hash with Ruby-like deterministic key order. A value is either a
// string (a scalar param) or a []string (the splat / captures arrays).
type orderedParams struct {
	keys   []string
	values map[string]any
}

func newOrderedParams() *orderedParams {
	return &orderedParams{values: map[string]any{}}
}

// Len reports the number of keys.
func (p *orderedParams) Len() int { return len(p.keys) }

func (p *orderedParams) set(key string, val any) {
	if _, ok := p.values[key]; !ok {
		p.keys = append(p.keys, key)
	}
	p.values[key] = val
}

// get returns the value for key and whether it was present.
func (p *orderedParams) get(key string) (any, bool) {
	v, ok := p.values[key]
	return v, ok
}

// has reports whether key is present.
func (p *orderedParams) has(key string) bool {
	_, ok := p.values[key]
	return ok
}

// keysInOrder returns the keys in insertion order (a copy).
func (p *orderedParams) keysInOrder() []string {
	out := make([]string, len(p.keys))
	copy(out, p.keys)
	return out
}

// mergeFrom overlays other (the route captures) onto p (seeded with request
// params) in place, mirroring Sinatra's
//
//	@params = @params.merge(params) { |_k, v1, v2| v2 || v1 }
//
// the route value (v2) wins unless it is nil, in which case any existing
// request value (v1) is kept. A key absent from p is added even when its route
// value is nil.
func (p *orderedParams) mergeFrom(other *orderedParams) {
	if other == nil {
		return
	}
	for _, k := range other.keys {
		v2 := other.values[k]
		if v2 == nil {
			if _, exists := p.values[k]; exists {
				continue // keep existing v1
			}
		}
		p.set(k, v2)
	}
}

// decodeCapture URI-decodes a captured route segment the way Mustermann does:
// '+' is left literal (unlike form decoding) and an invalid escape is returned
// unchanged.
func decodeCapture(s string) string {
	if !needsDecode(s) {
		return s
	}
	dec, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return dec
}

func needsDecode(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			return true
		}
	}
	return false
}
