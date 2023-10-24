package envoy

import (
	"fmt"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HandlerManager interface {
	Use(handler Handler)
	Handle(ctx Context) api.StatusType
}

type manager struct {
	errorHandler ErrorHandler
	handlers     []Handler
}

func NewManager() HandlerManager {
	return NewManagerWithErrorHandler(DefaultErrorHandler)
}

func NewManagerWithErrorHandler(errHandler ErrorHandler) HandlerManager {
	return &manager{
		errorHandler: errHandler,
	}
}

func (h *manager) Use(handler Handler) {
	if handler == nil {
		return
	}

	h.handlers = append(h.handlers, handler)
}

func (h *manager) Handle(ctx Context) (status api.StatusType) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w; %v", errs.ErrPanic, r)
		}

		status = ctx.StatusType()

		if err != nil {
			status = h.errorHandler(ctx, err)
		}

		return
	}()

	if len(h.handlers) == 0 {
		return
	}

	stack := func(c Context) error { return nil }
	for i := len(h.handlers) - 1; i >= 0; i-- {
		stack = HandlerDecorator(h.handlers[i](stack))
	}

	err = stack(ctx)
	return
}
