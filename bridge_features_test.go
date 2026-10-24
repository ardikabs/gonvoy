package gonvoy

import (
	"net/http"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeStreamInfo struct {
	api.StreamInfo
	drained bool
	ssl     api.SslConnection
}

func (f *fakeStreamInfo) DrainConnectionUponCompletion() { f.drained = true }

func (f *fakeStreamInfo) DownstreamSslConnection() api.SslConnection { return f.ssl }

func (f *fakeStreamInfo) GetRouteName() string { return "" }

func (f *fakeStreamInfo) FilterChainName() string { return "" }

func (f *fakeStreamInfo) Protocol() (string, bool) { return "", false }

func (f *fakeStreamInfo) ResponseCode() (uint32, bool) { return 0, false }

func (f *fakeStreamInfo) ResponseCodeDetails() (string, bool) { return "", false }

func (f *fakeStreamInfo) AttemptCount() uint32 { return 0 }

func (f *fakeStreamInfo) DynamicMetadata() api.DynamicMetadata { return nil }

func (f *fakeStreamInfo) DownstreamLocalAddress() string { return "" }

func (f *fakeStreamInfo) DownstreamRemoteAddress() string { return "" }

func (f *fakeStreamInfo) UpstreamLocalAddress() (string, bool) { return "", false }

func (f *fakeStreamInfo) UpstreamRemoteAddress() (string, bool) { return "", false }

func (f *fakeStreamInfo) UpstreamClusterName() (string, bool) { return "", false }

func (f *fakeStreamInfo) FilterState() api.FilterState { return nil }

func (f *fakeStreamInfo) VirtualClusterName() (string, bool) { return "", false }

func (f *fakeStreamInfo) WorkerID() uint32 { return 0 }

type fakeSecretManager struct {
	secrets map[string]string
}

func (f fakeSecretManager) GetGenericSecret(name string) (string, bool) {
	v, ok := f.secrets[name]
	return v, ok
}

type fakeSslConnection struct {
	api.SslConnection
}

func (fakeSslConnection) PeerCertificatePresented() bool { return true }

func (fakeSslConnection) PeerCertificateValidated() bool { return true }

func (fakeSslConnection) Sha256PeerCertificateDigest() string { return "ABCD" }

func (fakeSslConnection) SerialNumberPeerCertificate() string { return "42" }

func (fakeSslConnection) SubjectPeerCertificate() string { return "CN=test" }

func (fakeSslConnection) IssuerPeerCertificate() string { return "CN=ca" }

func (fakeSslConnection) SubjectLocalCertificate() string { return "CN=local" }

func (fakeSslConnection) UriSanPeerCertificate() []string { return []string{"spiffe://test"} }

func (fakeSslConnection) UriSanLocalCertificate() []string { return nil }

func (fakeSslConnection) DnsSansPeerCertificate() []string { return []string{"example.com"} }

func (fakeSslConnection) DnsSansLocalCertificate() []string { return nil }

func (fakeSslConnection) ValidFromPeerCertificate() (uint64, bool) { return 1700000000, true }

func (fakeSslConnection) ExpirationPeerCertificate() (uint64, bool) { return 0, false }

func (fakeSslConnection) TlsVersion() string { return "TLSv1.3" }

func (fakeSslConnection) CiphersuiteString() string { return "AES128-SHA" }

func (fakeSslConnection) CiphersuiteId() (uint16, bool) { return 47, true }

func (fakeSslConnection) SessionId() (string, bool) { return "", false }

func (fakeSslConnection) UrlEncodedPemEncodedPeerCertificate() string { return "PEM" }

func (fakeSslConnection) UrlEncodedPemEncodedPeerCertificateChain() string { return "" }

type hookDecoderCallbacks struct {
	api.DecoderFilterCallbacks
	onOverride func(host string, strict bool) error
}

func (f hookDecoderCallbacks) SetUpstreamOverrideHost(host string, strict bool) error {
	if f.onOverride != nil {
		return f.onOverride(host, strict)
	}

	return nil
}

func fakeWiredContext(t *testing.T, setup func(*mock_envoy.FilterCallbackHandler)) *context {
	t.Helper()

	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().GetProperty(mock.Anything).Return("", nil).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()

	if setup != nil {
		setup(fc)
	}

	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache:    newInternalCache(),
		strictBodyAccess: false,
	}})
	require.NoError(t, err)

	return mustCastToContext(c)
}

