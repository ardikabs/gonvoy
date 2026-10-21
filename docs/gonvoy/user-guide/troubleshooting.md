# Troubleshooting

Symptoms first, cause second, fix third. All of these come from real failures seen while building the bridge.

| Symptom | Cause | Fix |
|---|---|---|
| `request body is not accessible in this phase` when reading `r.Body` | No body option registered, or the strict gate blocked access | Add `gonvoy.ReadRequestBody()` (or the write variant). If the client does not send `X-Content-Operation`, add `gonvoy.OpenBodyAccess()`. Bodiless requests and responses without a `Content-Type` also read as empty, which is normal. |
| `response body is not accessible in this phase` when calling `gonvoy.ResponseBody` | Same as above, response side | Add `gonvoy.ReadResponseBody()` / `gonvoy.WriteResponseBody()`, same gate notes. Also: `ResponseBody` only works after `next`; calling it before is always an error. |
| Client gets 502 on a body write | You replaced a body without the write option | Request replace needs `gonvoy.WriteRequestBody()`; response replace needs `gonvoy.WriteResponseBody()`. Read options alone are not enough. |
| Client gets 500 after your handler runs | Write-and-next: you wrote to `w` and also called `next` | Pick one. Answer directly (write, return) or continue upstream (`next`, then optionally modify the response). Check the log line for the 500. |
| Client gets 500 from a direct reply | Your handler panicked | Panics become 500s and Envoy stays up, but prefer an explicit `http.Error` reply. Read the Envoy log for the panic details. |
| Upstream sees a truncated body after replace | Stale `Content-Length` | The bridge drops `Content-Length` on replace by default (chunked encoding). If you re-added the header or pass `KeepContentLength`, its value must match your new body exactly. |
| Route change ignored (`Reroute` seems to do nothing) | Missing registration option | `gonvoy.Reroute(r)` needs `gonvoy.ReloadRoute()` on the same `Handle` call. `r.Host`, `r.Method`, `r.URL`, and header changes all take part once both are in place. |
| `gonvoy.Config` returns "filter config is not available" | No config registered, or the route carries none | Pass `gonvoy.WithConfig(new(Config))` at registration. A handler that runs on routes without config should treat the error as "pass through", not as fatal. |
| After-`next` code never sees the body | No response-body option | Headers are always visible after `next`; the body needs `gonvoy.ReadResponseBody()`. |
| Unit test hangs after calling `Terminal` | `Terminal` parks until the response arrives | In unit tests there is no upstream, so the handler goroutine waits forever. Either return without calling `Terminal`, or close the test exchange down (see `bridge_test.go` for the pattern). |
| Filter not loaded: `plugin_name` mismatch | Names disagree | The `plugin_name` in `envoy.yaml` must equal the name in `envoy.Handle`. Envoy logs a load failure otherwise. |
| `go build` fails in a fixture | Missing replace directive | Each fixture `go.mod` needs `replace github.com/ardikabs/gonvoy => ../../../../` so it builds against the working tree. |

When none of this fits: run the failing request against the e2e harness and read the Envoy log. The bridge logs every 500 it synthesizes (write-and-next, send failures), and `gonvoy.Log(r)` lines appear tagged with your `plugin_name`.
