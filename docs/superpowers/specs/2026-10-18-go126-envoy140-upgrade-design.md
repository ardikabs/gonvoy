# Go 1.26.5 + Envoy v1.40.0 Upgrade — Design

## Intent and Why
Gonvoy is pinned to Go 1.23.1 and Envoy v1.31.2 (Oct 2024 era) while the ecosystem has moved on: Go 1.26.5 is the current secure patch (Jul 2026, CVE-2026-39822 / CVE-2026-42505 fixed) and Envoy v1.40.0 is released (Oct 2026). Envoy's own docs require the Go plugin SDK version to match the Envoy binary version, so every stale pin — Go toolchain, Go modules, Envoy SDK, Envoy container images, builder images — must move together. The purpose is a pure toolchain-and-dependency refresh that keeps today's filter behavior bit-for-bit from the user's perspective.

## Goals
- Go toolchain `1.23.1` → `1.26.5` in all 9 `go.mod` files (root, `example/go-simple-extension`, 7× `test/e2e/filters/*`) plus `go.work`.
- All direct dependencies bumped to latest compatible with Go 1.26.5; `github.com/envoyproxy/envoy` `v1.31.2` → `v1.40.0` everywhere it appears (direct in root, indirect in example/filters).
- All Envoy container references → v1.40 line: `pkg/suite/suite.go:DefaultEnvoyImageVersion`, `Dockerfile.example` (`envoyproxy/envoy:contrib-v1.31-latest`), `docker-compose.yaml`, `hack/run-filter-local.sh` (currently stale at `contrib-v1.30`), CI/e2e compile path.
- All Go builder references → 1.26.5 line: `Dockerfile.example` (`golang:1.23-bullseye` ×2), `hack/run-filter-local.sh` (`golang:1.22-bullseye`), `.github/workflows/test.yaml` (`setup-go 1.23.1`), `.golangci.yml` (`go: "1.23"`).
- `make audit` (fmt, vet, lint, unit test with `-race`) green; `make e2e` (docker-built `filter.so` fixtures ×7 + Envoy v1.40 containers) behaves as today.
- Code adapted only as far as compilation + today's expectations require, following `.opencode/skills/gonvoy-code-style` and `gonvoy-naming-convention` (2-group imports, `Http` not `HTTP`, `var()` sentinels + `%w`, single-letter receivers, godoc).

## Non-Goals
- No new filter features, no gonvoy public API redesign, no Go 1.26 idiom modernization beyond what compilation requires (`go fix` modernizers out of scope unless a build forces it).
- No attempt to keep older Envoy versions working; accepted gap from v1.31 → v1.40 (human decision 2026-10-18). README compatibility matrix update deferred to later.
- No release/tag of gonvoy itself in this work.

## Anti-Goals
- Silently changing e2e/unit expectations to hide a v1.40 behavior change. Prerequisite (human decision 2026-10-18): before any code change, do a thorough recon of what changed/dropped in Envoy v1.40.0's Go library versus v1.31.2 for every API gonvoy uses; note everything that no longer matches. Only then decide whether expectations can be met or a BREAKING note is owed — no partial landing until then.
- Partial bumps (e.g., root `go.mod` only) that leave the `go.work` workspace inconsistent.

## Builder Calls (settled without human input)
- E2E verification: attempt local Docker first (`make e2e`); if Docker/network unavailable, fall back to CI-driven verify loop.
- CI workflow files: minimal edits allowed (e.g., `setup-go` version, runner/action compat) only as far as needed for Go 1.26 / Envoy v1.40 to go green; no unrelated workflow modernization.

## Constraints
- Exact pins where stated: Go `1.26.5` literal in `go` directives (repo precedent: existing files use patch form `go 1.23.1`).
- Indirect deps: minor/patch bumps within current majors only (human decision 2026-10-18) — no major-version jumps for protobuf/testify/gjson/etc.
- Envoy SDK `github.com/envoyproxy/envoy` → `v1.40.0` exactly.
- `GODEBUG=cgocheck=0` requirement for Envoy Golang filter stays; Green Tea GC (default on since Go 1.26) + ~30% cheaper cgo calls are accepted runtime changes, not tuning targets.
- `golangci-lint 1.61.0` predates Go 1.26 and must be bumped to a version that supports Go 1.26; v1→v2 config format risk noted, resolve at plan time.
- E2E needs Docker (image pull + `filter.so` built with `-buildmode=c-shared`); `envoy.yaml` fixtures keep `v3alpha.Config` type URL unless v1.40 rejects it (then BREAKING note).

## Decided Parts (human calls, builder must follow)
- Strict v1.40.0 target, gap accepted — no compat shims for v1.29–v1.31.
- Relevance bar = today's unit + e2e expectations pass unchanged; failures become BREAKING notes.
- All 9 modules + workspace move atomically; `replace github.com/ardikabs/gonvoy => …` directives stay.

## Left to the Builder
- Exact indirect versions post-`tidy`, exact `contrib-v1.40-*` image tag variant (verify tag exists), exact golangci-lint replacement version, exact builder base-image debian codename for Go 1.26.5 (verify `golang:1.26.5-*` tag exists).
- Any Envoy Go-API signature drift between `contrib/golang/common/go/api` v1.31.2 → v1.40.0 (method sets used: `HeaderMap`, `BufferInstance`, `FilterCallbackHandler`, metrics, `StreamFilterConfigParser`) and minimal code deltas to compile.
- Whether `envoy.yaml` fixtures or `xds.TypedStruct`/`gjson`/`protobuf` handling needs changes under v1.40.
