# Filter Authoring Guide

This guide covers everything a handler can do, one topic at a time. Each section states the API, shows the smallest working example, and names the rule that applies. For a flat need-by-need lookup (including the old phase-based equivalents), see the [Function Reference](function-reference.md).

## Single handler vs middleware

A filter is either one self-contained function or a chain of layers. Choose before you write:

- **Single Echo function**, registered with `envoy.HandleEcho`. One job: check something, maybe answer, otherwise let the request through. To proceed upstream *and* run code on the response, call `c.Next()` and keep working after it returns.
- **`gonvoy.Use(...)`**, for layered jobs. Each layer is a `func(gonvoy.Handler) gonvoy.Handler` returning a `gonvoy.Echo` handler. Layers proceed with `c.Next()` and never name `Terminal`: `Use` wires one in as the innermost element. Order is outermost first.

```go
// Single handler: Next sends upstream, after-code sees the response.
envoy.HandleEcho("guard", func(c gonvoy.EchoContext) error {
    if c.Header("Authorization") == "" {
        return c.String(http.StatusUnauthorized, "unauthorized")
    }
    if err := c.Next(); err != nil {
        return err
    }
    c.SetHeader("via", "my-gateway")
    return nil
})

// Use: each layer returns an Echo handler, no Terminal in sight.
envoy.Handle("guard", gonvoy.Use(Auth, Log))
```

The two shapes compose: `Use` returns an ordinary `Handler` — the same underlying runtime `gonvoy.Echo` targets — so a chain can sit anywhere a handler fits. `Handler`/`Use`/`Terminal` are that runtime, not a second framework.

## Keep `pkg/envoy` in `main` only

> ❗ **Unit tests must never import `pkg/envoy`.** The package links Envoy's C bindings, which only exist inside a running Envoy process. A test binary that imports it dies with a segmentation fault on startup, before any test runs. Keep registration in `main`; test everything else as plain Go.

`envoy.Handle` (and its `envoy.HandleEcho` wrapper) is the only call that touches Envoy's framework libraries. The rule is simple:

- **`main` (or any package with no unit tests):** registration only. `init` calls `envoy.Handle` (or `envoy.HandleEcho`), nothing else. It is proven by the end-to-end suite, never by `go test`.
- **Everywhere else:** Echo handlers, `gonvoy.Use` chains, options, and reply helpers. These are plain Go in the root `gonvoy` package and never import `pkg/envoy`, so they unit-test normally.

If a filter grows past a few handlers, split it: handler code goes in a library package that imports only root `gonvoy` (fully unit-testable), and `main.go` stays a thin registration wrapper. See [Testing Filters](testing.md) for the full split.

## `gonvoy.Terminal` is an advanced tool

Most Echo handlers never name `Terminal`: `c.Next()` sends upstream through it for you. You need `Terminal` directly only when you work at the underlying `Handler` level:

1. **A raw single handler that observes the response.** Bare `return` passes through cheaply and never resumes. `Terminal` proceeds upstream *and parks your handler* until the response arrives, so code after it runs. That parking has a cost (the worker waits with a timeout backstop), which is why pass-through stays the default and observation stays opt-in.
2. **A chain you assemble yourself.** If you compose raw handlers by hand instead of `gonvoy.Use` (conditional chains, per-route assembly), `Terminal` is the innermost element that means "send upstream". `Use` is this pattern prebuilt.

Why it is exposed instead of hidden: the `Handler` interface keeps the `net/http` shape (`ServeHTTP(w, r)`, no `next` parameter), so the chain end needs a name users can invoke. `Terminal` is stateless, it resolves the live request from context, calling it twice is a no-op, and calling it outside a request is a no-op. `c.Next()` is the Echo spelling of the same call, and it reports the write-then-`Next` misuse below as an error instead of proceeding silently.

Constraints to respect when you use it directly:

- Pass through the writer you received. Bytes written into a different wrapper before the call are not captured; write into `w` after the call returns instead.
- Do not write a response and then proceed. That is the write-and-next bug: the bridge answers 500. In Echo form, `c.Next()` after writing returns an error — return it instead of proceeding. Pick one.

## Composing with `Use`

