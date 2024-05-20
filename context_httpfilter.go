package gonvoy

import (
	"bytes"
	"io"
	"net/http"
	"runtime"

	"github.com/ardikabs/gonvoy/pkg/types"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

func (c *context) RequestHeader() Header {
	if c.reqHeaderMap == nil {
		panic("The Request Header is not initialized yet, likely because the filter has not yet traversed the HTTP request or OnRequestHeader is disabled. Please refer to the previous HTTP filter behavior.")
	}

	return &header{c.reqHeaderMap}
}

func (c *context) ResponseHeader() Header {
	if c.respHeaderMap == nil {
		panic("The Response Header is not initialized yet and is only available during the OnRequestHeader, OnRequestBody, and OnResponseHeader phases")
	}

	return &header{c.respHeaderMap}
}

func (c *context) RequestBody() Body {
	if c.reqBufferInstance == nil {
		panic("The Request Body is not initialized yet and is only accessible during the OnRequestBody, OnResponseHeader, and OnResponseBody phases.")
	}

	return &bodyWriter{
		writeable: c.IsRequestBodyWriteable(),
		header:    c.reqHeaderMap,
		buffer:    c.reqBufferInstance,
	}
}

func (c *context) ResponseBody() Body {
	if c.respBufferInstance == nil {
		panic("The Response Body is not initialized yet and is only accessible during OnResponseBody phase.")
	}

	return &bodyWriter{
		writeable: c.IsResponseBodyWriteable(),
		header:    c.respHeaderMap,
		buffer:    c.respBufferInstance,
	}
}

func (c *context) SetRequestHeader(header api.RequestHeaderMap) {
	c.reset()

	req, err := types.NewRequest(
		header.Method(),
		header.Host(),
		types.WithRequestURI(header.Path()),
		types.WithRequestHeaderRangeSetter(header),
	)
	if err != nil {
		c.Log().Error(err, "while initialize Http Request")
		return
	}

	c.httpReq = req
	c.reqHeaderMap = header

	c.requestBodyAccessRead, c.requestBodyAccessWrite = checkBodyAccessibility(c.strictBodyAccess, c.requestBodyAccessRead, c.requestBodyAccessWrite, header)

	if off := req.Header.Get(HeaderXRequestBodyAccess) == XRequestBodyAccessOff; off {
		c.requestBodyAccessRead = !off && c.requestBodyAccessRead
		c.requestBodyAccessWrite = !off && c.requestBodyAccessWrite
	}
}

func (c *context) SetResponseHeader(header api.ResponseHeaderMap) {
	c.reset()

	code, ok := header.Status()
	if !ok {
		return
	}

	resp, err := types.NewResponse(code, types.WithResponseHeaderRangeSetter(header))
	if err != nil {
		c.Log().Error(err, "while initialize Http Response")
		return
	}

	c.httpResp = resp
	c.respHeaderMap = header

	c.responseBodyAccessRead, c.responseBodyAccessWrite = checkBodyAccessibility(c.strictBodyAccess, c.responseBodyAccessRead, c.responseBodyAccessWrite, header)

	if off := resp.Header.Get(HeaderXResponseBodyAccess) == XResponseBodyAccessOff; off {
		c.responseBodyAccessRead = !off && c.responseBodyAccessRead
		c.responseBodyAccessWrite = !off && c.responseBodyAccessWrite
	}
}

func (c *context) SetRequestBody(buffer api.BufferInstance, endStream bool) {
	if endStream {
		c.setRequestBody(buffer)
		return
	}

	if buffer.Len() > 0 {
		c.reqBufferBytes = append(c.reqBufferBytes, buffer.Bytes()...)
		buffer.Reset()
	}
}

func (c *context) setRequestBody(buffer api.BufferInstance) {
	if c.reqBufferBytes != nil {
		_ = buffer.Set(c.reqBufferBytes)
	}

	bytes := bytes.NewBuffer(buffer.Bytes())
	c.httpReq.Body = io.NopCloser(bytes)
	c.reqBufferInstance = buffer
}

func (c *context) SetResponseBody(buffer api.BufferInstance, endStream bool) {
	if endStream {
		c.setResponseBody(buffer)
		return
	}

	if buffer.Len() > 0 {
		c.respBufferBytes = append(c.respBufferBytes, buffer.Bytes()...)
		buffer.Reset()
	}
}

func (c *context) setResponseBody(buffer api.BufferInstance) {
	if c.respBufferBytes != nil {
		_ = buffer.Set(c.respBufferBytes)
	}

	buf := bytes.NewBuffer(buffer.Bytes())
	c.httpResp.Body = io.NopCloser(buf)
	c.respBufferInstance = buffer
}

func (c *context) Request() *http.Request {
	if c.httpReq == nil {
		panic("an HTTP Request is not initialized yet, likely because the filter has not yet traversed the HTTP request or OnRequestHeader is disabled. Please refer to the previous HTTP filter behavior.")
	}

	return c.httpReq
}

func (c *context) Response() *http.Response {
	if c.httpResp == nil {
		panic("an HTTP Response is not initialized yet, and is only available during the OnResponseHeader, and OnResponseBody phases.")
	}

	return c.httpResp
}

func (c *context) IsRequestBodyReadable() bool {
	// If the Request Body can be accessed, but the current phase has already been committed,
	// then Request Body is no longer accessible
	if c.committed {
		return false
	}

	return c.requestBodyAccessRead
}

func (c *context) IsRequestBodyWriteable() bool {
	// If the Request Body can be modified, but the current phase has already been committed,
	// then Request Body is no longer modifiable
	if c.committed {
		return false
	}

	return c.requestBodyAccessWrite
}

func (c *context) IsResponseBodyReadable() bool {
	// If the Response Body can be accessed, but the current phase has already been committed,
	// then Response Body is no longer accessible
	if c.committed {
		return false
	}

	return c.responseBodyAccessRead
}

func (c *context) IsResponseBodyWriteable() bool {
	// If the response body can be modified, but the current phase has already been committed,
	// then Response Body is no longer modifiable
	if c.committed {
		return false
	}

	return c.responseBodyAccessWrite
}

func (c *context) JSON(code int, body []byte, header http.Header, opts ...ReplyOption) error {
	options := NewDefaultReplyOptions(opts...)

	if header == nil {
		header = make(http.Header)
	}

	if body == nil {
		body = []byte("{}")
	}

	header.Set(HeaderContentType, MIMEApplicationJSON)
	c.callback.SendLocalReply(code, string(body), header, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	runtime.GC()
	return nil
}

func (c *context) String(code int, s string, header http.Header, opts ...ReplyOption) error {
	options := NewDefaultReplyOptions(opts...)

	if header == nil {
		header = make(http.Header)
	}

	header.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	c.callback.SendLocalReply(code, s, header, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	return nil
}

func (c *context) SkipNextPhase() error {
	c.statusType = api.Continue
	c.committed = true
	return nil
}
