# Performance contract

gonvoy trades some throughput for the `net/http` abstraction: your handler
runs on its own goroutine with plain Go values, and the bridge carries
decisions back to Envoy's worker thread. This page states exactly what that
costs, what is free, and how to write filters that pay the minimum. Nothing
here requires tuning by default; the defaults already skip every cost tied
to a feature you do not use.

## How a request flows

1. Envoy calls the filter on its worker thread with the raw headers.
2. The bridge builds the request view once (method, host, path, parsed URI,
   header map), snapshots it for diffing, and runs your handler on a pooled
   worker goroutine.
3. Your handler returns a decision; the bridge applies it on the worker
   thread (header diffs, replies, reroutes) and returns to Envoy.

Two crossings between the threads per request (handoff, completion signal)
are the irreducible core of this design. Everything else below is either
shared with any Go filter that looks at the request, or skipped when unused.

## Free

These cost nothing when you do not use them, with no options needed:

- Request `host`/`method`/`path` and the response code in `Attr`, served
  from memory already on hand.
- Metrics, logging, counters. Metric handles are memoized per filter
  registration, loggers build on first use, and empty flushes are skipped.
- Body-access scans never run when every body rule is off (the default).
- Snapshot maps and writers, pooled and refilled in place.
- Passthrough without `next`: a handler that returns unanswered passes
  upstream without parking for a response nobody observes (see below).

## Declared: pay only for what you list

Anything that needs an Envoy fetch is off unless declared:

| To read | Declare | Otherwise |
| --- | --- | --- |
| `xds.*`, `response.*` attributes via `Attr` | `EnableEnvoyAttributes(names...)` | Reports absent, plus a one-time migration warning naming the key |
| Downstream TLS via `TLS()` | `EnableDownstreamTLS()` | Reports absent (same one-time warning on TLS listeners; plaintext stays silent) |
| Request/response bodies | `Read...`/`Write...Body()` | Reads fail as not accessible, writes fail with 502 |

## Inherent: the priced remainder

Measured on M1 arm64, `contrib-v1.40`, wrk 4t/200c/30s against an
equivalent raw-Go filter doing identical request work: a doing-nothing
gonvoy passthrough lands roughly 10–30% below raw depending on run
conditions (host variance alone is ±10%, so read single runs accordingly).
The gap is a sum of small fixed costs: worker handoff plus completion
signal, snapshot clone and diff, context plumbing, a handful of small
allocations, plus the request build both sides share. No single item
dominates it. Handler logic on top adds only single-digit thousands of rps,
so the percentage gap shrinks as filters do more real work.

One cost does dominate, and it is opt-in by construction: calling `next(c)`
parks until the upstream response arrives (roughly halves throughput versus
passthrough in our harness). Call `next` only when code after it observes
the response. A middleware chain shaped as `return next(c)` with nothing
after pays the park for no benefit. Return `nil` instead and let the
request pass through.

## Decision guide

- **Gonvoy fits** logic-heavy filters: auth, validation, body transforms,
  anything where middleware composition and unit tests matter. The fixed
  tax amortizes as handler work grows.
- **Raw Go fits** pure edge throttling at maximum RPS: header-only
  allow/deny with no logic, where every microsecond is visible and no
  abstraction pays for itself.

For option-by-option savings, see [Registration Options](options.md).