Wrap handlers around each other with `gonvoy.Use`. It takes layer functions, outermost first, and returns a single handler you pass to `envoy.Handle` (see the `Auth`/`Log` example in [Getting Started](../getting-started.md#2-write-the-filter)).

`c.Next()` sends the request upstream. Code after that call runs when the upstream response arrives, so this pattern reads and modifies the response:

```go
if err := c.Next(); err != nil {
    return err
}
c.SetHeader("via", "my-gateway")
return nil
```

Rule: a handler that writes a response **and** calls `Next` is a bug. The bridge answers 500. Pick one.

## Reading and replacing the request body

Body access is off by default. Opt in at registration. The example is a single handler, so it proceeds with `c.Next()` (the same call proceeds inside `Use` layers):

```go
envoy.HandleEcho("inspect",
    func(c gonvoy.EchoContext) error {
        body, err := io.ReadAll(c.Body())
        if err != nil {
            return c.String(http.StatusInternalServerError, err.Error())
        }
        c.Log().Info("request body", "size", len(body))
        return c.Next()
    },
    gonvoy.ReadRequestBody(),
)
```

- `gonvoy.ReadRequestBody()`: `c.Body()` is readable. Reading it never disturbs the stream.
- `gonvoy.WriteRequestBody()`: you may also replace it. Assign a new body through the wrapped request **before** calling `Next`, and the upstream receives your bytes:

```go
c.Request().Body = io.NopCloser(bytes.NewReader(modified))
return c.Next()
```

Replacing the body converts the request to chunked encoding (the stale `Content-Length` is dropped). Pass `gonvoy.KeepContentLength(...)` only if your upstream requires the header to stay.

Under the default gate, body access also needs the client to send `X-Content-Operation: ReadOnly` (or `RO`) for reads, `ReadWrite` (or `RW`) for writes. If an intermediary strips that header, use `gonvoy.OpenBodyAccess()` to drop the gate. Prefer the gate when you can; it keeps body reads explicit.

Rule: reading the body without an option fails with a "not accessible" error. Writing (replacing) without a write option returns 502. These errors protect you from touching bodies you did not ask for.

## Reading and replacing the response body

After `Next` returns, the upstream response is available. Headers are always visible via `c.SetHeader`; the body needs an option:

```go
envoy.HandleEcho("modify",
    func(c gonvoy.EchoContext) error {
        if err := c.Next(); err != nil {
            return err
        }

        resp, err := c.ResponseBody()
        if err != nil {
            return nil // response body not enabled; leave the response alone
        }
        raw, err := io.ReadAll(resp)
        if err != nil {
            return nil
        }
        // ... change raw into changed, then answer with the upstream status:
        code := c.Status()
        if code == 0 {
            code = http.StatusOK
        }
        return c.String(code, string(changed))
    },
    gonvoy.ReadResponseBody(),    // to read
    gonvoy.WriteResponseBody(),   // to replace (implies read)
)
```

How the bridge treats your after-`Next` writes:

- No writes: the upstream response passes through with your header changes applied.
- Body-only write (no explicit status): replaces the buffered upstream body.
- Explicit status (for example `c.String(http.StatusOK, ...)`): wins a full local reply with your status, headers, and body.

Rule: replacing the response body needs `WriteResponseBody()`, otherwise you get a 502. Writing before `Next` and then calling `Next` is the write-and-next bug from above: 500.

## Plugin config

Attach typed config at registration and read it per request:

```go
type Config struct {
    AllowList []string `json:"allow_list,omitempty" envoy:"mergeable"`
}

envoy.HandleEcho("guard",
    func(c gonvoy.EchoContext) error {
        cfg, err := gonvoy.EchoConfig[*Config](c)
        if err != nil {
            return nil // no config for this route: pass through
        }
        _ = cfg
        return c.Next()
    },
    gonvoy.WithConfig(new(Config)),
)
```

Route config merges into listener config field by field. Fields tagged `envoy:"mergeable"` take part; `envoy:"preserve_root"` keeps the listener value when the route value is empty. Use `gonvoy.RouteConfigWins()` to let the route replace the listener config wholesale, and `gonvoy.OnConfigError(gonvoy.UseListener)` to fall back to the listener config when a merge fails (the default stops the plugin).

## Changing the route

Mutate the request, then ask Envoy to route again. This needs the registration option plus the per-request call:

```go
envoy.HandleEcho("reroute",
    func(c gonvoy.EchoContext) error {
        if c.Header("x-route-to") == "staticreply" {
            c.SetPath("/staticreply")
            c.Reroute()
        }
        return c.Next()
    },
    gonvoy.ReloadRoute(),
)
```

`c.SetHost`, `c.SetMethod`, and `c.SetPath` changes all take part in the new routing decision, along with header changes. Without `gonvoy.ReloadRoute()`, the mutations still apply to the request but Envoy keeps the original route.

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `c.Attr`. Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields, use `c.Method()`, `c.Path()`, `c.Host()`, and `c.Header(...)` instead.

```go
upstream, _ := c.Attr("xds.cluster_name", "-")
code, _ := c.Attr("response.code", "-")
```

Outside the bridge (plain unit tests), `Attr` returns the default you passed. That keeps handlers testable without Envoy.

## Metrics, logging, shared state

```go
c.Stats().Counter("requests_total", "host", host).Increment(1)
c.Stats().Gauge("inflight").Set(2)

c.Log().Info("handled request", "path", c.Path())

cache := c.Cache() // filter-wide, for expensive setup
```

Metric calls queue up and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy. `c.Log()` discards outside the bridge for the same reason. Set a prefix with `gonvoy.MetricPrefix("myfilter_")`.

## Replies

- Direct answer: `return c.JSON(code, body)` for JSON, `return c.String(code, text)` for plain text, `return c.NoContent(code)` for a status with no body. Returning nil without writing passes the response through.
- Upstream error pages: after `Next`, inspect `c.Status()` and overwrite the response the same way.
- A panic anywhere becomes a 500. Envoy stays up. Prefer returning an explicit reply over panicking.
