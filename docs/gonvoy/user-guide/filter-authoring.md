# Filter Authoring Guide

This guide covers everything a handler can do, one topic at a time. Each section states the API, shows the smallest working example, and names the rule that applies. For a flat need-by-need lookup (including the old phase-based equivalents), see the [Function Reference](function-reference.md).

## Single handler vs middleware

A filter is either one self-contained function or a chain of layers. Choose before you write:

- **Single `HandlerFunc`**, registered directly with `envoy.Handle`. One job: check something, maybe answer, otherwise let the request through. There is no `next` in scope, so when the handler must proceed upstream *and* run code on the response, it calls `gonvoy.Terminal{}.ServeHTTP(w, r)` explicitly.
- **`gonvoy.Use(...)`**, for layered jobs. Each layer is a function taking `next` and returning a handler. Layers never name `Terminal`: `Use` wires one in as the innermost `next`. Order is outermost first.

```go
// Single handler: Terminal sends upstream, after-code sees the response.
envoy.Handle("guard", gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
    if r.Header.Get("Authorization") == "" {
        gonvoy.String(w, http.StatusUnauthorized, "unauthorized")
        return
    }
    gonvoy.Terminal{}.ServeHTTP(w, r)
    w.Header().Set("via", "my-gateway")
}))

// Use: each layer gets its own next, no Terminal in sight.
envoy.Handle("guard", gonvoy.Use(Auth, Log))
```

The two shapes compose: `Use` returns an ordinary handler, so a chain can sit anywhere a handler fits.

## Keep `pkg/envoy` in `main` only

> ❗ **Unit tests must never import `pkg/envoy`.** The package links Envoy's C bindings, which only exist inside a running Envoy process. A test binary that imports it dies with a segmentation fault on startup, before any test runs. Keep registration in `main`; test everything else as plain Go.

`envoy.Handle` is the only call that touches Envoy's framework libraries. The rule is simple:

- **`main` (or any package with no unit tests):** registration only. `init` calls `envoy.Handle`, nothing else. It is proven by the end-to-end suite, never by `go test`.
- **Everywhere else:** handlers, `gonvoy.Use` chains, options, and reply helpers. These are plain Go in the root `gonvoy` package and never import `pkg/envoy`, so they unit-test normally.

If a filter grows past a few handlers, split it: handler code goes in a library package that imports only root `gonvoy` (fully unit-testable), and `main.go` stays a thin registration wrapper. See [Testing Filters](testing.md) for the full split.

## `gonvoy.Terminal` is an advanced tool

Most handlers never name `Terminal`. You need it directly in exactly two situations:

1. **A single handler that observes the response.** Bare `return` passes through cheaply and never resumes. `Terminal` proceeds upstream *and parks your handler* until the response arrives, so code after it runs. That parking has a cost (the worker waits with a timeout backstop), which is why pass-through stays the default and observation stays opt-in.
2. **A chain you assemble yourself.** If you compose handlers by hand instead of `gonvoy.Use` (conditional chains, per-route assembly), `Terminal` is the innermost element that means "send upstream". `Use` is this pattern prebuilt.

Why it is exposed instead of hidden: the `Handler` interface keeps the `net/http` shape (`ServeHTTP(w, r)`, no `next` parameter), so the chain end needs a name users can invoke. `Terminal` is stateless, it resolves the live request from context, calling it twice is a no-op, and calling it outside a request is a no-op.

Constraints to respect when you use it directly:

- Pass through the writer you received. Bytes written into a different wrapper before the call are not captured; write into `w` after the call returns instead.
- Do not write a response and then call it. That is the write-and-next bug: the bridge answers 500. Pick one.

## Composing with `Use`

