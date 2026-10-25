# Filter Authoring Guide

This guide covers everything a handler can do, one topic at a time. Each section states the API, shows the smallest working example, and names the rule that applies. For a flat need-by-need lookup (including the old phase-based equivalents), see the [Function Reference](function-reference.md). For the full registration option list with defaults, see [Registration Options](options.md).

## Single handler vs middleware

A filter is either one self-contained function or a chain of layers. Choose before you write:

- **Single handler function**, registered with `envoy.Handle`. One job: check something, maybe answer, otherwise let the request through. Returning `nil` without writing passes through; no method call is needed.
- **`gonvoy.Use(...)`**, for layered jobs. Each layer is a `gonvoy.Middleware`: `func(next gonvoy.Handler) gonvoy.Handler`. Call `next` to proceed toward upstream; code after it runs when the response arrives. Order is outermost first.

```go
// Single handler: answer or pass through, no chain call needed.
envoy.Handle("guard", gonvoy.Handler(func(c gonvoy.Context) error {
    if c.Request().Header.Get("Authorization") == "" {
        return c.String(http.StatusUnauthorized, "unauthorized")
    }
    return nil
}))

// Use: each layer calls next; the innermost next is the internal upstream hop.
func Auth(next gonvoy.Handler) gonvoy.Handler {
    return gonvoy.Handler(func(c gonvoy.Context) error {
        if c.Request().Header.Get("Authorization") == "" {
            return c.String(http.StatusUnauthorized, "unauthorized")
        }
        return next(c)
    })
}

func Log(next gonvoy.Handler) gonvoy.Handler {
    return gonvoy.Handler(func(c gonvoy.Context) error {
        c.Log().Info("incoming request", "path", c.Path())
        if err := next(c); err != nil {
            return err
        }
        c.Response().Header.Set("via", "my-gateway")
        return nil
    })
}

envoy.Handle("guard", gonvoy.Use(Auth, Log))
```

The two shapes compose: `Use` returns an ordinary `Handler` (the same Context-based contract throughout), so a chain can sit anywhere a handler fits. `Handler`/`Use`/`Middleware` are that runtime, not a second framework.

A `Middleware` wraps the next layer and decides whether that layer runs. Calling `next(c)` continues inward; returning directly skips every inner layer. The terminal lives outside that onion, so skipping skips only logic, never forwarding. `final` runs only on paths where each outer layer calls `next(c)`. With zero middlewares, `final` always runs.

## Why `final` is a separate parameter

`Chain` takes `final Handler` first, then `middlewares ...Middleware`:

```go
gonvoy.Chain(final, Auth, Log)
```

`final` is a plain `Handler`: it takes only `c`, it has no `next` to call, and it cannot forget or double-call it. That type boundary is the point. A `Middleware` must decide to delegate; an endpoint only decides to answer, pass through, or return an error.

Use the split this way:

* Need upstream observation or in-place upstream body work: use `Use`. Its hidden terminal forwards upstream, parks for the response, and resumes after-`next` code.
* Need an explicit endpoint that still keeps the upstream guarantee: use `Chain` with a `final` that focuses on request work. When the chain returns `nil` with no status set, `Chain` forwards through the same hidden terminal, so a custom terminal keeps the same upstream-forwarding behavior by default. A write sends a local reply and an error maps through the error handler, both without forwarding.
* Need a unit-test seam: pass a stub or fake upstream as `final` and assert on status, headers, and body without starting Envoy. Use an answering stub to keep the test off upstream; leave it unanswered to exercise the forward path.

Guarantee: the terminal sits outside the onion as `outermost(inner(innermost(final)))` plus terminal. `Use` and `Chain` share that shape. Skipping an inner layer skips only logic. An unanswered return from any path still forwards upstream, so the filter always continues unless it answered or errored.

## Keep `pkg/envoy` in `main` only

> ❗ **Unit tests must never import `pkg/envoy`.** The package links Envoy's C bindings, which only exist inside a running Envoy process. A test binary that imports it dies with a segmentation fault on startup, before any test runs. Keep registration in `main`; test everything else as plain Go.

`envoy.Handle` is the only call that touches Envoy's framework libraries. The rule is simple:

- **`main` (or any package with no unit tests):** registration only. `init` calls `envoy.Handle`, nothing else. It is proven by the end-to-end suite, never by `go test`.
- **Everywhere else:** handler functions, `gonvoy.Use` chains, options, and reply helpers. These are plain Go in the root `gonvoy` package and never import `pkg/envoy`, so they unit-test normally.

If a filter grows past a few handlers, split it: handler code goes in a library package that imports only root `gonvoy` (fully unit-testable), and `main.go` stays a thin registration wrapper. See [Testing Filters](testing.md) for the full split.

## The cost of observing the response

A bare `return` passes through cheaply and never resumes. Calling `next` inside `Use` proceeds upstream *and parks your handler* until the response arrives, so code after it runs. That parking has a cost (the worker waits with a timeout backstop), which is why pass-through stays the default and observation stays opt-in.

Constraints to respect when you proceed upstream:

- Do not write a response and then proceed. That is the write-and-proceed bug: the bridge answers 500. `next(c)` after writing returns an error, so return it instead of proceeding. Pick one.

## Composing with `Use`

