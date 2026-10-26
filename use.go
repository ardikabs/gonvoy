package gonvoy

import (
	"net/http"
	"slices"
)

// Middleware is one layer around the next Handler in the chain. Call
// next to proceed toward upstream; code after it runs when the upstream
// response arrives. Do not call the upstream hop directly: Use owns the
// internal terminal that parks for the response.
//
//	func Auth(next Handler) Handler {
//		return Handler(func(c Context) error {
//			if c.Request().Header.Get("Authorization") == "" {
//				return c.String(401, "unauthorized")
//			}
//			return next(c)
//		})
//	}
type Middleware func(next Handler) Handler

// newTerminalHandler returns the internal upstream hop as a Handler. It is
// idempotent by design: callNext guards with nextCalled, so a second call
// is a no-op and never re-parks or re-applies headers.
func newTerminalHandler() Handler {
	return terminalHandler
}

// terminalHandler is the shared stateless upstream hop. Hoisted from a per-request
// closure alloc: terminal behavior closes over nothing per-request.
var terminalHandler = Handler(func(c Context) error {
	if n, ok := c.(interface{ forwardUpstream() error }); ok {
		return n.forwardUpstream()
	}

	return nil
})

// Use composes layers around the internal upstream terminal. It is Chain
// with the internal terminal as the final: the innermost next sends the
// request upstream, and code after it runs when the upstream response
// arrives. Order is outermost first. Pass the result to the
// registration entrypoint:
//
//	envoy.Handle("auth", gonvoy.Use(Auth, Log))
func Use(middlewares ...Middleware) Handler {
	for _, m := range middlewares {
		if m == nil {
			panic("gonvoy.Use: middleware should not be nil")
		}
	}

	return Chain(newTerminalHandler(), middlewares...)
}

// Chain composes layers around an explicit final Handler. The terminal lives
// outside the onion: outermost(inner(innermost(final))) runs first. An
// answered chain (status set) or an error short-circuits without forwarding.
// A chain that returns unanswered passes through: the bridge continues
// upstream without parking, applying header, path, host, and method
// mutations. Only an explicit next call (or a swapped request body, which
// streams upstream solely through the terminal hop) parks for the response.
// Skipping an inner layer therefore skips only logic, never forwarding.
// Order is outermost first:
//
//	gonvoy.Chain(final, Auth, Log)
func Chain(final Handler, middlewares ...Middleware) Handler {
	if final == nil {
		panic("gonvoy.Chain: final handler should not be nil")
	}

	for _, m := range middlewares {
		if m == nil {
			panic("gonvoy.Chain: middleware should not be nil")
		}
	}

	// Build the chain once at registration: middlewares close over nothing
	// per-request. Rebuilding m(h) on every request wasted N closures.
	built := final
	for _, m := range slices.Backward(middlewares) {
		built = m(built)
	}

	return Handler(func(c Context) error {
		if err := built(c); err != nil {
			return err
		}

		if c.Status() != 0 {
			return nil
		}

		// Passthrough without parking: nobody asked for the upstream
		// response, so the bridge continues without a second rendezvous. A
		// swapped request body still needs the terminal hop to stream it
		// upstream.
		if bodyReplacedWithoutNext(c) {
			return terminalHandler(c)
		}

		return nil
	})
}

// bodyReplacedWithoutNext reports whether the handler swapped r.Body without
// proceeding. The replacement streams upstream solely through the terminal
// hop, so the chain fallback keeps it instead of passing through. Anything
// else (including a nil body, which the terminal hop would equally skip) is
// safe to pass through.
func bodyReplacedWithoutNext(c Context) bool {
	hc, ok := c.(*handlerContext)
	if !ok || hc.ex == nil || hc.r == nil || hc.r.Body == nil || hc.r.Body == http.NoBody {
		return false
	}

	gb, ok := hc.r.Body.(*gatedBodyReader)
	return !ok || gb != hc.ex.gatedBody
}
