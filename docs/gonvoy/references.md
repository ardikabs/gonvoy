# References

## What gonvoy stands on

- **Envoy**, the proxy that runs your filter. Docs: [Envoy Proxy](https://www.envoyproxy.io/docs/envoy/latest/). gonvoy currently targets Envoy v1.40 (images on the `contrib-v1.40` line, builders on `golang:1.26.5-bookworm`). The HTTP connection manager, the Golang HTTP filter entries in `envoy.yaml`, routing, and the admin interface (`/server_info`, `/listeners`, stats) all behave as documented there.
- **Envoy's Go HTTP filter support**, the `contrib/golang` tree in the Envoy repo: [Envoy Go Filter](https://github.com/envoyproxy/envoy/tree/main/contrib/golang). This is the framework gonvoy bridges: `StreamDecoderFilter` / `StreamEncoderFilter` callbacks, `Continue` / `StopAndBuffer` / `StopNoBuffer` / `LocalReply` statuses, header maps, buffer instances, and `SendLocalReply`. gonvoy's `pkg/envoy` is the only package that imports these C bindings, which is why unit tests never import it (native Go test binaries crash on the C init; registration is proven by e2e instead).

## How gonvoy maps to it

| gonvoy | Envoy concept |
|---|---|
| `envoy.Handle` + `plugin_name` | A `envoy.filters.http.golang` chain entry with that plugin name |
| `Terminal` / `next.ServeHTTP` | `Continue` upstream; after-code resumes on `EncodeHeaders`/`EncodeData` |
| Direct write, then return | `SendLocalReply` with your status, headers, body |
| `ReadRequestBody` / `WriteRequestBody` | Decode-side body access; write implies chunked encoding unless kept |
| `ReadResponseBody` / `WriteResponseBody` | Encode-side buffering (`StopAndBuffer`) so the body is present |
| `gonvoy.Reroute(r)` + `gonvoy.ReloadRoute()` | `ClearRouteCache` after your mutations |
| `gonvoy.Stats(r)` | Envoy counter/gauge definitions, flushed on callback threads |
| `WithConfig` + route `typed_per_filter_config` | Listener/route plugin config parsed and merged by `NewConfigParser` |
| `X-Content-Operation: ReadOnly` / `ReadWrite` | The default strict gate for body access (`OpenBodyAccess` drops it) |

## Code to read first

- `handler.go`: the core handler surface (`Handler`, `ResponseWriter`, `Config`, `Attr`, `Reroute`, `Log`, `SharedCache`, `Stats`, `ResponseBody`, `Terminal`).
- `use.go`, `options.go`, `reply.go`: handler composition (`Use`), registration options, and reply helpers (`JSON`, `String`). All pure Go, all unit-testable.
- `pkg/envoy/envoy.go`: `Handle` alone. It is the only registration entrypoint for the new API: it wires your handler and options into Envoy's Go framework, which links Envoy's C bindings. Old-path `Register` / `MustRegister` live in the same file. Nothing else lives here on purpose, so unit tests never need to import this package.
- `bridge.go` / `bridgefilter.go`: the coroutine driver, for when you need to know exactly what happens between your handler and Envoy.
- `test/e2e/filters/`: one working fixture per pattern. `http_body` shows read/replace on both sides, `http_reroute` shows routing, `metrics` shows labels, `panic_filter` shows failure behavior.
