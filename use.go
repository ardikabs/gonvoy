package gonvoy

import (
	"net/http"
	"slices"
)

// Use composes layers around a handler. The innermost next is
// the terminal: calling it sends the request upstream, and code after it
// runs when the upstream response arrives. Pass the result to the
// registration entrypoint:
//
//	envoy.Handle("auth", gonvoy.Use(Auth, Log))
func Use(handlers ...func(Handler) Handler) Handler {
	for _, handler := range handlers {
		if handler == nil {
			panic("gonvoy.Use: handler should not be nil")
		}
	}

	return HandlerFunc(func(w ResponseWriter, r *http.Request) {
		var h Handler = Terminal{}
		for _, m := range slices.Backward(handlers) {
			h = m(h)
		}
		h.ServeHTTP(w, r)
	})
}
