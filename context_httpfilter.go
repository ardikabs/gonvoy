package gonvoy

import (
	"bytes"
	"io"
	"net/http"
	"runtime"

	"github.com/ardikabs/gonvoy/pkg/types"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

func (c *context) RequestHeader() Header {
	if c.reqHeaderMap == nil {
		panic("The Request Header is not initialized yet, likely because the filter has not yet traversed the HTTP request or OnRequestHeader is disabled. Please refer to the previous HTTP filter behavior.")
	}

	h := &header{HeaderMap: c.reqHeaderMap}
	if c.reloadRouteOnRequestHeader {
		h.clearRouteCache = c.callback.ClearRouteCache
	}

	return h
}

func (c *context) ResponseHeader() Header {
	if c.respHeaderMap == nil {
		panic("The Response Header is not initialized yet and is only available during the OnRequestHeader, OnRequestBody, and OnResponseHeader phases")
	}

	return &header{HeaderMap: c.respHeaderMap}
}

func (c *context) RequestBody() Body {
	if c.reqBufferInstance == nil {
		panic("The Request Body is not initialized yet and is only accessible during the OnRequestBody, OnResponseHeader, and OnResponseBody phases.")
	}

	return &bodyWriter{
		writeable: c.IsRequestBodyWritable(),
		header:    c.reqHeaderMap,
		buffer:    c.reqBufferInstance,
	}
}

func (c *context) ResponseBody() Body {
	if c.respBufferInstance == nil {
		panic("The Response Body is not initialized yet and is only accessible during OnResponseBody phase.")
	}

	return &bodyWriter{
		writeable: c.IsResponseBodyWritable(),
		header:    c.respHeaderMap,
		buffer:    c.respBufferInstance,
	}
}

func (c *context) SetRequestHost(host string) {
	c.reqHeaderMap.SetHost(host)

	if c.reloadRouteOnRequestHeader {
		c.callback.ClearRouteCache()
	}
}

func (c *context) SetRequestMethod(method string) {
	c.reqHeaderMap.SetMethod(method)

	if c.reloadRouteOnRequestHeader {
		c.callback.ClearRouteCache()
	}
}

func (c *context) SetRequestPath(path string) {
	c.reqHeaderMap.SetPath(path)

	if c.reloadRouteOnRequestHeader {
		c.callback.ClearRouteCache()
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
	c.checkRequestBodyAccessibility()
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
	c.checkResponseBodyAccessibility()
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

func (c *context) IsRequestBodyAccessible() bool {
	return c.IsRequestBodyReadable() || c.IsRequestBodyWritable()
}

func (c *context) IsRequestBodyReadable() bool {
	// If the Request Body can be accessed, but the current phase has already been committed,
	// then Request Body is no longer accessible
	if c.committed {
		return false
	}

	return c.requestBodyAccessRead
}

func (c *context) IsRequestBodyWritable() bool {
	// If the Request Body can be modified, but the current phase has already been committed,
	// then Request Body is no longer modifiable
	if c.committed {
		return false
	}

	return c.requestBodyAccessWrite
}

func (c *context) IsResponseBodyAccessible() bool {
	return c.IsResponseBodyReadable() || c.IsResponseBodyWritable()
}

func (c *context) IsResponseBodyReadable() bool {
	// If the Response Body can be accessed, but the current phase has already been committed,
	// then Response Body is no longer accessible
	if c.committed {
		return false
	}

	return c.responseBodyAccessRead
}

func (c *context) IsResponseBodyWritable() bool {
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

func (c *context) ReloadRoute() {
	c.callback.ClearRouteCache()
}

// checkRequestBodyAccessibility checks the accessibility of the request body based on the request header.
func (c *context) checkRequestBodyAccessibility() {
	header := c.httpReq.Header

	if off := header.Get(HeaderXRequestBodyAccess) == XRequestBodyAccessOff; off {
		c.requestBodyAccessRead, c.requestBodyAccessWrite = false, false
		return
	}

	c.requestBodyAccessRead, c.requestBodyAccessWrite = c.checkHTTPBodyAccessibility(c.strictBodyAccess, c.requestBodyAccessRead, c.requestBodyAccessWrite, header)
}

// checkResponseBodyAccessibility checks the accessibility of the response body based on the response header.
func (c *context) checkResponseBodyAccessibility() {
	header := c.httpResp.Header

	if off := header.Get(HeaderXResponseBodyAccess) == XResponseBodyAccessOff; off {
		c.responseBodyAccessRead, c.responseBodyAccessWrite = false, false
		return
	}

	c.responseBodyAccessRead, c.responseBodyAccessWrite = c.checkHTTPBodyAccessibility(c.strictBodyAccess, c.responseBodyAccessRead, c.responseBodyAccessWrite, header)
}

// checkHTTPBodyAccessibility checks the accessibility of the HTTP body based on the provided parameters.
// If strict is false, it determines the accessibility based on the allowRead and allowWrite flags.
// If strict is true, it checks the accessibility based on the operation specified in the header.
// The read and write flags indicate whether the HTTP body is readable and writable, respectively.
// The header parameter contains the HTTP header.
func (c *context) checkHTTPBodyAccessibility(strict, allowRead, allowWrite bool, header http.Header) (read, write bool) {
	access := c.isHTTPBodyAccessible(header)
	if !access {
		return
	}

	if !strict {
		read = access && (allowRead || allowWrite)
		write = access && allowWrite
		return
	}

	operation := header.Get(HeaderXContentOperation)

	if util.In(operation, ContentOperationReadOnly, ContentOperationRO) {
		read = access && allowRead
		return
	}

	if util.In(operation, ContentOperationReadWrite, ContentOperationRW) {
		write = access && allowWrite
		read = write
		return
	}

	return
}

// isHTTPBodyAccessible checks if the HTTP body is accessible based on the provided header.
// It returns true if the body is accessible, otherwise false.
// It checks the accessibility of the body based on the content type and/or content length.
func (c *context) isHTTPBodyAccessible(header http.Header) bool {
	cType := header.Get(HeaderContentType)

	if cType == "" {
		return false
	}

	validContentTypes := []string{
		MIMEApplicationJSON,
		MIMEApplicationXML,
		MIMEApplicationForm,
		MIMEApplicationProtobuf,
		MIMEApplicationMsgpack,
		MIMETextXML,
		MIMEMultipartForm,
		MIMEOctetStream,
	}

	if util.StringStartsWith(cType, validContentTypes...) {
		return true
	}

	// gRPC content type is considered inaccessible.
	// Content type is gRPC if it is exactly "application/grpc" or starts with "application/grpc+".
	// Particularly, something like "application/grpc-web" is not gRPC.
	if cType == MIMEApplicationGRPC &&
		(len(cType) == len(MIMEApplicationGRPC) || cType[len(MIMEApplicationGRPC)] == '+') {
		return false
	}

	// For other content types, data is considered accessible only when Content-Length is neither empty nor zero.
	// Consequently, chunked data of these content types is considered as inaccessible.
	if cLength := header.Get(HeaderContentLength); cLength != "" {
		return cLength != "0"
	}

	return false
}
