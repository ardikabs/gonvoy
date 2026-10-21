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

Registered with `gonvoy.WithConfig(new(Config))`. Treat a missing config as "pass through" rather than fatal. Route config merges into listener config field by field; only `envoy:"mergeable"` struct fields take part (`envoy:"preserve_root"` keeps the listener value when the route value is empty). `gonvoy.RouteConfigWins()` lets the route replace the listener config wholesale, and `gonvoy.OnConfigError(gonvoy.UseListener)` falls back to the listener config when a merge fails (the default stops the plugin). If your config type implements `Destroy()`, it is called when Envoy deletes the config; keep it idempotent, as it can fire once per config instance the framework holds.

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `gonvoy.Attr`:

```go
upstream, _ := gonvoy.Attr(r, "xds.cluster_name", "-")
code, _ := gonvoy.Attr(r, "response.code", "-")
```

Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields use `r.Host`, `r.Method`, and `r.URL` instead. The boolean reports whether the attribute exists; on miss you get your default with `false`, which keeps handlers unit-testable outside the bridge.

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

Registered with `gonvoy.ReloadRoute()`. Host, method, path, and header changes all take part in the new decision; without the option the mutation applies but Envoy keeps the original route.

## Native Envoy access (advanced)

`gonvoy.RefreshRouteCache` bypasses the `net/http` vocabulary on purpose. It takes the standard context instead of the request:

```go
gonvoy.RefreshRouteCache(r.Context())
```

It re-fetches the route immediately for the current request. Reach for it only when no same-callback mutations are pending: a synchronous re-fetch races Go-side header and path propagation and can resolve the stale route. If the handler mutated the request, use `gonvoy.Reroute` instead, whose lazy re-evaluation happens after the mutations land. Outside the bridge it is a no-op.

The `r.Context()` shape is the convention for this kind of door: anything that talks raw Envoy semantics instead of HTTP semantics takes the context, so the vocabulary split stays visible at the call site.

## Pinning the upstream host

`gonvoy.OverrideUpstreamHost(r, host, strict)` bypasses the load balancer and sends this request directly to `host`, which must be an IP address with a port (`"10.0.0.9:8080"`). With `strict` true and the host unavailable, Envoy answers 503; otherwise it falls back to normal load balancing. An invalid host (not an IP with a port) answers 500. Unlike `Reroute`, this needs no registration option: the override decision is per request.

Use it for canary pinning or sticky debug routing, not as a replacement for routes. Routing still selects the cluster; the override only chooses the host inside it.

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

## Secrets

`gonvoy.Secret(r, name)` reads a generic secret by name and reports whether it exists:

```go
if value, ok := gonvoy.Secret(r, "api-token"); ok {
    r.Header.Set("x-api-token", value)
}
```

Secrets live in the golang filter's own config, not in global static resources. Declare each name the filter may read under that filter's `generic_secrets` list (with a `sds_config` for dynamic secrets, or bare to resolve a static secret):

```yaml
typed_config:
  "@type": type.googleapis.com/envoy.extensions.filters.http.golang.v3alpha.Config
  library_id: myfilter
  library_path: "/filter.so"
  plugin_name: myfilter
  generic_secrets:
    - name: api-token
```

The lookup runs on Envoy's callback thread even though your handler runs on its own goroutine; the bridge parks your handler until the value arrives. Outside the bridge it returns `"", false`.

## TLS connection info

`gonvoy.TLS(r)` snapshots the downstream TLS connection as it was when the request headers arrived:

```go
if tls, ok := gonvoy.TLS(r); ok {
    _ = tls.SubjectPeerCertificate
}
```

The second return value is false for plaintext connections and outside the bridge. The snapshot carries the peer and local certificate subjects, SANs, TLS version, ciphersuite, validity window, session ID, and the URL-encoded PEM chain. String and slice fields are empty when Envoy reports them unavailable; the `*uint64`, `*uint16`, and `*string` fields are nil instead. No registration option is needed; snapshotting is always on because it never disturbs the stream.

## Draining the connection

`gonvoy.DrainConnection(r)` marks the downstream connection for drain after the current request completes: `Connection: close` on HTTP/1.x, a GOAWAY frame on HTTP/2 and HTTP/3. No registration option is needed. Calling it is explicit and one-shot per request; use it when the backend signals it is going away, not as routine behavior.

## Deliberately not wrapped

Three newer Envoy APIs stay out on purpose:

- `InjectData`. The bridge replaces bodies with `Set` (complete body) and `AddData` (headers-only requests), which covers every handler need with no streaming special-cases. A raw inject entry point would only duplicate those paths.
- `HttpTcpBridge`. A different filter kind (HTTP-to-TCP bridging) with no `net/http` mapping. gonvoy is an HTTP-filter framework.
