package envoy

import (
	"errors"
	"net/http"
	"runtime"
	"sync"

	"github.com/ardikabs/go-envoy/pkg/types"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Context interface {
	RequestHeader() Header

	ResponseHeader() Header

	Request() *http.Request

	SetRequest(api.RequestHeaderMap)

	Response() *http.Response

	SetResponse(api.ResponseHeaderMap)

	StreamInfo() api.StreamInfo

	// Store stores a value of any types to a key of any types.
	// Caution! The Store behavior is overwrite.
	Store(key any, value any)

	// Load loads a value from a key and put it to the receiver.
	// Load will return with true if value is loaded and compatible with the receiver,
	// otherwise return false if no value is found or an error occured during the load.
	Load(key any, receiver interface{}) (ok bool, err error)

	Log(lvl LogLevel, msg string)

	JSON(code int, b []byte, headers map[string]string, opts ...ReplyOption) error

	String(code int, s string, opts ...ReplyOption) error

	StatusType() api.StatusType

	Committed() bool
}

type context struct {
	reqHeaderMap  api.RequestHeaderMap
	respHeaderMap api.ResponseHeaderMap

	callback   api.FilterCallbacks
	statusType api.StatusType

	httpReq  *http.Request
	httpResp *http.Response

	storage sync.Map

	committed bool
}

func NewContext(callback api.FilterCallbacks) (Context, error) {
	if callback == nil {
		return nil, errors.New("callback MUST not nil")
	}

	return &context{
		callback: callback,
	}, nil
}

func (c *context) RequestHeader() Header {
	return &header{c.reqHeaderMap}
}

func (c *context) ResponseHeader() Header {
	return &header{c.respHeaderMap}
}

func (c *context) StreamInfo() api.StreamInfo {
	return c.callback.StreamInfo()
}

func (c *context) Log(lvl LogLevel, msg string) {
	c.callback.Log(api.LogType(lvl), msg)
}

func (c *context) JSON(code int, body []byte, headers map[string]string, opts ...ReplyOption) error {
	options := GetDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	if body == nil {
		body = []byte("{}")
	}

	headers["content-type"] = "application/json"
	c.callback.SendLocalReply(code, string(body), headers, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	runtime.GC()
	return nil
}

func (c *context) String(code int, s string, opts ...ReplyOption) error {
	options := GetDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	c.callback.SendLocalReply(code, s, map[string]string{}, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	return nil
}

func (c *context) SetRequest(header api.RequestHeaderMap) {
	c.reset()

	req, err := types.NewRequest(
		header.Method(),
		header.Host(),
		types.WithRequestURI(header.Path()),
		types.WithRequestHeaderRangeSetter(header),
	)
	if err != nil {
		c.Log(WarnLevel, err.Error())
		return
	}

	c.httpReq = req
	c.reqHeaderMap = header
}

func (c *context) Request() *http.Request {
	return c.httpReq
}

func (c *context) SetResponse(header api.ResponseHeaderMap) {
	c.reset()

	code, ok := header.Status()
	if !ok {
		return
	}

	resp, err := types.NewResponse(code, types.WithResponseHeaderRangeSetter(header))
	if err != nil {
		c.Log(WarnLevel, err.Error())
		return
	}

	c.httpResp = resp
	c.respHeaderMap = header
}

func (c *context) Response() *http.Response {
	return c.httpResp
}

func (c *context) StatusType() api.StatusType {
	return c.statusType
}

func (c *context) Committed() bool {
	return c.committed
}

func (c *context) reset() {
	c.statusType = api.Continue
	c.committed = false
}

func (c *context) Store(key any, value any) {
	c.storage.Store(key, value)
}

func (c *context) Load(key any, receiver interface{}) (bool, error) {
	if receiver == nil {
		return false, errors.New("context: receiver should not be nil")
	}

	v, ok := c.storage.Load(key)
	if !ok {
		return false, nil
	}

	if !CastTo(receiver, v) {
		return false, errors.New("context: receiver and value has an incompatible type")
	}

	return true, nil
}

type ReplyOptions struct {
	statusType          api.StatusType
	responseCodeDetails string
	grpcStatusCode      int64
}

type ReplyOption func(o *ReplyOptions)

func GetDefaultReplyOptions() *ReplyOptions {
	return &ReplyOptions{
		statusType:          api.LocalReply,
		grpcStatusCode:      -1,
		responseCodeDetails: "terminated from plugin",
	}
}

func WithResponseCodeDetails(detail string) ReplyOption {
	return func(o *ReplyOptions) {
		o.responseCodeDetails = detail
	}
}

func WithGrpcStatus(status int64) ReplyOption {
	return func(o *ReplyOptions) {
		o.grpcStatusCode = status
	}
}
