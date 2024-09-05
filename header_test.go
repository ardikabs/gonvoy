package gaetway

import (
	"net/http"
	"strings"
	"testing"

	mock_envoy "github.com/ardikabs/gaetway/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeHeaderMap struct {
	api.RequestHeaderMap

	data map[string][]string
}

func (d *fakeHeaderMap) Get(key string) (string, bool) {
	if v, ok := d.data[strings.ToLower(key)]; ok {
		return v[0], true
	}

	return "", false
}

func (d *fakeHeaderMap) Set(key, value string) {
	d.data[strings.ToLower(key)] = []string{value}
}

func (d *fakeHeaderMap) Add(key, value string) {
	d.data[strings.ToLower(key)] = append(d.data[strings.ToLower(key)], value)
}

func (d *fakeHeaderMap) Range(f func(string, string) bool) {
	for k, values := range d.data {
		for _, v := range values {
			if !f(strings.ToLower(k), v) {
				return
			}
		}
	}
}

func TestHeader_Export(t *testing.T) {
	fakeHeaderMap := &fakeHeaderMap{
		data: map[string][]string{
			"foo":       {"bar"},
			"x-foo":     {"x-bar", "x-foobar"},
			"x-foo-bar": {"x-foo-bar,x-foo-bar-2"},
		},
	}

	h := &header{HeaderMap: fakeHeaderMap}
	assert.NotNil(t, h)

	headers := h.Export()
	assert.Equal(t, "bar", headers.Get("foo"))
	assert.Equal(t, []string{"x-bar", "x-foobar"}, headers.Values("x-foo"))
	assert.Equal(t, "x-foo-bar,x-foo-bar-2", headers.Get("x-foo-bar"))
}

func TestHeader_Import(t *testing.T) {
	fakeHeaderMap := &fakeHeaderMap{
		data: map[string][]string{
			"foo":       {"bar"},
			"x-foo":     {"x-bar", "x-foobar"},
			"x-foo-bar": {"x-foo-bar,x-foo-bar-2"},
		},
	}

	h := &header{HeaderMap: fakeHeaderMap}
	assert.NotNil(t, h)

	headers := h.Export()
	assert.NotEmpty(t, headers)

	headers.Add("boo", "far")
	headers.Add("foo", "foobar")
	headers.Set("x-foo-bar", "bar-foo-x")

	h.Import(headers)

	if v, ok := h.Get("boo"); ok {
		assert.Equal(t, "far", v)
	}

	if v, ok := h.Get("foo"); ok {
		assert.Equal(t, "bar", v)
	}

	if v, ok := h.Get("x-foo-bar"); ok {
		assert.Equal(t, "bar-foo-x", v)
	}
}

func TestNewGatewayHeaders(t *testing.T) {
	t.Run("ideal", func(t *testing.T) {
		headers := NewGatewayHeaders()

		assert.NotEmpty(t, headers)
		assert.Equal(t, "gateway", headers.Get("reporter"))
	})

	t.Run("additional headers, with a valid argument", func(t *testing.T) {
		headers := NewGatewayHeaders(
			"foo", "bar",
			"foobar", "barfoo",
			"loo", "bar",
		)

		assert.NotEmpty(t, headers)
		assert.Equal(t, "gateway", headers.Get("reporter"))
		assert.Equal(t, "bar", headers.Get("foo"))
		assert.Equal(t, "barfoo", headers.Get("foobar"))
		assert.Equal(t, "bar", headers.Get("loo"))
	})

	t.Run("expect additional headers, but argument is not valid because it contains odd arguments", func(t *testing.T) {
		headers := NewGatewayHeaders(
			"foo", "bar",
			"foobar",
		)

		assert.Len(t, headers, 1)
		assert.Equal(t, "", headers.Get("foo"))
		assert.Equal(t, "", headers.Get("foobar"))
	})

	t.Run("new gateway headers with envoy header", func(t *testing.T) {
		reqHeaderMapMock := mock_envoy.NewRequestHeaderMap(t)
		reqHeaderMapMock.EXPECT().Host().Return("foo.bar.com")
		reqHeaderMapMock.EXPECT().Method().Return(http.MethodGet)
		reqHeaderMapMock.EXPECT().Path().Return("/foo/bar")
		reqHeaderMapMock.EXPECT().Range(mock.Anything).Return().Run(func(f func(string, string) bool) {
			headers := map[string][]string{
				"x-request-id": {"asdf12345"},
				"x-foo":        {"x-bar", "x-foobar"},
				"x-foo-bar":    {"x-foo-bar,x-foo-bar-2"},
			}
			for k, values := range headers {
				for _, v := range values {
					f(k, v)
				}
			}
		})

		ctx := fakeDummyContext(t, nil)
		ctx.LoadRequestHeaders(reqHeaderMapMock)

		headers := NewGatewayHeadersWithEnvoyHeader(ctx.RequestHeader())

		assert.Equal(t, "gateway", headers.Get("reporter"))
		assert.Equal(t, "asdf12345", headers.Get("x-request-id"))
	})
}
