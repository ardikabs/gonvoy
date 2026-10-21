# Function Reference

What you can do inside a handler, organized by need. If you are porting a filter written against the phase-based API, the [Deprecation Notice](deprecation.md) maps each old name to its replacement.

## Answering the request directly

Default to the reply helpers. They set the content type, record the status, and write the body in one call:

```go
gonvoy.JSON(w, http.StatusUnauthorized, gonvoy.NewMinimalJSONResponse("UNAUTHORIZED", "Unauthorized"))
gonvoy.String(w, http.StatusForbidden, "access denied")
```

Fall back to raw calls for any payload that is neither JSON nor plain text:

```go
w.Header().Set("Content-Type", "application/octet-stream")
w.WriteHeader(http.StatusOK)
_, _ = w.Write(raw)
```

How `net/http` habits port over: `http.Error(w, msg, code)` behaves like `gonvoy.String` with the same arguments, plus the no-sniff header the standard library adds. A bare `return` after writing answers directly; calling `next` (or `Terminal`) after writing is the write-and-next bug and answers 500 instead.

## Passing the request through

Return without writing and without calling `next`: the request continues upstream unchanged. This is the cheap path, nothing parks and nothing waits for the response.

To proceed upstream *and* run code when the response arrives, call `gonvoy.Terminal{}.ServeHTTP(w, r)` (single handler) or `next.ServeHTTP(w, r)` (inside middleware), then write your after-code. That call parks until the response arrives; see [`gonvoy.Terminal` is an advanced tool](filter-authoring.md#gonvoyterminal-is-an-advanced-tool).

## Reading the request

- Method, host, path, query: `r.Method`, `r.Host`, `r.URL.Path`, `r.URL.Query()`. These are live; changing them changes what goes upstream.
- Headers: `r.Header.Get`, `r.Header.Values`, `r.Header.Set`, `r.Header.Add`, `r.Header.Del`.
- Body: `io.ReadAll(r.Body)`. Needs `gonvoy.ReadRequestBody()` at registration. Under the default gate the client must also send `X-Content-Operation: ReadOnly` (or `RO`); `gonvoy.OpenBodyAccess()` drops the gate.

## Writing the request

- Headers: `r.Header.Set` / `Add` / `Del`, any time before proceeding.
- Body: assign `r.Body = io.NopCloser(bytes.NewReader(modified))` before `next` or `Terminal`. Needs `gonvoy.WriteRequestBody()`. Replacing the body drops `Content-Length` (chunked encoding) unless you pass `gonvoy.KeepContentLength(...)`.
- Host, method, path: assign `r.Host`, `r.Method`, `r.URL.Path`, then call `gonvoy.Reroute(r)` and register with `gonvoy.ReloadRoute()` so Envoy re-evaluates the route.

Reading without the matching body option fails with a "not accessible" error. Replacing without the write option returns 502.

## Reading the response

Available after `next` (or `Terminal`) returns.

- Status: `w.Status()`. Zero means the upstream response has not been recorded yet, which is your cue that nothing was written.
- Headers: `w.Header().Get`, plus `Set` / `Add` / `Del` to change what reaches the client.
- Body: `gonvoy.ResponseBody(r)` plus `io.ReadAll`. Needs `gonvoy.ReadResponseBody()`. It errors when body access is not enabled; treat that as "leave the response alone", not as fatal.

## Writing the response

How the bridge treats your after-`next` writes:

- No writes: the upstream response passes through with your header changes applied.
- Body-only write (no explicit status): replaces the buffered upstream body. Needs `gonvoy.WriteResponseBody()`, otherwise 502.
- Explicit `w.WriteHeader(code)`: always wins a full local reply with your status, headers, and body.

## Plugin config

```go
cfg, err := gonvoy.Config[*Config](r)
```

Registered with `gonvoy.WithConfig(new(Config))`. Treat a missing config as "pass through" rather than fatal. Route config merges into listener config field by field; only `envoy:"mergeable"` struct fields take part (`envoy:"preserve_root"` keeps the listener value when the route value is empty). `gonvoy.RouteConfigWins()` lets the route replace the listener config wholesale, and `gonvoy.OnConfigError(envoy.UseListener)` falls back to the listener config when a merge fails (the default stops the plugin).

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `gonvoy.Attr`:

```go
upstream, _ := gonvoy.Attr(r, "xds.cluster_name", "-")
code, _ := gonvoy.Attr(r, "response.code", "-")
```

Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields use `r.Host`, `r.Method`, and `r.URL` instead. Outside the bridge `Attr` returns your default, which keeps handlers unit-testable.

Attribute names worth knowing:

| Attribute | Meaning |
|---|---|
| `request.host`, `request.method`, `request.path` | The request line (prefer `r.Host`, `r.Method`, `r.URL` for these) |
| `xds.cluster_name`, `xds.route_name` | Where Envoy is routing |
| `response.code`, `response.code_details`, `response.flags` | What the upstream answered (response phases only) |

## Routing

```go
r.URL.Path = "/staticreply"
gonvoy.Reroute(r)
gonvoy.Terminal{}.ServeHTTP(w, r)
```

Registered with `gonvoy.ReloadRoute()`. Host, method, path, and header changes all take part in the new decision; without the option the mutations apply but Envoy keeps the original route.

## Metrics

```go
gonvoy.Stats(r).Counter("requests_total", "host", host).Increment(1)
gonvoy.Stats(r).Gauge("inflight").Set(2)
```

Intents queue up and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy. Set a prefix with `gonvoy.MetricPrefix("myfilter_")`.

## Logging and shared state

```go
gonvoy.Log(r).Info("handled request", "path", r.URL.Path)
cache := gonvoy.SharedCache(r)
```

The logger discards outside the bridge. The cache lives for the life of Envoy: `Store(key, value)` overwrites, `Load(key, &receiver)` needs a pointer to the stored type and reports `ok=false` on a miss or a type mismatch. Use it when setup is expensive or stateful.
