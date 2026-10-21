# Testing Filters

Test at two levels: unit-test the handler as plain Go, then prove it against real Envoy.

## ❗ Never import `pkg/envoy` from unit-tested code

> ❗ **Unit tests must never import `pkg/envoy`.** The package links Envoy's C bindings, which only exist inside a running Envoy process. A test binary that imports it dies with a segmentation fault on startup, before any test runs. Keep registration in `main`; test everything else as plain Go.

This is the one rule that breaks loudly when ignored, so it is worth stating three ways:

1. **What happens.** A `go test` binary has no Envoy host process. `pkg/envoy` links framework code that only exists inside one, so the import kills the test process with a segmentation fault on startup. The fault takes down the whole binary: every test in the package dies, including ones that never touch Envoy. It is not catchable and not retryable.
2. **Where the line is.** `main` (or any package with no unit tests) may import `pkg/envoy`, for registration only. Every package with a `_test.go` file must not import it, directly or transitively. The root `gonvoy` package is safe on purpose: handlers, `Use`, options, and reply helpers live there as plain Go with no C bindings.
3. **How to structure a filter.** Keep `main.go` to registration: `init` plus `envoy.Handle`, nothing else. Put handler code in a library package that imports only root `gonvoy` and unit-test that package freely. If you inherit a fixture with everything inline in `main.go`, that file is e2e-only by construction; extract what you want tested.

If a test binary dies instantly with exit 139 and no test output, check the import graph first. Nine times out of ten, `pkg/envoy` got in through a helper import.

## Unit tests

Handlers are ordinary functions, and every bridge helper degrades gracefully outside Envoy: `Attr` returns your default, `Reroute` does nothing, `Log` discards, `Stats` drops, `SharedCache` returns nil. So most handlers need no mocks at all:

```go
func TestAuth_RejectsAnonymous(t *testing.T) {
    h := Auth(gonvoy.Terminal{})
    r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
    w := ... // any gonvoy.ResponseWriter; see below

    h.ServeHTTP(w, r)

    require.Equal(t, http.StatusUnauthorized, w.Status())
}
```

For the writer, use the same recorder the bridge uses, or a small fake implementing `Header`, `Write`, `WriteHeader`, and `Status`. For `gonvoy.Config`, attach it to the request context the way the bridge does (see `handler_test.go` in the repo root for the pattern). Run the suite with:

```bash
go test ./...
make test   # unit tests with -race and coverage
make lint   # must stay clean
```

Registration (`envoy.Handle`) is proven by the end-to-end suite instead (see the warning above).

## End-to-end tests

The e2e suite boots real Envoy (currently the `contrib-v1.40` image line) with your compiled filter and runs HTTP assertions against it. A fixture is one directory:

```
test/e2e/filters/<name>/
  main.go      # the filter: envoy.Handle(...) in init()
  envoy.yaml   # listener, golang filter entries, routes
  go.mod       # module with: replace github.com/ardikabs/gonvoy => ../../../../
  go.sum
```

`plugin_name` in `envoy.yaml` must match the name passed to `envoy.Handle`. A test case lives in `test/e2e/tests/<name>.go`: build a `suite.TestCase`, register it in the `TestCases` list, and assert on real responses (status, headers, body) plus `kit.CheckEnvoyLog` for log lines.

Commands:

```bash
make e2e          # build filters in Docker, then run the suite
make e2e.go       # run the suite against already-built filters
make e2e.cleanup  # remove build artifacts
```

The existing fixtures under `test/e2e/filters/` (`helloworld`, `http_body`, `http_headers_modifier`, `http_reroute`, `metrics`, `panic_filter`, `custom_err_response`) are working examples of every pattern in the [Filter Authoring Guide](filter-authoring.md). Copy the closest one when you start a new fixture.

Rule for e2e assertions: never edit an assertion to make a behavior change pass. If your implementation cannot satisfy an existing assertion, that is a breaking change. Note it, do not smooth it over.
