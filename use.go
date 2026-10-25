package gonvoy

import (
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
	return Handler(func(c Context) error {
		if n, ok := c.(interface{ forwardUpstream() error }); ok {
			return n.forwardUpstream()
		}

		return nil
	})
}

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
// outside the onion: outermost(inner(innermost(final))) runs first, then the
// hidden upstream hop runs when the chain leaves the request unanswered.
// Skipping an inner layer therefore skips only logic, never forwarding. An
// answered chain (status set) or an error short-circuits without forwarding.
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

	return Handler(func(c Context) error {
		h := final
		for _, m := range slices.Backward(middlewares) {
			h = m(h)
		}

		if err := h(c); err != nil {
			return err
		}

		if c.Status() != 0 {
			return nil
		}

		return newTerminalHandler()(c)
	})
}
