# sinatra examples

Runnable pure-Ruby usage of the `sinatra` routing DSL — declaring routes, filters and handlers on a `Sinatra::Base` subclass and serving a Rack environment — verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/sinatra_usage.rb
```

| File | Shows |
| --- | --- |
| `sinatra_usage.rb` | Define routes on a `Sinatra::Base` subclass with `get` (named `:name` captures and `*` splats read from `params`), short-circuit with a `before` filter and `halt`, issue a `redirect` (302 + absolute `location`), set `content_type`, register a `not_found` fallback, then serve each Rack env with `App.call` and inspect the `[status, headers, body]` triple. |
