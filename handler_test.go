package gonvoy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerFunc_ServesHTTP(t *testing.T) {
	t.Parallel()

	var gotW ResponseWriter
	var gotR *http.Request
	want, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)

	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		gotW = w
		gotR = r
	})

	w := newRecordingWriter()
	h.ServeHTTP(w, want)

	assert.Same(t, w, gotW)
	assert.Same(t, want, gotR)
}

func TestMiddleware_Order(t *testing.T) {
	t.Parallel()

	var order []string
	mk := func(name string) func(Handler) Handler {
		return func(next Handler) Handler {
			return HandlerFunc(func(w ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				next.ServeHTTP(w, r)
				order = append(order, name+":after")
			})
		}
	}

	final := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		order = append(order, "final")
	})

	h := mk("outer")(mk("inner")(final))
	h.ServeHTTP(newRecordingWriter(), &http.Request{})

	assert.Equal(t, []string{"outer:before", "inner:before", "final", "inner:after", "outer:after"}, order)
}

func TestRecordingWriter(t *testing.T) {
	t.Run("write header and body", func(t *testing.T) {
		t.Parallel()

		w := newRecordingWriter()
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusCreated)
		n, err := w.Write([]byte("hi"))

		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, http.StatusCreated, w.status)
		assert.Equal(t, "1", w.Header().Get("X-Test"))
		assert.Equal(t, "hi", string(w.body))
		assert.True(t, w.wroteHeader)
	})

	t.Run("first write header wins", func(t *testing.T) {
		t.Parallel()

		w := newRecordingWriter()
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusCreated, w.status)
	})

	t.Run("write implies 200", func(t *testing.T) {
		t.Parallel()

		w := newRecordingWriter()
		_, _ = w.Write([]byte("hi"))

		assert.Equal(t, http.StatusOK, w.status)
	})
}

func TestConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	type myConfig struct{ Allow bool }

	t.Run("typed config", func(t *testing.T) {
		t.Parallel()

		r := withBridgeConfig(&http.Request{}, &myConfig{Allow: true})
		got, err := Config[*myConfig](r)

		require.NoError(t, err)
		assert.True(t, got.Allow)
	})

	t.Run("missing config errors", func(t *testing.T) {
		t.Parallel()

		_, err := Config[*myConfig](&http.Request{})

		require.Error(t, err)
		assert.ErrorIs(t, err, errInternalConfigNotFound)
	})

	t.Run("mismatched type errors", func(t *testing.T) {
		t.Parallel()

		r := withBridgeConfig(&http.Request{}, "not-a-struct")
		_, err := Config[*myConfig](r)

		require.Error(t, err)
	})
}

func TestAttr(t *testing.T) {
	t.Parallel()

	r := withBridgeHooks(&http.Request{}, bridgeHooks{
		attr: func(name string) (string, bool) {
			if name == "xds.cluster_name" {
				return "test-cluster", true
			}
			return "", false
		},
	})

	v, ok := Attr(r, "xds.cluster_name", "-")
	assert.True(t, ok)
	assert.Equal(t, "test-cluster", v)

	v, ok = Attr(r, "xds.route_name", "-")
	assert.False(t, ok)
	assert.Equal(t, "-", v)
}

func TestReroute_SetsFlag(t *testing.T) {
	t.Parallel()

	var rerouted bool
	r := withBridgeHooks(&http.Request{}, bridgeHooks{
		reroute: func() { rerouted = true },
	})

	Reroute(r)

	assert.True(t, rerouted)
}

func TestMetricsRecorder_Queues(t *testing.T) {
	t.Parallel()

	var ops []MetricOp
	m := MetricsRecorder{emit: func(op MetricOp) { ops = append(ops, op) }}

	m.Counter("requests_total", "host", "example.com").Increment(1)
	m.Counter("requests_total", "host", "example.com").Increment(2)
	m.Gauge("inflight").Set(3)

	require.Len(t, ops, 3)
	assert.Equal(t, MetricOp{Kind: MetricCounterAdd, Name: "requests_total", Labels: []string{"host", "example.com"}, Delta: 1}, ops[0])
	assert.Equal(t, int64(2), ops[1].Delta)
	assert.Equal(t, MetricOp{Kind: MetricGaugeSet, Name: "inflight", Value: 3}, ops[2])
}

func TestNewHelpers_OutsideBridge(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)

	v, ok := Secret(r, "db")
	assert.False(t, ok)
	assert.Equal(t, "", v)

	_, ok = TLS(r)
	assert.False(t, ok)

	assert.NotPanics(t, func() {
		OverrideUpstreamHost(r, "10.0.0.9:8080", true)
		DrainConnection(r)
		RefreshRouteCache(r.Context())
	})
}

func TestAttr_MissingKeyReturnsDefault(t *testing.T) {
	t.Parallel()

	r := withBridgeHooks(&http.Request{}, bridgeHooks{
		attr: func(name string) (string, bool) { return "", false },
	})

	v, ok := Attr(r, "xds.cluster_name", "-")

	assert.False(t, ok)
	assert.Equal(t, "-", v)
}
