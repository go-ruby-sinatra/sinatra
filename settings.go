// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

// Settings is the per-application registry behind Sinatra's set/enable/disable.
// Values are arbitrary; the dispatcher reads a handful of well-known keys
// (default_content_type, default_encoding). It is insertion-ordered so a
// caller can enumerate settings deterministically.
type Settings struct {
	keys   []string
	values map[string]any
}

func newSettings() *Settings {
	s := &Settings{values: map[string]any{}}
	// Sinatra defaults the dispatcher relies on.
	s.Set("default_content_type", "text/html")
	s.Set("default_encoding", "utf-8")
	return s
}

// Set assigns key to val (Sinatra's `set`), appending new keys in order.
func (s *Settings) Set(key string, val any) {
	if _, ok := s.values[key]; !ok {
		s.keys = append(s.keys, key)
	}
	s.values[key] = val
}

// Enable sets key to true (Sinatra's `enable`).
func (s *Settings) Enable(key string) { s.Set(key, true) }

// Disable sets key to false (Sinatra's `disable`).
func (s *Settings) Disable(key string) { s.Set(key, false) }

// Get returns the value for key and whether it was set.
func (s *Settings) Get(key string) (any, bool) {
	v, ok := s.values[key]
	return v, ok
}

// Bool returns the boolean value of key, or false if unset or non-bool. It
// mirrors Sinatra treating a truthy setting (used by enable?/disable?).
func (s *Settings) Bool(key string) bool {
	v, ok := s.values[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// String returns the string value of key, or "" if unset or non-string.
func (s *Settings) String(key string) string {
	v, ok := s.values[key]
	if !ok {
		return ""
	}
	str, ok := v.(string)
	if !ok {
		return ""
	}
	return str
}

// Keys returns the setting keys in insertion order (a copy).
func (s *Settings) Keys() []string {
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	return out
}
