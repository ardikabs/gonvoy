

# Gonvoy

[![Go Reference](https://pkg.go.dev/badge/github.com/ardikabs/gonvoy.svg)](https://pkg.go.dev/github.com/ardikabs/gonvoy)
[![Go Report Card](https://goreportcard.com/badge/github.com/ardikabs/gonvoy)](https://goreportcard.com/report/github.com/ardikabs/gonvoy)
[![Test](https://github.com/ardikabs/gonvoy/actions/workflows/test.yaml/badge.svg?branch=main)](https://github.com/ardikabs/gonvoy/actions/workflows/test.yaml)
[![Codecov](https://codecov.io/gh/ardikabs/gonvoy/branch/main/graph/badge.svg)](https://codecov.io/gh/ardikabs/gonvoy)

A thin Go framework to write an HTTP Filter extension on Envoy Proxy. It leverages the Envoy [HTTP Golang Filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/golang_filter) as its foundation.

## Features

* Full Go experience for building Envoy HTTP Filter extension.

* Porting `net/http` interface experience to extend Envoy Proxy behavior with [HTTP Golang Filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/golang_filter).

* Logging with [go-logr](https://github.com/go-logr/logr).

* [Stats](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/observability/statistics#arch-overview-statistics) support; Enabling users to generate their own custom metrics.

* Panic-free support; If a panic does occur, it is ensured that it won't break the user experience, particularly the Envoy proxy processes, as it will be handled in a graceful manner by returning a configurable response, defaults to `500`.

### Compatibility Matrix

| Gonvoy | Envoy Proxy |
|-----------------|-----------------|
| v0.3 | v1.29 |
| v0.4 | v1.30 |
| [latest](https://github.com/ardikabs/gonvoy) | v1.40 |

## Installation

```bash
go get github.com/ardikabs/gonvoy
```

## Quickstart

One handler, plain Go shape. Copy `example/helloworld` and change the function:

```go
package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle("hello", gonvoy.Handler(func(c gonvoy.Context) error {
		c.Response().Header.Set("x-hello", "world")
		return nil
	}))
}

func main() {}
```

For layered handling, wrap it: `next(c)` proceeds toward upstream, and
code after it runs when the upstream response arrives:

```go
func Auth(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.Handler(func(c gonvoy.Context) error {
		if c.Request().Header.Get("Authorization") == "" {
			return c.String(http.StatusUnauthorized, "unauthorized")
		}
		return next(c)
	})
}

envoy.Handle("auth", gonvoy.Use(Auth))
```

Need an explicit endpoint without upstream, for example in tests? Use
`gonvoy.Chain(final, Auth)` with a plain `Handler` as `final`.

Build it as a shared library and point Envoy at it:

```bash
go build -o filter.so -buildmode=c-shared *.go
```

Need config, body access, metrics, or routing? See `example/go-simple-extension`
for the same shape with `WithConfig`, `ReadRequestBody`, `ReloadRoute`, and
`MetricPrefix` options. Return `c.String(...)` or `c.JSON(...)` to answer
directly; return `nil` to pass through unanswered, and a panic still becomes
a 500 that keeps Envoy up.

## Development Guide

### Prerequisites

* Go 1.26 or later. Follow [Golang installation guideline](https://golang.org/doc/install).

### Setup

* Install Git.

* Install Go 1.23.

* Clone the project.

    ```bash
    git clone git@github.com:ardikabs/gonvoy.git
    ```

* Create a meaningful branch

    ```bash
    git checkout -b <your-meaningful-branch>
    ```

* Test your changes.

    ```bash
    make test
    ```

* We highly recommend instead of only run test, please also do audit which include formatting, linting, vetting, and testing.

    ```bash
    make audit
    ```

* Add, commit, and push changes to repository

    ```bash
    git add .
    git commit -s -m "<conventional commit style>"
    git push origin <your-meaningful-branch>
    ```

    For writing commit message, please use [conventionalcommits](https://www.conventionalcommits.org/en/v1.0.0/) as a reference.

* Create a Pull Request (PR). In your PR's description, please explain the goal of the PR and its changes.

### Testing

#### Unit Test

```bash
make test
```

### Try It

To try this framework in action, heads to [example](./example) directory.

## License

[MIT](./LICENSE)
