# gRPC scope

gonvoy is an HTTP filter. gRPC requests reach it because gRPC runs on HTTP/2, but gonvoy is built around plain HTTP bodies: read once, optionally replace, then continue. Use this page to decide whether a gRPC job belongs here.

## Fits well: metadata decisions

These jobs never parse message frames, so they stay cheap and safe:

* Check metadata such as `:authority`, `grpc-timeout`, or custom `x-` headers, then answer or pass through.
* Enforce auth, log calls, record metrics, change routes, pin an upstream host, drain a connection, or read secrets and TLS details.
* Reject locally when the caller is not allowed. Prefer the gRPC-aware status detail for local rejects where the setup supports it. Direct answers from filter code stay HTTP shaped (JSON, plain text, or a custom type set through headers plus an explicit status and body bytes).

Return without writing to pass through. Nothing parks unless a middleware layer explicitly proceeds toward upstream.

Good examples: allow or deny by API key, attach a request ID, pick a route by path, count calls by method.

## Does not fit: frames, trailers, streams

Do not use gonvoy to transform gRPC payloads or to hold streams open:

* gRPC messages are length-prefixed binary frames, not JSON documents. gonvoy does not parse that framing for you.
* Replacements are buffered fully in memory, which suits small payloads and breaks down for streams held open across many messages.
* Reads block the handler while bytes arrive, with a timeout backstop that pauses the stream and resumes on the next callback. That model fits one buffered body, not a long-lived RPC.
* The built-in body helper accepts JSON only. Anything else needs manual decoding from the raw request body.
* Final RPC status travels in trailers (`grpc-status`, `grpc-message`). Filter code has no trailer accessors, so branching on the final status or rewriting it is out of scope.
* Upstream bodies are only available after forwarding and only when response access is turned on. That fits a small unary reply, not a stream.

A tiny unary transform can technically run, but it stays fragile: one framing mistake corrupts the message, and there is no trailer signal to fall back on.

## Rule

If the job reads headers and decides, gonvoy fits. If the job reads frames, changes payload bytes, branches on trailers, or stays open for a stream, keep it out of gonvoy and use a gRPC-aware filter path instead.
