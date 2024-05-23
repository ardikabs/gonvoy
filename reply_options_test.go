package gonvoy

import (
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
)

func TestResponseCodeDetailPrefix(t *testing.T) {
	prefix := ResponseCodeDetailPrefix("goext_test")
	assert.Equal(t, "goext_test{any_message_appears}", prefix.Wrap("any message appears"))

	msg := ResponseCodeDetailPrefix("goext_test").Wrap("foo bar")
	assert.Equal(t, "goext_test{foo_bar}", prefix.Wrap(msg))
}

func TestLocalReplyOptions(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		ro := NewLocalReplyOptions()
		assert.Equal(t, api.LocalReply, ro.statusType)
		assert.Equal(t, int64(-1), ro.grpcStatusCode)
		assert.Equal(t, DefaultResponseCodeDetails, ro.responseCodeDetails)
	})

	t.Run("custom", func(t *testing.T) {
		ro := NewLocalReplyOptions(
			LocalReplyWithRCDetails(DefaultResponseCodeDetailInfo.Wrap("any message")),
			LocalReplyWithGRPCStatus(10),
			LocalReplyWithStatusType(api.StopAndBuffer),
		)

		assert.Equal(t, api.StopAndBuffer, ro.statusType)
		assert.Equal(t, int64(10), ro.grpcStatusCode)
		assert.Equal(t, "goext_info{any_message}", ro.responseCodeDetails)
	})
}
