# Function Reference

What you can do inside a handler, organized by need. If you are porting a filter written against the phase-based API, the [Deprecation Notice](deprecation.md) maps each old name to its replacement.

## Answering the request directly

Default to the reply helpers. They set the content type, record the status, and write the body in one call:

```go
return c.JSON(http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED"})
return c.String(http.StatusForbidden, "access denied")
```

Fall back to pre-encoded JSON for payloads you already marshalled:

```go
return c.JSON(http.StatusOK, json.RawMessage(raw))
```

How `net/http` habits port over: `http.Error(w, msg, code)` behaves like `return c.String(code, msg)`, plus the no-sniff header the standard library adds. A bare `return nil` after answering ends it; writing an answer and then calling `next` is the write-and-proceed bug and returns an error instead of proceeding.

## Passing the request through

Return without writing and without calling `next`: the request continues upstream unchanged. This is the cheap path, nothing parks and nothing waits for the response.

To proceed upstream *and* run code when the response arrives, wrap the handler in `Use` and call `next(c)`, then write your after-code. That call parks until the response arrives; see [The cost of observing the response](filter-authoring.md#the-cost-of-observing-the-response).

## Reading the request

- Method, host, path, query: `c.Method()`, `c.Host()`, `c.Path()`, `c.QueryParam(name)`. These are live; changing them via `c.SetMethod`, `c.SetHost`, `c.SetPath` changes what goes upstream.
- Headers: `c.Request().Header` with the standard `Get`, `Set`, `Add`, `Del` verbs. Mutations land on the live Envoy map through the request diff any time before proceeding.
- Body: `io.ReadAll(c.Body())` or `c.Bind(&v)` for JSON. Needs `gonvoy.ReadRequestBody()` at registration. Under the default gate the client must also send `X-Content-Operation: ReadOnly` (or `RO`); `gonvoy.OpenBodyAccess()` drops the gate.

## Writing the request

- Headers: `c.Request().Header.Set(key, val)` (or `Add` to append, `Del` to drop), any time before proceeding. Writes after proceeding are a no-op; reads stay valid.
- Body: assign `c.Request().Body = io.NopCloser(bytes.NewReader(modified))` before calling `next`. Needs `gonvoy.WriteRequestBody()`. Replacing the body drops `Content-Length` (chunked encoding) unless you pass `gonvoy.KeepContentLength(...)`.
- Host, method, path: `c.SetHost`, `c.SetMethod`, `c.SetPath`, then call `c.Reroute()` and register with `gonvoy.ReloadRoute()` so Envoy re-evaluates the route.

Reading without the matching body option fails with a "not accessible" error. Replacing without the write option returns 502.

## Reading the response

Available after `next` returns inside `Use`.

- Status: `c.Status()` (or `c.Response().StatusCode`). Zero means the upstream response has not been recorded yet, which is your cue that nothing was written.
- Headers: `c.Response().Header` with the standard `Get`, `Set`, `Add`, `Del` verbs to change what reaches the client. Writes before `next` merge into the pass-through response or stash for re-application on top of upstream; writes after `next` diff against the upstream snapshot.
- Body: `c.ResponseBody()` plus `io.ReadAll`. Needs `gonvoy.ReadResponseBody()`. It errors when body access is not enabled; treat that as "leave the response alone", not as fatal.

## Writing the response

How the bridge treats your after-`next` writes:

- No writes: the upstream response passes through with your header changes applied.
- Body-only write (no explicit status): `_, _ = c.Write(body)` replaces the buffered upstream body. Needs `gonvoy.WriteResponseBody()`, otherwise 502.
- Explicit status (for example `c.NoContent(code)`, `c.String`, or `c.JSON`): always wins a full local reply with your status, headers, and body.

## Plugin config

```go
cfg, err := gonvoy.ConfigOf[*Config](c)
```

Registered with `gonvoy.WithConfig(new(Config))`. Treat a missing config as "pass through" rather than fatal. Route config merges into listener config field by field; only `envoy:"mergeable"` struct fields take part (`envoy:"preserve_root"` keeps the listener value when the route value is empty). `gonvoy.RouteConfigWins()` lets the route replace the listener config wholesale, and `gonvoy.OnConfigError(gonvoy.UseListener)` falls back to the listener config when a merge fails (the default stops the plugin). If your config type implements `Destroy()`, it is called when Envoy deletes the config; keep it idempotent, as it can fire once per config instance the framework holds.

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `gonvoy.Attr`:

```go
upstream, _ := c.Attr("xds.cluster_name", "-")
code, _ := c.Attr("response.code", "-")
```

Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields use `c.Host()`, `c.Method()`, and `c.Path()` instead. The boolean reports whether the attribute exists; on miss you get your default with `false`, which keeps handlers unit-testable outside the bridge.

Attribute names worth knowing:

| Attribute | Meaning |
|---|---|
| `request.host`, `request.method`, `request.path` | The request line (prefer `c.Host()`, `c.Method()`, `c.Path()` for these) |
| `xds.cluster_name`, `xds.route_name` | Where Envoy is routing |
| `response.code`, `response.code_details`, `response.flags` | What the upstream answered (response phases only) |

## Routing

```go
c.SetPath("/staticreply")
c.Reroute()
return nil
```

Registered with `gonvoy.ReloadRoute()`. Host, method, path, and header changes all take part in the new decision; without the option the mutation applies but Envoy keeps the original route.

## Native Envoy access (advanced)

`gonvoy.RefreshRouteCache` bypasses the `net/http` vocabulary on purpose. It takes the standard context instead of the request:

```go
c.RefreshRoute()
```

It re-fetches the route immediately for the current request. Reach for it only when no same-callback mutations are pending: a synchronous re-fetch races Go-side header and path propagation and can resolve the stale route. If the handler mutated the request, use `c.Reroute` instead, whose lazy re-evaluation happens after the mutations land. Outside the bridge it is a no-op.

## Pinning the upstream host

`c.OverrideUpstream(host, strict)` bypasses the load balancer and sends this request directly to `host`, which must be an IP address with a port (`"10.0.0.9:8080"`). With `strict` true and the host unavailable, Envoy answers 503; otherwise it falls back to normal load balancing. An invalid host (not an IP with a port) answers 500. Unlike `Reroute`, this needs no registration option: the override decision is per request.

Use it for canary pinning or sticky debug routing, not as a replacement for routes. Routing still selects the cluster; the override only chooses the host inside it.

## Metrics

```go
c.Stats().Counter("requests_total", "host", host).Increment(1)
c.Stats().Gauge("inflight").Set(2)
```

Intents queue up and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy. Set a prefix with `gonvoy.MetricPrefix("myfilter_")`.

## Logging and shared state

```go
c.Log().Info("handled request", "path", c.Path())
cache := c.Cache()
```

The logger discards outside the bridge. The cache lives for the life of Envoy: `Store(key, value)` overwrites, `Load(key, &receiver)` needs a pointer to the stored type and reports `ok=false` on a miss or a type mismatch. Use it when setup is expensive or stateful.

## Secrets

`c.Secret(name)` reads a generic secret by name and reports whether it exists:

```go
if value, ok := c.Secret("api-token"); ok {
    c.Request().Header.Set("x-api-token", value)
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

`c.TLS()` snapshots the downstream TLS connection as it was when the request headers arrived:

```go
if tls, ok := c.TLS(); ok {
    _ = tls.SubjectPeerCertificate
}
```

The second return value is false for plaintext connections and outside the bridge. The snapshot carries the peer and local certificate subjects, SANs, TLS version, ciphersuite, validity window, session ID, and the URL-encoded PEM chain. String and slice fields are empty when Envoy reports them unavailable; the `*uint64`, `*uint16`, and `*string` fields are nil instead. No registration option is needed; snapshotting is always on because it never disturbs the stream.

## Draining the connection

`c.Drain()` marks the downstream connection for drain after the current request completes: `Connection: close` on HTTP/1.x, a GOAWAY frame on HTTP/2 and HTTP/3. No registration option is needed. Calling it is explicit and one-shot per request; use it when the backend signals it is going away, not as routine behavior.

## Deliberately not wrapped

Three newer Envoy APIs stay out on purpose:

- `InjectData`. The bridge replaces bodies with `Set` (complete body) and `AddData` (headers-only requests), which covers every handler need with no streaming special-cases. A raw inject entry point would only duplicate those paths.
- `HttpTcpBridge`. A different filter kind (HTTP-to-TCP bridging) with no `net/http` mapping. gonvoy is an HTTP-filter framework.
