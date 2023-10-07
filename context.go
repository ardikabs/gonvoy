package envoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Header interface {
	api.HeaderMap
}

type Context interface {
	Header

	Log(lvl LogLevel, msg string)

	JSON(statusCode int, body []byte, headers map[string]string, opts ...LocalReplyOption) error

	PlainText(statusCode int, msg string, opts ...LocalReplyOption) error

	StatusType() api.StatusType
}

type RequestContext interface {
	Context

	Host() string

	Path() string

	Method() string
}

type ResponseContext interface {
	Context

	Status() (int, bool)
}

type LocalReplyOptions struct {
	statusType       api.StatusType
	localReplyDetail string
	grpcStatusCode   int64
}

type LocalReplyOption func(o *LocalReplyOptions)

func GetDefaultLocalReplyOptions() *LocalReplyOptions {
	return &LocalReplyOptions{
		statusType:       api.LocalReply,
		grpcStatusCode:   -1,
		localReplyDetail: "terminated from plugin",
	}
}

func WithLocalReplyDetail(detail string) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.localReplyDetail = detail
	}
}

func WithGrpcStatus(status int64) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.grpcStatusCode = status
	}
}
