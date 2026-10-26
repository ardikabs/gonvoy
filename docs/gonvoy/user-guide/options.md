# Registration Options

All options are passed to `envoy.Handle` as trailing arguments. Each option sets one behavior. Omit what you do not need; defaults stay on the safe side.

```go
envoy.Handle("guard", handler,
    gonvoy.WithConfig(new(Config)),
    gonvoy.ReadRequestBody(),
)
```

Body access has three independent dimensions, grouped as the body policy: the access rule per direction, the gate, and `Content-Length` handling.

### Access rule per direction

Each direction, request and response, has its own rule with three levels: no access (the default), read, or read plus write. Write implies read.

| Option | Direction | Level |
|---|---|---|
| (none) | Request, response | No access |
| `ReadRequestBody()` | Request | Read: `c.RequestBody()`, `c.Bind` work |
| `WriteRequestBody()` | Request | Read and write: you may also replace `c.Request().Body` |
| `ReadResponseBody()` | Response | Read: after `next` returns, `c.ResponseBody()` works |
| `WriteResponseBody()` | Response | Read and write: a body-only `c.Write` replaces the buffered upstream body |

Without the matching level, reads fail as not accessible and replacements fail with 502. A write option always includes its read side, so `WriteRequestBody` alone covers a read-then-replace flow.

### Gate: when access applies

Even with a read or write level, the default gate still checks the client header: `X-Content-Operation: ReadOnly` (or `RO`) for reads, `ReadWrite` (or `RW`) for writes. Requests without it read as empty or fail, depending on the call.

| Option | Intent |
|---|---|
| `RequireBodyPermissionHeader()` | Keep the strict gate; pass it to be explicit. This is the default. |
| `OpenBodyAccess()` | Drop the gate; use it only when an intermediary strips the header. Prefer the gate when you can; it keeps body reads explicit. |

### Length: what happens to Content-Length on writes

Replacing a body drops the stale `Content-Length` by default and uses chunked encoding.

| Option | Intent |
|---|---|
| (none) | Drop `Content-Length` on writes. This is the default. |
| `KeepContentLength(KeepLengthRequest)` | Preserve it on request body writes |
| `KeepContentLength(KeepLengthResponse)` | Preserve it on response body writes |
| `KeepContentLength(KeepLengthBoth)` | Preserve it on both |

Only pass `KeepContentLength` when the upstream requires the header to stay, with a value that matches the new body exactly. Otherwise a truncated body reaches upstream.

## Routing

| Option | Intent |
|---|---|
| `ReloadRoute()` | Re-evaluate the route when the handler changes host, method, path, or headers |

Pair it with the per-request `Reroute()` call, which flags the reload. Without the option, mutations still apply to the request, but Envoy keeps the original route.

## Envoy attributes and TLS

`c.Attr` serves request host/method/path and the response code for free. Anything else costs an Envoy fetch per request, so it is off unless declared:

| Option | Intent |
|---|---|
| `EnableEnvoyAttributes(names...)` | Prefetch exactly these attribute names (for example `"xds.cluster_name"`) for `Attr` lookups. Undeclared keys report absent with a one-time migration warning naming the key |
| `EnableDownstreamTLS()` | Snapshot the downstream TLS connection per request for `TLS()` lookups. Without it `TLS()` reports absent (with a one-time warning when a connection is present). Plaintext listeners pay nothing either way |

Omit both when the filter never calls `Attr` beyond request fields and never calls `TLS()` — that is the cheapest shape and the default.

## Plugin config

| Option | Intent |
|---|---|
| `WithConfig(...)` | Attach the typed plugin config, read back per request with `ConfigOf` |
| `RouteConfigWins()` | Let the route config (child) replace the listener config (parent/root) instead of merging field by field |
| `OnConfigError(...)` | Decide merge-failure handling for struct configs: `Stop` (the default) stops the plugin, `UseListener` falls back to the listener config |

Treat a missing config as pass through, not as fatal. Route config merges into listener config field by field; only fields tagged `envoy:"mergeable"` take part, and `envoy:"preserve_root"` keeps the listener value when the route value is empty.

## Observability

| Option | Intent |
|---|---|
| `MetricPrefix(...)` | Prefix every metric this filter emits, so dashboards can tell filters apart |

Metric calls queue on the handler goroutine and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy.
