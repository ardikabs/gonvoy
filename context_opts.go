package envoy

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ardikabs/go-envoy/pkg/types"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type ContextOption func(c *context) error

func WithRequestHeaderMap(header api.RequestHeaderMap) ContextOption {
	return func(c *context) error {
		req, err := types.NewRequest(
			header.Method(),
			header.Host(),
			types.WithRequestURI(header.Path()),
			types.WithRequestHeaderRangeSetter(header),
		)
		if err != nil {
			return fmt.Errorf("failed during initializing Http Request, %w", err)
		}

		c.httpReq = req
		c.reqHeaderMap = header
		return nil
	}
}

func WithBufferInstance(buffer api.BufferInstance) ContextOption {
	return func(c *context) error {
		if buffer.Len() == 0 {
			return nil
		}

		if c.httpReq == nil || c.httpResp == nil {
			return fmt.Errorf("either http request or response must not nil, it should be initialize first. See WithRequestHeaderMap or WithResponseHeaderMap options.")
		}

		if c.httpReq != nil {
			buf := bytes.NewBuffer(buffer.Bytes())
			c.httpReq.Body = io.NopCloser(buf)
		}

		if c.httpResp != nil {
			buf := bytes.NewBuffer(buffer.Bytes())
			c.httpResp.Body = io.NopCloser(buf)
		}

		return nil
	}
}

func WithResponseHeaderMap(header api.ResponseHeaderMap) ContextOption {
	return func(c *context) error {
		code, ok := header.Status()
		if !ok {
			return fmt.Errorf("status code is not defined")
		}

		resp, err := types.NewResponse(code, types.WithResponseHeaderRangeSetter(header))
		if err != nil {
			return fmt.Errorf("failed during initializing Http Response, %w", err)
		}

		c.httpResp = resp
		c.respHeaderMap = header
		return nil
	}
}
