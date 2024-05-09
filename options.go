package gonvoy

import (
	"fmt"
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

type ReplyOptions struct {
	statusType          api.StatusType
	responseCodeDetails string
	grpcStatusCode      int64
}

type ReplyOption func(o *ReplyOptions)

func NewDefaultReplyOptions() *ReplyOptions {
	return &ReplyOptions{
		statusType:          api.LocalReply,
		grpcStatusCode:      -1,
		responseCodeDetails: DefaultResponseCodeDetails,
	}
}

// WithResponseCodeDetails sets response code details for a request/response to the envoy context
// It accepts a string, but commonly for convention purpose please check RespCodeDetails constants.
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

func WithStatusType(status api.StatusType) ReplyOption {
	return func(o *ReplyOptions) {
		o.statusType = status
	}
}