Wrap handlers around each other with `gonvoy.Use`. It takes `Middleware` layers, outermost first, and returns a single handler you pass to `envoy.Handle` (see the `Auth`/`Log` example in [Getting Started](../getting-started.md#2-write-the-filter)).

Calling `next` sends the request upstream. Code after that call runs when the upstream response arrives, so this pattern reads and modifies the response:

```go
func Log(next gonvoy.Handler) gonvoy.Handler {
    return gonvoy.Handler(func(c gonvoy.Context) error {
        if err := next(c); err != nil {
            return err
        }
        c.Response().Header.Set("via", "my-gateway")
        return nil
    })
}
```

Rule: a handler that writes a response **and** calls `next` is a bug. The bridge answers 500. Pick one.

Need an explicit endpoint for tests or direct answers? Use `gonvoy.Chain(final, Auth, Log)`. See why `final` stays separate above.

## Reading and replacing the request body

Body access is off by default. Opt in at registration. The example is a single handler, so it passes through with `return nil` (the same mutation pattern applies inside `Use` layers before calling `next`):

```go
envoy.Handle("inspect",
    gonvoy.Handler(func(c gonvoy.Context) error {
        body, err := io.ReadAll(c.Body())
        if err != nil {
            return c.String(http.StatusInternalServerError, err.Error())
        }
        c.Log().Info("request body", "size", len(body))
        return nil
    }),
    gonvoy.ReadRequestBody(),
)
```

- `gonvoy.ReadRequestBody()`: `c.Body()` is readable. Reading it never disturbs the stream.
- `gonvoy.WriteRequestBody()`: you may also replace it. Assign a new body through the wrapped request **before** calling `next`, and the upstream receives your bytes:

```go
c.Request().Body = io.NopCloser(bytes.NewReader(modified))
return next(c)
```

Replacing the body converts the request to chunked encoding (the stale `Content-Length` is dropped). Pass `gonvoy.KeepContentLength(...)` only if your upstream requires the header to stay.

Under the default gate, body access also needs the client to send `X-Content-Operation: ReadOnly` (or `RO`) for reads, `ReadWrite` (or `RW`) for writes. If an intermediary strips that header, use `gonvoy.OpenBodyAccess()` to drop the gate. Prefer the gate when you can; it keeps body reads explicit.

Rule: reading the body without an option fails with a "not accessible" error. Writing (replacing) without a write option returns 502. These errors protect you from touching bodies you did not ask for. See [Registration Options](options.md) for the full body-access list.

## Reading and replacing the response body

After `next` returns, the upstream response is available. Headers are always visible via `c.Response().Header`; the body needs an option. Observe the response only inside `Use`: a plain `Handler` that returns without calling `next` never parks, so after-code would never run.

```go
envoy.Handle("modify",
    gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
        return gonvoy.Handler(func(c gonvoy.Context) error {
            if err := next(c); err != nil {
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
        })
    }),
    gonvoy.ReadResponseBody(),    // to read
    gonvoy.WriteResponseBody(),   // to replace (implies read)
)
```

How the bridge treats your after-`next` writes:

- No writes: the upstream response passes through with your header changes applied.
- Body-only write (no explicit status): replaces the buffered upstream body.
- Explicit status (for example `c.String(http.StatusOK, ...)`): wins a full local reply with your status, headers, and body.

Rule: replacing the response body needs `WriteResponseBody()`, otherwise you get a 502. Writing before `next` and then calling `next` is the write-and-proceed bug from above: 500. See [Registration Options](options.md) for the full body-access list.

## Plugin config

Attach typed config at registration and read it per request:

```go
type Config struct {
    AllowList []string `json:"allow_list,omitempty" envoy:"mergeable"`
}

envoy.Handle("guard",
    gonvoy.Handler(func(c gonvoy.Context) error {
        cfg, err := gonvoy.ConfigOf[*Config](c)
        if err != nil {
            return nil // no config for this route: pass through
        }
        _ = cfg
        return nil
    }),
    gonvoy.WithConfig(new(Config)),
)
```

Route config merges into listener config field by field. Fields tagged `envoy:"mergeable"` take part; `envoy:"preserve_root"` keeps the listener value when the route value is empty. Use `gonvoy.RouteConfigWins()` to let the route replace the listener config wholesale, and `gonvoy.OnConfigError(gonvoy.UseListener)` to fall back to the listener config when a merge fails (the default stops the plugin). The full option list lives in [Registration Options](options.md).

## Changing the route

Mutate the request, then ask Envoy to route again. This needs the registration option plus the per-request call:

```go
envoy.Handle("reroute",
    gonvoy.Handler(func(c gonvoy.Context) error {
        if c.Request().Header.Get("x-route-to") == "staticreply" {
            c.SetPath("/staticreply")
            c.Reroute()
        }
        return nil
    }),
    gonvoy.ReloadRoute(),
)
```

`c.SetHost`, `c.SetMethod`, and `c.SetPath` changes all take part in the new routing decision, along with header changes. Without `gonvoy.ReloadRoute()`, the mutations still apply to the request but Envoy keeps the original route. See [Registration Options](options.md) for the default.

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `c.Attr`. Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields, use `c.Method()`, `c.Path()`, `c.Host()`, `c.Request().Header.Get(...)`, and `c.Response().Header.Get(...)` instead.

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

Metric calls queue up and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy. `c.Log()` discards outside the bridge for the same reason. Set a prefix with `gonvoy.MetricPrefix("myfilter_")`. See [Registration Options](options.md) for the default (no prefix).

## Replies

- Direct answer: `return c.JSON(code, body)` for JSON, `return c.String(code, text)` for plain text, `return c.NoContent(code)` for a status with no body. Returning nil without writing passes the response through.
- Upstream error pages: inside `Use`, after `next`, inspect `c.Status()` and overwrite the response the same way.
- A panic anywhere becomes a 500. Envoy stays up. Prefer returning an explicit reply over panicking.
