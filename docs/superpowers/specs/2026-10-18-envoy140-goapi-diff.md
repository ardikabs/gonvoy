# Envoy Go API diff: v1.31.2 → v1.40.0 (`contrib/golang/common/go/api`)

Source of truth: module cache copies fetched via
`go mod download -x github.com/envoyproxy/envoy@v1.40.0` in a scratch dir
outside the repo (2026-10-18):

- Old: `/Users/ardika.saputro/go/pkg/mod/github.com/envoyproxy/envoy@v1.31.2/contrib/golang/common/go/api/`
- New: `/Users/ardika.saputro/go/pkg/mod/github.com/envoyproxy/envoy@v1.40.0/contrib/golang/common/go/api/`
- Files diffed: `type.go`, `filter.go`, `capi.go`, `logger.go`, `api.h`.
  `logger.go` is identical. `capi.go`/`api.h` change only in additive C
  entrypoints (plus an internal `ClearRouteCache(r unsafe.Pointer)` →
  `ClearRouteCache(r unsafe.Pointer, refresh bool)` CABI tweak behind the
  Go-level `ClearRouteCache()` / new `RefreshRouteCache()` wrappers).

Verdict column: `present` = unchanged, `changed-signature` = same symbol,
different signature, `dropped` = gone in v1.40.0. Nothing in the inventory
below is dropped.

## Per-symbol table

