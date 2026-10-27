package gonvoy

import (
	"net/http"

	"github.com/ardikabs/gonvoy/pkg/types"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

func (c *context) LoadRequestHeaders(header api.RequestHeaderMap) {
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
	c.checkRequestBodyAccessibility()
}

func (c *context) LoadResponseHeaders(header api.ResponseHeaderMap) {
	c.reset()

	if _, ok := header.Status(); !ok {
		return
	}

	// Single-materialization: body-access check needs 2-3 headers, not a full
	// C-iterator walk. Direct Gets cost 2-3 cgo calls vs N callbacks.
	c.checkResponseBodyAccessibilityLive(header)
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

func (c *context) SendResponse(code int, bodyText string, opts ...LocalReplyOption) error {
	reply := NewLocalReplyOptions(opts...)

	c.pcb.SendLocalReply(code, bodyText, reply.headers, reply.grpcStatusCode, reply.responseCodeDetails)
	c.committed = true

	return nil
}

func (c *context) JSON(code int, body []byte, opts ...LocalReplyOption) error {
	reply := NewLocalReplyOptions(opts...)

	if reply.headers == nil {
		reply.headers = make(http.Header)
	}

	if body == nil {
		body = []byte("{}")
	}

	reply.headers.Set(HeaderContentType, MIMEApplicationJSON)

	c.pcb.SendLocalReply(code, string(body), reply.headers, reply.grpcStatusCode, reply.responseCodeDetails)
	c.committed = true

	return nil
}

func (c *context) ReloadRoute() {
	c.cb.ClearRouteCache()
}

// bodyAccessPossible reports whether any body rule is enabled. With every
// rule off the outcome is forced false in both strict and open modes, so the
// header inspection below short-circuits. The response path consults the
// request flags, mirroring checkResponseBodyAccessibility exactly.
func (c *context) bodyAccessPossible() bool {
	return c.requestBodyAccessRead || c.requestBodyAccessWrite ||
		c.responseBodyAccessRead || c.responseBodyAccessWrite
}

// checkRequestBodyAccessibility checks the accessibility of the request body based on the request header.
func (c *context) checkRequestBodyAccessibility() {
	if !c.bodyAccessPossible() {
		c.requestBodyAccessRead, c.requestBodyAccessWrite = false, false
		return
	}

	header := c.httpReq.Header

	if off := header.Get(HeaderXRequestBodyAccess) == XRequestBodyAccessOff; off {
		c.requestBodyAccessRead, c.requestBodyAccessWrite = false, false
		return
	}

	c.requestBodyAccessRead, c.requestBodyAccessWrite = c.checkHTTPBodyAccessibility(c.strictBodyAccess, c.requestBodyAccessRead, c.requestBodyAccessWrite, header)
}

// checkResponseBodyAccessibility checks the accessibility of the response body based on the response header.
func (c *context) checkResponseBodyAccessibility(header http.Header) {

	if !c.bodyAccessPossible() {
		c.responseBodyAccessRead, c.responseBodyAccessWrite = false, false
		return
	}

	if off := header.Get(HeaderXResponseBodyAccess) == XResponseBodyAccessOff; off {
		c.responseBodyAccessRead, c.responseBodyAccessWrite = false, false
		return
	}

	c.responseBodyAccessRead, c.responseBodyAccessWrite = c.checkHTTPBodyAccessibility(c.strictBodyAccess, c.responseBodyAccessRead, c.responseBodyAccessWrite, header)
}

// checkResponseBodyAccessibilityLive is the EncodeHeaders fast path: same semantics
// as checkResponseBodyAccessibility but reads 2-4 headers via direct Gets instead
// of materializing the full map. Saves one full C-iterator walk per response.
func (c *context) checkResponseBodyAccessibilityLive(header api.HeaderMap) {
	if !c.bodyAccessPossible() {
		c.responseBodyAccessRead, c.responseBodyAccessWrite = false, false
		return
	}

	if v, ok := header.Get(HeaderXResponseBodyAccess); ok && v == XResponseBodyAccessOff {
		c.responseBodyAccessRead, c.responseBodyAccessWrite = false, false
		return
	}

	c.responseBodyAccessRead, c.responseBodyAccessWrite = c.checkHTTPBodyAccessibilityLive(c.strictBodyAccess, c.responseBodyAccessRead, c.responseBodyAccessWrite, header)
}

// checkHTTPBodyAccessibilityLive mirrors checkHTTPBodyAccessibility for a live Envoy map.
func (c *context) checkHTTPBodyAccessibilityLive(strict, allowRead, allowWrite bool, header api.HeaderMap) (read, write bool) {
	access := c.isHTTPBodyAccessibleLive(header)
	if !access {
		return
	}

	if !strict {
		read = access && (allowRead || allowWrite)
		write = access && allowWrite
		return
	}

	operation := ""
	if v, ok := header.Get(HeaderXContentOperation); ok {
		operation = v
	}

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

// isHTTPBodyAccessibleLive mirrors isHTTPBodyAccessible for a live Envoy map.
func (c *context) isHTTPBodyAccessibleLive(header api.HeaderMap) bool {
	cType, ok := header.Get(HeaderContentType)
	if !ok || cType == "" {
		return false
	}

	if util.StringStartsWith(cType, validContentTypes...) {
		return true
	}

	if util.StringStartsWith(cType, MIMEApplicationGRPC) &&
		(cType == MIMEApplicationGRPC || cType[len(MIMEApplicationGRPC)] == '+') {
		return false
	}

	if cLength, ok := header.Get(HeaderContentLength); ok && cLength != "" {
		return cLength != "0"
	}

	return false
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

// validContentTypes is the package-level allowlist for HTTP body access.
// Hoisted from per-call literal: isHTTPBodyAccessible runs on every request.
var validContentTypes = []string{
	MIMEApplicationJSON,
	MIMEApplicationXML,
	MIMEApplicationForm,
	MIMEApplicationProtobuf,
	MIMEApplicationMsgpack,
	MIMETextXML,
	MIMEMultipartForm,
	MIMEOctetStream,
}

// isHTTPBodyAccessible checks if the HTTP body is accessible based on the provided header.
// It returns true if the body is accessible, otherwise false.
// It checks the accessibility of the body based on the content type and/or content length.
func (c *context) isHTTPBodyAccessible(header http.Header) bool {
	cType := header.Get(HeaderContentType)

	if cType == "" {
		return false
	}

	if util.StringStartsWith(cType, validContentTypes...) {
		return true
	}

	// gRPC content type is considered inaccessible.
	// Content type is gRPC if it is exactly "application/grpc" or starts with "application/grpc+".
	// Particularly, something like "application/grpc-web" is not gRPC.
	if util.StringStartsWith(cType, MIMEApplicationGRPC) &&
		(cType == MIMEApplicationGRPC || cType[len(MIMEApplicationGRPC)] == '+') {
		return false
	}

	// For other content types, data is considered accessible only when Content-Length is neither empty nor zero.
	// Consequently, chunked data of these content types is considered as inaccessible.
	if cLength := header.Get(HeaderContentLength); cLength != "" {
		return cLength != "0"
	}

	return false
}

func shouldOmitContentLengthOnRequest(c bridgeContext, header api.HeaderMap) bool {
	ctx := mustCastToContext(c)
	if ctx.preserveContentLengthOnRequest {
		return false
	}

	// If Content-Length is not preserved, it is removed, implying that the request is converted to chunked encoding.
	// This removal is also necessary to prevent Envoy from crashing if the body is modified differently from the downstream.
	return deleteContentLength(header)
}

func shouldOmitContentLengthOnResponse(c bridgeContext, header api.HeaderMap) bool {
	ctx := mustCastToContext(c)
	if ctx.preserveContentLengthOnResponse {
		return false
	}

	// If Content-Length is not preserved, it is removed, implying that the request is converted to chunked encoding.
	// This removal is also necessary to prevent Envoy from crashing if the body is modified differently from the downstream.
	return deleteContentLength(header)
}

func deleteContentLength(header api.HeaderMap) bool {
	if _, ok := header.Get(HeaderContentLength); ok {
		header.Del(HeaderContentLength)
		return true
	}

	return false
}

func mustCastToContext(c bridgeContext) *context {
	ctx, ok := c.(*context)
	if !ok {
		panic("invalid context type")
	}
	return ctx
}
