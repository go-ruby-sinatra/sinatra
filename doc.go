// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package sinatra is a pure-Go (no cgo) reimplementation of the deterministic
// core of Ruby's Sinatra web framework (Sinatra 4.x) — the route table, the
// Mustermann-style pattern compiler, the request dispatcher (before/after
// filters, halt/pass/redirect, not_found/error handlers) and the assembly of
// the Rack [status, headers, body] tuple — matching the MRI sinatra gem.
//
// It is built on github.com/go-ruby-rack/rack, reusing rack.Request to read
// the incoming environment and rack.Response to shape the outgoing tuple, and
// it is a sibling of the other go-ruby-* front-ends
// (go-ruby-regexp, go-ruby-erb, go-ruby-yaml, go-ruby-rack).
//
// # What it is — and isn't
//
// Compiling a route pattern to a matcher, extracting params (named captures,
// splats, query merge), ordering the filters, and the halt/pass control flow
// are all fully deterministic and need no interpreter, so they live here as
// pure Go. The action bodies (the `do…end` blocks of a Ruby route), the
// session store, and template rendering (ERB/Haml) are SEAMS the embedding
// runtime (go-embedded-ruby's rbgo) supplies. The HTTP server — the socket
// accept loop, TLS, Rack::Handler — is the host's job and is out of scope.
//
// An action body is modelled as an [Action]: a Go func given a *[Context] that
// returns a body (or invokes Halt/Pass/Redirect on the context). rbgo binds a
// Ruby block to an Action; a Go caller can supply one directly.
package sinatra
