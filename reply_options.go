package gonvoy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	DefaultResponseCodeDetailInfo         = ResponseCodeDetailPrefix("goext_info")
	DefaultResponseCodeDetailUnauthorized = ResponseCodeDetailPrefix("goext_unauthorized")
	DefaultResponseCodeDetailAccessDenied = ResponseCodeDetailPrefix("goext_access_denied")
	DefaultResponseCodeDetailError        = ResponseCodeDetailPrefix("goext_error")

	DefaultResponseCodeDetails = DefaultResponseCodeDetailInfo.Wrap("via_Go_extension")
)

// ResponseCodeDetailPrefix represents a prefix for response code details.
type ResponseCodeDetailPrefix string

// Wrap wraps message with given response code detail prefix
func (prefix ResponseCodeDetailPrefix) Wrap(message string) string {
	switch {
	case strings.HasPrefix(message, string(prefix)):
		// if the incoming message has a same prefix, return as it is
		return message
	}

	return fmt.Sprintf("%s{%s}", prefix, util.ReplaceAllEmptySpace(message))
}

type LocalReplyOptions struct {
	headers             http.Header
	statusType          api.StatusType
	responseCodeDetails string
	grpcStatusCode      int64
}

type LocalReplyOption func(o *LocalReplyOptions)

func NewLocalReplyOptions(opts ...LocalReplyOption) *LocalReplyOptions {
	ro := &LocalReplyOptions{
		statusType:          api.LocalReply,
		grpcStatusCode:      -1,
		responseCodeDetails: DefaultResponseCodeDetails,
	}

	for _, opt := range opts {
		opt(ro)
	}

	return ro
}

// LocalReplyWithRCDetails sets response code details for a request/response to the envoy context
// It accepts a string, but commonly for convention purpose please check ResponseCodeDetailPrefix.
func LocalReplyWithRCDetails(detail string) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.responseCodeDetails = detail
	}
}

// LocalReplyWithGRPCStatus sets the gRPC status code for the local reply options.
// The status code is used to indicate the result of the gRPC operation.
func LocalReplyWithGRPCStatus(status int64) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.grpcStatusCode = status
	}
}

// LocalReplyWithStatusType sets the status type for the local reply options.
func LocalReplyWithStatusType(status api.StatusType) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.statusType = status
	}
}

// LocalReplyWithHTTPHeaders sets the HTTP headers for the local reply options.
func LocalReplyWithHTTPHeaders(headers http.Header) LocalReplyOption {
	return func(o *LocalReplyOptions) {
		o.headers = headers
	}
}