| Symbol | Verdict | v1.31.2 signature | v1.40.0 signature |
|---|---|---|---|
| `PassThroughStreamFilter` | present (gains methods via embedded `StreamFilter` change; see `StreamFilter` row) | `type PassThroughStreamFilter struct { PassThroughStreamDecoderFilter; PassThroughStreamEncoderFilter }` | same struct; embedded no-op methods updated to match new `StreamFilter` signatures plus `OnStreamComplete()` |
| `StreamFilterFactory` | present | `type StreamFilterFactory func(config interface{}, callbacks FilterCallbackHandler) StreamFilter` | identical |
| `StreamFilter` | **changed-signature** | `OnLog()` / `OnLogDownstreamStart()` / `OnLogDownstreamPeriodic()` / `OnDestroy(DestroyReason)` + `// TODO add more for stream complete` | `OnLog(RequestHeaderMap, RequestTrailerMap, ResponseHeaderMap, ResponseTrailerMap)` / `OnLogDownstreamStart(RequestHeaderMap)` / `OnLogDownstreamPeriodic(RequestHeaderMap, RequestTrailerMap, ResponseHeaderMap, ResponseTrailerMap)` / `OnDestroy(DestroyReason)` / **`OnStreamComplete()`** (new, no args, no return) |
| `StreamFilter.DecodeHeaders` | present | `DecodeHeaders(RequestHeaderMap, bool) StatusType` | identical |
| `StreamFilter.DecodeData` | present | `DecodeData(BufferInstance, bool) StatusType` | identical |
| `StreamFilter.EncodeHeaders` | present | `EncodeHeaders(ResponseHeaderMap, bool) StatusType` | identical |
| `StreamFilter.EncodeData` | present | `EncodeData(BufferInstance, bool) StatusType` | identical |
| `StreamFilter.DecodeTrailers` | present | `DecodeTrailers(RequestTrailerMap) StatusType` | identical |
| `StreamFilter.EncodeTrailers` | present | `EncodeTrailers(ResponseTrailerMap) StatusType` | identical |
| `StreamFilter.OnDestroy` | present | `OnDestroy(DestroyReason)` | identical |
| `BufferInstance` | present | `Append([]byte) error`, `AppendString(string) error`, `Bytes() []byte`, `Drain(int)`, `Len() int`, `Prepend([]byte) error`, `PrependString(string) error`, `Set([]byte) error`, `SetString(string) error`, `String() string` | identical (no diff hunks touch it) |
| `HeaderMap` | present | `Add(string, string)`, `Del(string)`, `Get(string) (string, bool)`, `Range(func(string, string) bool)`, `Set(string, string)`, `Values(string) []string` | identical |
| `RequestHeaderMap` | present | `HeaderMap` + `Method() (string, bool)`, `Path() (string, bool)`, `Host() (string, bool)`, `Scheme() (string, bool)`, protocol helpers | identical |
| `ResponseHeaderMap` | present | `HeaderMap` + `Status() (string, bool)` | identical |
| `RequestTrailerMap` | present | `HeaderMap` | identical |
| `ResponseTrailerMap` | present | `HeaderMap` | identical |
| `StatusType` (`Continue`/`StopAndBuffer`/`StopNoBuffer`/`LocalReply`) | present | `LocalReply StatusType = 1`, `Continue StatusType = 2`, `StopAndBuffer StatusType = 3`, `StopNoBuffer StatusType = 5` (plus `StopAndBufferWatermark = 4`, `HeaderContinue = 100`, `DataContinue = 200`, `TrailerContinue = 300` families) | identical |
| `LogType` (`Debug`/`Info`/`Error`) | present | `Debug LogType = 1`, `Info LogType = 2`, `Warn LogType = 3`, `Error LogType = 4` | identical |
| `DestroyReason` | present | `type DestroyReason int` + `Normal DestroyReason = 0`, `Terminate DestroyReason = 1` | identical |
| `FilterCallbackHandler` | present (embedded interfaces grow; see rows below) | `StreamFilterCallbacks` + `DecoderFilterCallbacks() DecoderFilterCallbacks` + `EncoderFilterCallbacks() EncoderFilterCallbacks` | identical declaration; newly satisfied set grows via `StreamFilterCallbacks.RefreshRouteCache()`, `StreamFilterCallbacks.SecretManager()`, `FilterProcessCallbacks.AddData/InjectData`, `DecoderFilterCallbacks.SetUpstreamOverrideHost` |
| `FilterCallbackHandler.Log` | present | `Log(level LogType, msg string)` (via `StreamFilterCallbacks`) | identical |
| Decoder callbacks (`DecoderFilterCallbacks()`) | present (grows) | `FilterProcessCallbacks` only | + `SetUpstreamOverrideHost(host string, strict bool) error` |
| Encoder callbacks (`EncoderFilterCallbacks()`) | present (grows) | `FilterProcessCallbacks` only | `FilterProcessCallbacks` now also includes `AddData(data []byte, isStreaming bool)` and `InjectData(data []byte)` |
| `ConfigCallbackHandler` | present | `ConfigCallbacks` (`DefineCounterMetric(name string) CounterMetric`, `DefineGaugeMetric(name string) GaugeMetric`) | identical |
| `StreamFilterConfigParser.Parse` | present (doc-comment only) | `Parse(any *anypb.Any, callbacks ConfigCallbackHandler) (interface{}, error)` | identical signature; comment adds "You can return a config implementing the Config interface if you need fine control over its lifecycle." |
| `StreamFilterConfigParser.Merge` | present (doc-comment only) | `Merge(parentConfig interface{}, childConfig interface{}) interface{}` | identical signature; same `Config`-interface lifecycle comment added |
| `StreamInfo` | present (grows) | 16 methods: `GetRouteName() string`, `FilterChainName() string`, `Protocol() (string, bool)`, `ResponseCode() (uint32, bool)`, `ResponseCodeDetails() (string, bool)`, `AttemptCount() uint32`, `DynamicMetadata() DynamicMetadata`, `DownstreamLocalAddress() string`, `DownstreamRemoteAddress() string`, `UpstreamLocalAddress() (string, bool)`, `UpstreamRemoteAddress() (string, bool)`, `UpstreamClusterName() (string, bool)`, `FilterState() FilterState`, `VirtualClusterName() (string, bool)`, `WorkerID() uint32`, `GetProperty`-via-comment | + `DrainConnectionUponCompletion()` and + `DownstreamSslConnection() SslConnection` (new `SslConnection` interface mirroring `envoy/ssl/connection.h`); all 16 existing methods identical |
| `FilterProcessCallbacks` | present (grows) | `Continue(StatusType)`, `SendLocalReply(responseCode int, bodyText string, headers map[string][]string, grpcStatus int64, details string)`, `RecoverPanic()` | + `AddData(data []byte, isStreaming bool)` and + `InjectData(data []byte)` |
| `CounterMetric` | present | `Increment(uint64)`, `Add(uint64)` (via shared metric iface) | identical |
| `GaugeMetric` | present | `Increment(uint64)`, `Decrement(uint64)`, `Add(uint64)`, `Sub(uint64)`, `Set(uint64)` | identical |

### Extra `api.*` symbols used by the repo (outside the brief inventory, mapped for the Step-3 self-check)

| Symbol | Verdict | Notes |
|---|---|---|
| `api.ConfigCallbacks` (`config.go`) | present | identical |
| `api.DecoderFilterCallbacks` / `api.EncoderFilterCallbacks` (`httpfilter_manager.go`) | present (grow, see table) | accessor usage `cb.DecoderFilterCallbacks()` / `cb.EncoderFilterCallbacks()` unchanged |
| `api.HistogramMetric` (`config.go`, `metrics.go`) | present | still `type HistogramMetric interface {}` (empty, TODO) in both versions |
| `api.ErrValueNotFound` (`util.go` presumably) | present | still defined; joined by new `ErrInvalidIPAddress` and unchanged `ErrSerializationFailure`/`ErrInternalFailure` |
| `api.Info`, `api.Debug`, `api.Error` (log-level call sites) | present | `LogType` consts unchanged |

