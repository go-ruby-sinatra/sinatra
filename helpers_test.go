// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"strings"

	"github.com/go-ruby-rack/rack"
)

// mkEnv builds a minimal Rack environment for a GET/POST request.
func mkEnv(method, path, query string) rack.Env {
	return rack.Env{
		rack.RequestMethod: method,
		rack.PathInfo:      path,
		rack.QueryString:   query,
		rack.ServerName:    "example.org",
		rack.ServerPort:    "80",
		rack.RackURLScheme: "http",
	}
}

// call serves env and returns the SPEC tuple with the body joined to a string.
func call(s *Sinatra, env rack.Env) (status int, headers *rack.Headers, body string) {
	st, h, parts := s.CallTuple(env)
	return st, h, strings.Join(parts, "")
}

// get is the common case of a GET request.
func get(s *Sinatra, path, query string) (int, *rack.Headers, string) {
	return call(s, mkEnv(rack.MethodGet, path, query))
}
