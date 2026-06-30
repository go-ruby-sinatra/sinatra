<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-sinatra/brand/main/social/go-ruby-sinatra-sinatra.png" alt="go-ruby-sinatra/sinatra" width="720"></p>

# sinatra — go-ruby-sinatra

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-sinatra.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[Sinatra](https://github.com/sinatra/sinatra)** (Sinatra 4.x) — the route table,
the Mustermann-style pattern compiler, the request dispatcher (before/after
filters, `halt`/`pass`/`redirect`, `not_found`/`error` handlers) and the
assembly of the Rack `[status, headers, body]` tuple — matching the MRI
`sinatra` gem, **without any Ruby runtime**.

It is built on [go-ruby-rack](https://github.com/go-ruby-rack/rack), reusing
`rack.Request` to read the incoming environment and `rack.Response` to shape the
outgoing tuple, and it is a sibling of the other pure-Go Ruby front-ends
([go-ruby-regexp](https://github.com/go-ruby-regexp/regexp),
[go-ruby-erb](https://github.com/go-ruby-erb/erb),
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml)). It is the Sinatra backend
for [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby).

> **What it is — and isn't.** Compiling a route pattern, extracting `params`
> (named captures, splats, the query merge), ordering the filters and the
> `halt`/`pass` control flow are fully deterministic and need **no interpreter**,
> so they live here as pure Go. The route **action bodies** (the Ruby `do…end`
> blocks), the **session store**, and **template rendering** (ERB/Haml) are
> **seams** the embedding runtime (rbgo) supplies. The HTTP server — the socket
> accept loop, TLS, `Rack::Handler` — is the **host's** job and is out of scope.

## Features

Validated against the `sinatra` gem (4.x) via a differential MRI oracle:

- **Routing** — a route table per HTTP verb (`Get`/`Post`/`Put`/`Delete`/`Patch`/
  `Options`/`Head`, plus `Route` for arbitrary verbs). `Get` also registers
  `HEAD`, exactly like Sinatra.
- **Pattern compilation** — Mustermann-style `"/hello/:name"` named captures,
  `*` splats, the optional `?`, and raw regexp routes, compiled to an anchored
  `regexp` with the exact `:name → [^/?#]+`, `:name? → (…)?`, `* → .*?`
  semantics. URI-decoded captures; multiple splats accumulate into the
  `params["splat"]` array; regexp captures feed `params["captures"]`.
- **Params merge** — request (query + form) params seed `params`, then route
  captures override on collision via Sinatra's `v2 || v1` rule (a nil capture
  keeps the query value; an absent optional `:name?` still yields the key).
- **Dispatch** — first-match-wins ordering, `before`/`after` filters (with their
  own patterns and capture merge), `halt`/`pass`/`redirect`/`status`/
  `content_type`/`headers`/`body`, and the `not_found`/`error(code)` handlers,
  producing the Rack `[status, headers, body]` tuple.
- **Helpers** — `Params`/`Request`/`Response`/`Session` shaping, `content_type`
  mime resolution with Sinatra's `add_charset` rule, `url`/`uri` resolution,
  conditional `halt`, and a `set`/`enable`/`disable` settings registry.

**Out of scope (a noted seam):** templating (ERB/Haml) — rbgo's ERB compiler can
layer on top.

CGO-free, **100% test coverage**, `gofmt` + `go vet` clean, race-clean, and green
across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-sinatra/sinatra
```

## Usage

```go
package main

import (
	"strings"

	"github.com/go-ruby-rack/rack"
	"github.com/go-ruby-sinatra/sinatra"
)

func main() {
	app := sinatra.New()

	app.Get("/hello/:name", func(c *sinatra.Context) any {
		return "Hello, " + c.ParamString("name") + "!"
	})

	app.Get("/say/*/to/*", func(c *sinatra.Context) any {
		v, _ := c.Param("splat") // []string{"hello", "world"}
		return strings.Join(v.([]string), " ")
	})

	app.Before("/admin/*", func(c *sinatra.Context) any {
		c.Halt(401, "unauthorized")
		return nil
	})

	app.Get("/old", func(c *sinatra.Context) any {
		c.Redirect("/new") // 302, absolute Location
		return nil
	})

	app.NotFound(func(c *sinatra.Context) any { return "nothing here" })

	// Serve a Rack env; a host HTTP server feeds the env and writes the tuple.
	status, headers, body := app.CallTuple(rack.Env{
		rack.RequestMethod: "GET",
		rack.PathInfo:      "/hello/world",
		rack.ServerName:    "localhost",
		rack.ServerPort:    "9292",
		rack.RackURLScheme: "http",
	})
	_ = status
	_ = headers
	_ = body
}
```

### The action-body / session / template seams

A route or filter body is a `sinatra.Action` — `func(*sinatra.Context) any`. The
embedding runtime (rbgo) binds a Ruby `do…end` block to an `Action`; a Go caller
supplies one directly. The returned value is coerced to the response body
(string, `[]string`, `[]byte`, an `int` status, or `nil` to keep a body set via
`c.Body`/`c.Halt`). The control-flow helpers `c.Halt`, `c.Pass` and `c.Redirect`
unwind the action immediately, exactly like Sinatra's `throw :halt`/`:pass`.

The **session store** is reached through `c.Session()` (the `rack.session` env
value the host populates); this library only shapes access to it. **Templating**
is not included — rbgo's ERB compiler renders templates and feeds the result
back as the action body.

## Tests & coverage

```sh
GOWORK=off go test -race -cover ./...
```

The differential oracle compiles the same routes under the real `sinatra` gem
and compares the route→params, filter order, `halt`/`pass`/`redirect`,
`content_type` and the Rack tuple byte-for-byte. It skips itself where Ruby or
the gem is absent (Windows, the cross-arch qemu lanes); the deterministic,
Ruby-free suite alone holds coverage at **100%**.

## License

BSD-3-Clause © the go-ruby-sinatra/sinatra authors.