### New v1.40.0-only symbols (not used by repo; informational, no action)

- `Config` interface (`Destroy()` lifecycle hook for parser-returned configs).
- `SslConnection` interface + `StreamInfo.DownstreamSslConnection()`.
- `SecretManager` interface + `StreamFilterCallbacks.SecretManager()`.
- `StreamInfo.DrainConnectionUponCompletion()`; `StreamFilterCallbacks.RefreshRouteCache()`.
- `HttpTcpBridge`, `HttpTcpBridgeFactory`, `HttpTcpBridgeConfigParser`,
  `HttpTcpBridgeCallbackHandler`, `PassThroughHttpTcpBridge`,
  `HttpTcpBridgeInfoType`, `EndStreamType`, `HttpTcpBridgeStatus` (TCP-bridge extension; unrelated to HTTP filter path).
- `ErrInvalidIPAddress`.

## Changed/dropped list (exact fix list for Task 5)

Only one breaking change; nothing is dropped:

1. **`api.StreamFilter` log-phase signatures changed + `OnStreamComplete()` added.**
   Repo impact: `httpfilter_impl.go` declares `OnLog()`,
   `OnLogDownstreamStart()`, `OnLogDownstreamPeriodic()` (zero-arg) and does
   **not** implement `OnStreamComplete()`, so `var _ api.StreamFilter =
   &httpFilterImpl{}` will fail to compile against v1.40.0 until the three
   signatures are updated and a no-op `OnStreamComplete()` is added.
   `httpfilter.go`'s `NoOpHttpFilter = &api.PassThroughStreamFilter{}`
   keeps compiling (upstream no-op methods were updated in place).
2. **Mock regen required (non-breaking for non-test build, breaks `go vet`/`go test`):**
   `test/mock/envoy/FilterCallbackHandler.go` (generated by mockery from
   v1.31.2) lacks `RefreshRouteCache()`, `SecretManager()`,
   `AddData([]byte, bool)`, `InjectData([]byte)`,
   `SetUpstreamOverrideHost(string, bool) error`; regen via `.mockery.yaml`
   after the dep bump. Other mocked interfaces (`BufferInstance`,
   `RequestHeaderMap`, `ResponseHeaderMap`, `CounterMetric`, `GaugeMetric`,
   `ConfigCallbackHandler`) are unchanged and need no regen.

## E2E-affecting behavior notes (for Task 6)

- **Golang filter `v3alpha.Config` status: unchanged.** v1.40.0 still serves
  `envoy.extensions.filters.http.golang.v3alpha.Config`
  (`api/contrib/envoy/extensions/filters/http/golang/v3alpha/golang.proto`,
  `package_version_status = ACTIVE`). All e2e `envoy.yaml` files using
  `"@type": type.googleapis.com/envoy.extensions.filters.http.golang.v3alpha.Config`
  remain valid; no config-type migration needed.
- **`GODEBUG=cgocheck=0` still required.** The cgo bridge design is unchanged
  in v1.40.0 (`contrib/golang/router/cluster_specifier/test/BUILD` still sets
  `env = {"GODEBUG": "cgocheck=0"}`); Go filters running under Envoy still
  pass Go pointers through cgo, so the e2e/build environment must keep
  `GODEBUG=cgocheck=0`.
- **xds/protobuf wire notes:** repo config parsing (`configparser.go`) goes
  through generic `anypb.Any` + `cncf/xds` `TypedStruct`, not the golang
  `v3alpha.Config` Go type, so no wire-level break. For reference, Envoy
  v1.40.0's own `go.mod` uses `google.golang.org/protobuf v1.36.11`,
  `github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2`, and
  `github.com/envoyproxy/go-control-plane/envoy v1.37.0` — the dep bump in
  Task 3 should align `protobuf`/`cncf/xds` forward, not pin to the repo's
  current (`protobuf v1.34.2`, `cncf/xds v0.0.0-20240905190251-b4127c9b8d78`).
- **New log-phase args are e2e-observable:** `OnLog` now receives request /
  response headers+trailers and `OnLogDownstreamStart` receives request
  headers; if any e2e filter asserts log-phase behavior, it can now use the
  maps directly instead of cached state. `OnStreamComplete()` is a new
  stream-end hook with no current repo usage.
- **E2E image pin ( informational for Task 4):** suite default is
  `envoyproxy/envoy:contrib-v1.31-latest` (`pkg/suite/suite.go:16`); the
  v1.40.0 counterpart is `contrib-v1.40-latest`. No e2e yaml schema change
  required beyond the image pin.
