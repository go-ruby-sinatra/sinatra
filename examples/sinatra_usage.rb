# frozen_string_literal: true
#
# Usage of Sinatra — the class-level routing DSL added by `require "sinatra"`.
# A Sinatra::Base subclass declares routes/filters/handlers; `.call(env)`
# serves a Rack environment and returns the [status, headers, body] triple.
# Runs under go-embedded-ruby (rbgo); see examples/README.md.

require "sinatra"

class App < Sinatra::Base
  # A route with a named capture, read back from `params`.
  get "/hello/:name" do
    "Hello, #{params["name"]}!"
  end

  # Splats (`*`) accumulate into params["splat"].
  get "/say/*/to/*" do
    params["splat"].join(" ")
  end

  # A before-filter can short-circuit the request with halt.
  before "/admin/*" do
    halt 401, "unauthorized"
  end

  # redirect sets a 302 and an absolute Location header.
  get "/old" do
    redirect "/new"
  end

  # content_type shapes the response header.
  get "/data.json" do
    content_type "application/json"
    '{"ok":true}'
  end

  # The fallback for an unmatched path.
  not_found do
    "nothing here"
  end
end

# Serve a minimal Rack env; a host HTTP server would feed this and write back.
def env(path)
  {
    "REQUEST_METHOD" => "GET", "PATH_INFO" => path,
    "SERVER_NAME" => "localhost", "SERVER_PORT" => "9292",
    "rack.url_scheme" => "http"
  }
end

status, _headers, body = App.call(env("/hello/world"))
p [status, body]                            # => [200, ["Hello, world!"]]

_s, _h, splat = App.call(env("/say/hi/to/there"))
p splat                                     # => ["hi there"]

code, _h, msg = App.call(env("/admin/panel"))
p [code, msg]                               # => [401, ["unauthorized"]]

code, headers, _b = App.call(env("/old"))
p [code, headers["location"]]               # => [302, "http://localhost:9292/new"]

_s, headers, body = App.call(env("/data.json"))
p [headers["content-type"], body]           # => ["application/json", ["{\"ok\":true}"]]

code, _h, body = App.call(env("/missing"))
p [code, body]                              # => [404, ["nothing here"]]
