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
    envoy.Handle("hello", gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        w.Header().Set("x-hello", "world")
    }))
}

func main() {}
```

The handler above does the simplest useful thing: it tags every response with a header and lets the request continue. Returning without writing means "pass through". This shape, one `HandlerFunc` registered directly, covers any self-contained job: check something, maybe answer, otherwise let the request through.

A handler that answers directly looks like this:

```go
envoy.Handle("auth", gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
    if r.Header.Get("Authorization") == "" {
        gonvoy.String(w, http.StatusUnauthorized, "unauthorized")
        return
    }
    gonvoy.Terminal{}.ServeHTTP(w, r)
}))
```

`gonvoy.Terminal` is the end of the chain. Calling it sends the request upstream, and code after it runs when the response arrives. You need it here because a single handler has no `next` of its own. (There is more to say about when `Terminal` is required and when it is not; see [Single handler vs middleware](user-guide/filter-authoring.md#single-handler-vs-middleware). For now: one job, one function, `Terminal` to proceed.)

When the job has layers, reach for `gonvoy.Use` instead of a bigger single function:

```go
func Auth(next gonvoy.Handler) gonvoy.Handler {
    return gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") == "" {
            gonvoy.String(w, http.StatusUnauthorized, "unauthorized")
            return
        }
        next.ServeHTTP(w, r)
    })
}

func Log(next gonvoy.Handler) gonvoy.Handler {
    return gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
        gonvoy.Log(r).Info("incoming request", "path", r.URL.Path)
        next.ServeHTTP(w, r)
        w.Header().Set("via", "my-gateway")
    })
}

envoy.Handle("auth", gonvoy.Use(Auth, Log))
```

Each layer receives its own `next`, so layers never name `Terminal`. `Use` wires one in as the innermost `next` for you. Order is outermost first: `Auth` runs before `Log` on the way in, and after it on the way out.

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
