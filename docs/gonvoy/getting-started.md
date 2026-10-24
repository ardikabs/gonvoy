# Getting Started

This page takes you from an empty directory to a filter running inside Envoy. It assumes you have Go 1.26 and Docker installed.

## 1. Create the module

```bash
mkdir myfilter && cd myfilter
go mod init myfilter
```

gonvoy is a normal Go dependency. Point at it with a `replace` while you develop, or require the released version:

```go
// go.mod
module myfilter

go 1.26.5

require github.com/ardikabs/gonvoy v0.3.5

replace github.com/ardikabs/gonvoy => /path/to/gonvoy
```

## 2. Write the filter

Create `main.go`. Registration happens in `init`, so Envoy picks it up when the library loads:

```go
package main

import (
    "net/http"

    "github.com/ardikabs/gonvoy"
    "github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
    envoy.HandleEcho("hello", func(c gonvoy.EchoContext) error {
        c.SetHeader("x-hello", "world")
        return nil
    })
}

func main() {}
```

The handler above does the simplest useful thing: it tags every response with a header and lets the request continue. Returning nil without writing means "pass through". This shape, one Echo function registered directly with `envoy.HandleEcho`, covers any self-contained job: check something, maybe answer, otherwise let the request through.

A handler that answers directly looks like this:

```go
envoy.HandleEcho("auth", func(c gonvoy.EchoContext) error {
    if c.Header("Authorization") == "" {
        return c.String(http.StatusUnauthorized, "unauthorized")
    }
    return c.Next()
})
```

`c.Next()` sends the request upstream, and code after it runs when the response arrives. (There is more to say about single handlers versus layers; see [Single handler vs middleware](user-guide/filter-authoring.md#single-handler-vs-middleware). For now: one job, one function, `Next` to proceed.)

When the job has layers, reach for `gonvoy.Use` instead of a bigger single function:

```go
func Auth(gonvoy.Handler) gonvoy.Handler {
    return gonvoy.Echo(func(c gonvoy.EchoContext) error {
        if c.Header("Authorization") == "" {
            return c.String(http.StatusUnauthorized, "unauthorized")
        }
        return c.Next()
    })
}

func Log(gonvoy.Handler) gonvoy.Handler {
    return gonvoy.Echo(func(c gonvoy.EchoContext) error {
        c.Log().Info("incoming request", "path", c.Path())
        if err := c.Next(); err != nil {
            return err
        }
        c.SetHeader("via", "my-gateway")
        return nil
    })
}

envoy.Handle("auth", gonvoy.Use(Auth, Log))
```

Each layer ignores its `next` argument and proceeds with `c.Next()` instead, so layers never name `Terminal`. `Use` wires one in as the innermost element for you. Order is outermost first: `Auth` runs before `Log` on the way in, and after it on the way out.

`envoy.HandleEcho` is a one-line wrapper around `envoy.Handle`: `gonvoy.Echo` adapts the Echo function to the underlying `Handler` runtime, and `Handler`/`Use`/`Terminal` are that runtime, not a second framework.

## 3. Build the shared library

Envoy loads Go filters as `.so` files:

```bash
go build -o filter.so -buildmode=c-shared *.go
```

## 4. Point Envoy at it

Add the filter to the HTTP filter chain in your Envoy config. `plugin_name` must match the name you passed to `envoy.Handle`:

```yaml
http_filters:
  - name: envoy.filters.http.golang
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.golang.v3alpha.Config
      library_id: hello
      library_path: "/filter.so"
      plugin_name: hello
  - name: envoy.filters.http.router
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
```

Start Envoy with the config, send a request, and check for the `x-hello: world` header in the response.

## What to read next

- [Filter Authoring Guide](user-guide/filter-authoring.md): middleware, request and response bodies, plugin config, routing changes, metrics.
- [Testing Filters](user-guide/testing.md): unit-test your handler as plain Go, then prove it against real Envoy.