Wrap handlers around each other with `gonvoy.Use`. It takes layer functions, outermost first, and returns a single handler you pass to `envoy.Handle` (see the `Auth`/`Log` example in [Getting Started](../getting-started.md#2-write-the-filter)).

`next.ServeHTTP(w, r)` sends the request upstream. Code after that call runs when the upstream response arrives, so this pattern reads and modifies the response:

```go
next.ServeHTTP(w, r)
w.Header().Set("via", "my-gateway")
```

Rule: a handler that writes a response **and** calls `next` is a bug. The bridge answers 500. Pick one.

## Reading and replacing the request body

Body access is off by default. Opt in at registration. The example is a single handler, so it calls `Terminal` to proceed (inside middleware you would call `next` instead; see [Single handler vs middleware](#single-handler-vs-middleware)):

```go
envoy.Handle("inspect",
    gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        gonvoy.Log(r).Info("request body", "size", len(body))
        gonvoy.Terminal{}.ServeHTTP(w, r)
    }),
    gonvoy.ReadRequestBody(),
)
```

- `gonvoy.ReadRequestBody()`: `r.Body` is readable. Reading it never disturbs the stream.
- `gonvoy.WriteRequestBody()`: you may also replace it. Assign a new `r.Body` **before** calling `next`, and the upstream receives your bytes:

```go
r.Body = io.NopCloser(bytes.NewReader(modified))
gonvoy.Terminal{}.ServeHTTP(w, r)
```

Replacing the body converts the request to chunked encoding (the stale `Content-Length` is dropped). Pass `gonvoy.KeepContentLength(...)` only if your upstream requires the header to stay.

Under the default gate, body access also needs the client to send `X-Content-Operation: ReadOnly` (or `RO`) for reads, `ReadWrite` (or `RW`) for writes. If an intermediary strips that header, use `gonvoy.OpenBodyAccess()` to drop the gate. Prefer the gate when you can; it keeps body reads explicit.

Rule: reading the body without an option fails with a "not accessible" error. Writing (replacing) without a write option returns 502. These errors protect you from touching bodies you did not ask for.

## Reading and replacing the response body

After `next` returns (or `Terminal`, in a single handler), the upstream response is available. Headers are always visible on `w`. The body needs an option:

```go
envoy.Handle("modify",
    gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        gonvoy.Terminal{}.ServeHTTP(w, r)

        resp, err := gonvoy.ResponseBody(r)
        if err != nil {
            return // response body not enabled; leave the response alone
        }
        raw, err := io.ReadAll(resp)
        if err != nil {
            return
        }
        // ... change raw, then:
        _, _ = w.Write(changed)
    }),
    gonvoy.ReadResponseBody(),    // to read
    gonvoy.WriteResponseBody(),   // to replace (implies read)
)
```

How the bridge treats your after-`next` writes:

- No writes: the upstream response passes through with your header changes applied.
- Body-only write (no explicit status): replaces the buffered upstream body.
- Explicit `WriteHeader` status: wins a full local reply with your status, headers, and body.

Rule: replacing the response body needs `WriteResponseBody()`, otherwise you get a 502. Writing into `w` before `next` and then calling `next` is the write-and-next bug from above: 500.

## Plugin config

Attach typed config at registration and read it per request:

```go
type Config struct {
    AllowList []string `json:"allow_list,omitempty" envoy:"mergeable"`
}

envoy.Handle("guard",
    gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        cfg, err := gonvoy.Config[*Config](r)
        if err != nil {
            return // no config for this route: pass through
        }
        _ = cfg
        gonvoy.Terminal{}.ServeHTTP(w, r)
    }),
    gonvoy.WithConfig(new(Config)),
)
```

Route config merges into listener config field by field. Fields tagged `envoy:"mergeable"` take part; `envoy:"preserve_root"` keeps the listener value when the route value is empty. Use `gonvoy.RouteConfigWins()` to let the route replace the listener config wholesale, and `gonvoy.OnConfigError(envoy.UseListener)` to fall back to the listener config when a merge fails (the default stops the plugin).

## Changing the route

Mutate the request, then ask Envoy to route again. This needs the registration option plus the per-request call:

```go
envoy.Handle("reroute",
    gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        if r.Header.Get("x-route-to") == "staticreply" {
            r.URL.Path = "/staticreply"
            gonvoy.Reroute(r)
        }
        gonvoy.Terminal{}.ServeHTTP(w, r)
    }),
    gonvoy.ReloadRoute(),
)
```

`r.Host`, `r.Method`, and `r.URL` changes all take part in the new routing decision, along with header changes. Without `gonvoy.ReloadRoute()`, the mutations still apply to the request but Envoy keeps the original route.

## Envoy attributes

Anything that is not an HTTP field (cluster name, route name, response flags) comes from `gonvoy.Attr`. Values are snapshotted at each filter phase, so what you read is current as of the last phase boundary. For HTTP fields, use `r.Host`, `r.Method`, and `r.URL` instead.

```go
upstream, _ := gonvoy.Attr(r, "xds.cluster_name", "-")
code, _ := gonvoy.Attr(r, "response.code", "-")
```

Outside the bridge (plain unit tests), `Attr` returns the default you passed. That keeps handlers testable without Envoy.

## Metrics, logging, shared state

```go
gonvoy.Stats(r).Counter("requests_total", "host", host).Increment(1)
gonvoy.Stats(r).Gauge("inflight").Set(2)

gonvoy.Log(r).Info("handled request", "path", r.URL.Path)

cache := gonvoy.SharedCache(r) // filter-wide, for expensive setup
```

Metric calls queue up and flush on Envoy callback threads, in order. Outside the bridge they are dropped, so unit tests never touch Envoy. `gonvoy.Log(r)` discards outside the bridge for the same reason. Set a prefix with `gonvoy.MetricPrefix("myfilter_")`.

## Replies

- Direct answer: `gonvoy.JSON(w, code, body)` for JSON, `gonvoy.String(w, code, text)` for plain text, then return without calling `next`. Raw `w.WriteHeader(code)` plus `w.Write(body)` covers any other payload.
- Upstream error pages: after `next`, inspect `w.Status()` and overwrite the response the same way.
- A panic anywhere becomes a 500. Envoy stays up. Prefer returning an explicit reply over panicking.
