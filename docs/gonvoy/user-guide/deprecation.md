# Deprecation Notice

> **Retired:** the entire phase-based API described on this page — including `envoy.Register`, `envoy.MustRegister`, and `envoy.RegisterHttpFilter` — has been removed from the tree. Only `envoy.Handle` (bridge handler API) remains. This page is kept as a porting reference.

gonvoy has two APIs. The handler API (`HandlerFunc`, `Use`, `Terminal`) is the default for all new filters. The phase-based API below it no longer compiles; it is removed, not merely superseded. This page writes up the whole old surface so you can maintain existing filters and port them with intent.

Only two names carry an explicit deprecation marker. Everything else in this page works exactly as before.

## Explicitly deprecated

| Name | Status | Replacement |
|---|---|---|
| `RuntimeContext.GetProperty` | Deprecated marker in code: use `Property` instead | `c.Property(name, def)` on the old API; `gonvoy.Attr(r, name, def)` on the new one |
| `envoy.RegisterHttpFilter` | Removed, no marker left in tree | `envoy.Handle(name, handler, ...)` on the new one |

## The old mechanism, area by area

### Registration and config

- `envoy.MustRegister(name, envoy.Options{New, Config, MetricsPrefix, Route, Body, Merge})` onboarded a filter. **Removed; no marker left in tree.**
- `envoy.RegisterHttpFilter(name, factoryFunc, gonvoy.ConfigOptions{...})` was the older entry point and is removed (see above).
- Per-route config arrives as `typed_per_filter_config` in `envoy.yaml` and merges into the listener config.

### Filter shape

- `HttpFilter` has two methods: `OnBegin(c RuntimeContext, ctrl HttpFilterController)` for setup (read config, add handlers, define metrics; an error ignores the filter) and `OnComplete(c Context)` for teardown (errors are ignored).
- Phase interfaces attach behavior: `OnRequestHeader`, `OnRequestBody`, `OnResponseHeader`, `OnResponseBody`, each returning an error. A returned error becomes a reply through the default error handler; `nil` continues.
- `Composer` builds the per-request chain with `Use(c Context) ([]Filter, error)`. It owns position zero of its own chain: its own phases and `OnComplete` run first.
- `PassthroughHttpFilterHandler` is the embed for handlers that implement only some phases.
- `HttpFilterController.AddHandler` registers phase handlers from `OnBegin`. The manager walks them in order.

### Reading the request and response

- `c.RequestHeader()` / `c.ResponseHeader()` return a `Header` (an Envoy header map plus `Export`/`Import`). `Set`, `Add`, `Del` apply immediately but panic outside their phases.
- `c.Request()` / `c.Response()` return read-only snapshots. Writing to them does nothing; they exist for inspection only.
- `c.RequestBody()` / `c.ResponseBody()` return a `Body` (`Bytes()`, `String()`, plus `io.Writer` for replacement). Access panics unless the matching body option is on and the phase allows it.

### Replying

- `c.JSON(code, body, ...)`, `c.String(code, text, ...)`, `c.SendResponse(code, text, ...)` answer directly and halt the chain.
- Returned errors map to statuses in the default error handler: `ErrUnauthorized` to 401, `ErrAccessDenied` to 403, `ErrOperationNotPermitted` (for example, writing a body without the write option) to 502, `ErrClientClosedRequest` to 499, anything else to 500.
- `c.SkipNextPhase()` ends processing from inside a phase: from `OnRequestHeader` it skips the body phase, and analogously on the response side.

### Routing and attributes

- `c.SetRequestHost` / `SetRequestMethod` / `c.SetRequestPath` rewrite the request line; `c.ReloadRoute()` (or the auto-reload option) makes Envoy re-evaluate the route.
- `c.Property(name, def)` fetches Envoy attributes (`xds.cluster_name`, `response.code`, and the rest). `gonvoy.MustGetProperty` is the panicking helper.
- `gonvoy.FilterConfig[T](c)` reads the typed plugin config; `gonvoy.FilterCache(c)` returns the filter-wide `Cache` (`Store` overwrites, `Load` needs a pointer receiver).

### Metrics, logging, low-level access

- `c.Metrics().Counter(name, labels...).Increment(n)` and the gauge/histogram variants define and record Envoy metrics.
- `c.Log()` is the request logger.
- `c.StreamInfo()`, `LoadRequestHeaders`, `LoadRequestBody`, `StatusType()`, and `Committed()` live on the unexported `hostContext`. They are runtime internals, not user surface: StreamInfo exposes raw Envoy details and the Load/Status methods drive the filter manager.

## When the old API is still the right choice

If a filter needs `StreamInfo` (route names, filter chain names, dynamic metadata beyond string attributes), write it against the phase API. The new handler surface deliberately does not expose it. Everything else should start on the new API; port old filters when you touch them, using the [Function Reference](function-reference.md) in reverse.
