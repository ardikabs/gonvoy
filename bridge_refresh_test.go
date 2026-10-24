package gonvoy

import (
	"net/http"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
)

func TestBridge_RefreshRouteCache(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		r.URL.Path = "/staticreply"
		RefreshRouteCache(r.Context())
	})

	m := fakeBridgeHeaders(t, "")
	m.EXPECT().SetPath("/staticreply").Maybe()
	c := fakeWiredContext(t, func(fc *mock_envoy.FilterCallbackHandler) {
		fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{})
		fc.EXPECT().RefreshRouteCache()
	})
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done
}