func TestBridge_OverrideUpstream(t *testing.T) {
	t.Parallel()

	var gotHost string
	var gotStrict bool
	hook := hookDecoderCallbacks{onOverride: func(host string, strict bool) error {
		gotHost, gotStrict = host, strict
		return nil
	}}

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		OverrideUpstreamHost(r, "10.0.0.9:8080", true)
	})

	c := fakeWiredContext(t, func(fc *mock_envoy.FilterCallbackHandler) {
		fc.EXPECT().DecoderFilterCallbacks().Return(hook)
	})
	f := newBridgeFilter(c, h)

	m := fakeBridgeHeaders(t, "")
	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	assert.Equal(t, "10.0.0.9:8080", gotHost)
	assert.True(t, gotStrict)
}

func TestBridge_DrainConnection(t *testing.T) {
	t.Parallel()

	si := &fakeStreamInfo{}
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		DrainConnection(r)
	})

	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().GetProperty(mock.Anything).Return("", nil).Maybe()
	fc.EXPECT().StreamInfo().Return(si)

	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache:    newInternalCache(),
		strictBodyAccess: false,
	}})
	require.NoError(t, err)

	f := newBridgeFilter(mustCastToContext(c), h)

	m := fakeBridgeHeaders(t, "")
	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	assert.True(t, si.drained)
}

func TestBridge_SecretRoundTrip(t *testing.T) {
	t.Parallel()

	type result struct {
		value string
		found bool
	}
	got := make(chan result, 1)
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		v, ok := Secret(r, "db")
		got <- result{value: v, found: ok}
	})

	c := fakeWiredContext(t, func(fc *mock_envoy.FilterCallbackHandler) {
		fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{})
		fc.EXPECT().SecretManager().Return(fakeSecretManager{secrets: map[string]string{"db": "s3cr3t"}})
	})
	f := newBridgeFilter(c, h)

	m := fakeBridgeHeaders(t, "")
	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	r := <-got
	assert.True(t, r.found)
	assert.Equal(t, "s3cr3t", r.value)
}

func TestBridge_SecretDestroyUnblocks(t *testing.T) {
	t.Parallel()

	m := fakeBridgeHeaders(t, "")
	c := fakeWiredContext(t, nil)
	f := newBridgeFilter(c, HandlerFunc(func(w ResponseWriter, r *http.Request) {}))
	ex := f.newExchange(m, true)
	ex.destroy()

	v, ok := Secret(ex.req, "db")

	assert.False(t, ok)
	assert.Equal(t, "", v)
}

func TestBridge_TLSNil(t *testing.T) {
	t.Parallel()

	var got bool
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		_, got = TLS(r)
	})

	c := fakeWiredContext(t, func(fc *mock_envoy.FilterCallbackHandler) {
		fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{})
	})
	f := newBridgeFilter(c, h)

	m := fakeBridgeHeaders(t, "")
	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	assert.False(t, got)
}

func TestSnapshotTLSMapping(t *testing.T) {
	t.Parallel()

	got := snapshotTLS(fakeSslConnection{})

	assert.True(t, got.PeerCertificatePresented)
	assert.True(t, got.PeerCertificateValidated)
	assert.Equal(t, "ABCD", got.Sha256PeerCertificateDigest)
	assert.Equal(t, "CN=test", got.SubjectPeerCertificate)
	assert.Equal(t, []string{"spiffe://test"}, got.URISANsPeerCertificate)
	assert.Equal(t, "TLSv1.3", got.TLSVersion)
	require.NotNil(t, got.ValidFromPeerCertificate)
	assert.Equal(t, uint64(1700000000), *got.ValidFromPeerCertificate)
	assert.Nil(t, got.ExpirationPeerCertificate)
	require.NotNil(t, got.CiphersuiteID)
	assert.Equal(t, uint16(47), *got.CiphersuiteID)
	assert.Nil(t, got.SessionID)
	assert.Equal(t, "PEM", got.PEMPeerCertificate)
}
