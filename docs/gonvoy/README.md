# gonvoy

gonvoy lets you write Envoy HTTP filters in Go using the shape you already know: `net/http` handlers.

If you can write this, you can write a gonvoy filter:

```go
envoy.Handle("hello", gonvoy.Handler(func(c gonvoy.Context) error {
    c.Response().Header.Set("x-hello", "world")
    return nil
}))
```

## How it fits together

A request passes through three layers:

1. **Envoy** receives the HTTP request and hands it to your compiled filter (a `.so` shared library).
2. **The bridge** translates between Envoy's filter callbacks and plain Go values. It runs your code on its own goroutine, so your handler never touches Envoy internals directly.
3. **Your handler** reads the request through `c` and answers with `c.String`/`c.JSON`. Returning `nil` without writing passes the request upstream.

```
client -> Envoy -> bridge -> your handler -> (upstream or direct reply)
```

## Two patterns

Most filters are one of these shapes. Pick one per filter:

- **A single `Handler`** for one self-contained job: check something, maybe answer, otherwise let the request through.
- **`gonvoy.Use(Auth, Log)`** for layered jobs: auth outside, logging inside, each layer calling `next(c)`.

Both end the same way: either you answer directly, or the request goes upstream. The next section is about that decision.

## The three decisions

Every handler answers one question: what happens to this request?

| Decision | What you write | What happens |
|---|---|---|
| Pass through | Return without writing | The request continues upstream unchanged |
| Answer directly | `c.String(...)`/`c.JSON(...)`, then return `nil` | Envoy replies to the client, upstream never sees it |
| Inspect the round trip | Inside `Use`, call `next(c)`, then run code after it | The request goes upstream; your after-code runs when the response arrives |

One rule covers the only invalid combination: do not write an answer **and** call `next`. The bridge replies 500 and logs, instead of guessing which one you meant.

## Where to go next

- New here: start with [Getting Started](getting-started.md). You will have a working filter in a few minutes.
- Writing real filters: read the [Filter Authoring Guide](user-guide/filter-authoring.md). It covers middleware, bodies, config, routing, metrics, and the rules that keep you out of trouble.
- Looking up a specific capability: see the [Function Reference](user-guide/function-reference.md). It lists every handler need side by side with the old phase-based equivalent.
- Choosing registration options: see [Registration Options](user-guide/options.md). It lists every `Handle` option, what it enables, and its default.
- Costing a filter: see [Performance contract](user-guide/performance.md). It states what gonvoy's abstraction costs, what is free, and when raw Go fits better.
- Verifying your work: see [Testing Filters](user-guide/testing.md) for unit tests and the end-to-end suite.
- gRPC traffic: see [gRPC scope](user-guide/grpc-scope.md) before building message or stream logic.
- Something wrong: check [Troubleshooting](user-guide/troubleshooting.md) first. It lists every common failure and its fix.
- Background on the platform: see [References](references.md).
