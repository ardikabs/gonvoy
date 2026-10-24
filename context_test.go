package gonvoy

import (
	"errors"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func fakeDummyContext(t *testing.T, config *internalConfig) *context {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	c, err := NewContext(fc, contextOptions{config: config})
	if errors.Is(err, errInternalConfigNotFound) {
		return mustCastToContext(c)
	}

	require.NoError(t, err)
	return mustCastToContext(c)
}

func TestBridgeContextSurface(t *testing.T) {
	// Compile pin: the bridge must compile against exactly this surface.
	// Method expressions (not values on a nil interface) keep it runtime-safe.
	_ = Context.LoadRequestHeaders
	_ = Context.LoadResponseHeaders
	_ = Context.SendResponse
	_ = Context.ReloadRoute
	_ = Context.IsRequestBodyReadable
	_ = Context.IsRequestBodyWritable
	_ = Context.IsResponseBodyReadable
	_ = Context.IsResponseBodyWritable
	// Kept beyond the brief: DefaultErrorHandler answers through JSON.
	_ = Context.JSON
}

func TestContext(t *testing.T) {
	t.Run("Request Body Accessibility", func(t *testing.T) {
		testcases := []struct {
			name           string
			config         *internalConfig
			contentType    string
			contentLength  string
			expectedAccess bool
			expectedRead   bool
			expectedWrite  bool
		}{
			{
				name: "non-writeable| content-type: application/json, content-length: 100",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    MIMEApplicationJSON,
				expectedAccess: true,
				expectedRead:   true,
				expectedWrite:  false,
			},
			{
				name: "body accessible| content-type: application/json, content-length: 100",
				config: &internalConfig{
					strictBodyAccess:      false,
					allowRequestBodyWrite: true,
				},
				contentType:    MIMEApplicationJSON,
				contentLength:  "100",
				expectedAccess: true,
				expectedRead:   true,
				expectedWrite:  true,
			},
			{
				name: "body accessible| content-type: application/json and data chunked",
				config: &internalConfig{
					strictBodyAccess:      false,
					allowRequestBodyWrite: true,
				},
				contentType:    MIMEApplicationJSON,
				expectedAccess: true,
				expectedRead:   true,
				expectedWrite:  true,
			},
			{
				name: "body accessible| content-type: application/ld+json, content-length: 100",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    "application/ld+json",
				contentLength:  "100",
				expectedAccess: true,
				expectedRead:   true,
				expectedWrite:  false,
			},
			{
				name: "body inaccessible| content-type: application/ld+json and data chunked (no content-length)",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    "application/ld+json",
				expectedAccess: false,
				expectedRead:   false,
				expectedWrite:  false,
			},
			{
				name: "body inaccessible| content-type: application/grpc",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    MIMEApplicationGRPC,
				expectedAccess: false,
				expectedRead:   false,
				expectedWrite:  false,
			},
			{
				name: "body inaccessible| content-type: application/grpc+",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    "application/grpc+proto",
				contentLength:  "100",
				expectedAccess: false,
				expectedRead:   false,
				expectedWrite:  false,
			},
			{
				name: "body accessible for non-grpc| content-type: application/grpc-web, content-length: 100",
				config: &internalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    "application/grpc-web",
				contentLength:  "100",
				expectedAccess: true,
				expectedRead:   true,
				expectedWrite:  false,
			},
		}

		for _, tc := range testcases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				reqHeaderMapMock := mock_envoy.NewRequestHeaderMap(t)
				reqHeaderMapMock.EXPECT().Host().Return("example.com")
				reqHeaderMapMock.EXPECT().Method().Return("GET")
				reqHeaderMapMock.EXPECT().Path().Return("/")
				reqHeaderMapMock.EXPECT().Range(mock.Anything).Run(func(f func(string, string) bool) {
					headers := map[string]string{
						"content-type":   tc.contentType,
						"content-length": tc.contentLength,
					}

					for k, v := range headers {
						f(k, v)
					}
				})

				ctx := fakeDummyContext(t, tc.config)
				ctx.LoadRequestHeaders(reqHeaderMapMock)

				assert.Equal(t, tc.expectedAccess, ctx.IsRequestBodyAccessible())
				assert.Equal(t, tc.expectedRead, ctx.IsRequestBodyReadable())
				assert.Equal(t, tc.expectedWrite, ctx.IsRequestBodyWritable())
			})
		}
	})
}

func TestContext_Property(t *testing.T) {
	testcases := []struct {
		name        string
		value       string
		valueErr    error
		defaultVal  string
		expected    string
		expectError bool
	}{
		{
			name:     "found value",
			value:    "example.com",
			expected: "example.com",
		},
		{
			name:       "not found returns default",
			valueErr:   api.ErrValueNotFound,
			defaultVal: "-",
			expected:   "-",
		},
		{
			name:       "empty value returns default",
			defaultVal: "-",
			expected:   "-",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fc := mock_envoy.NewFilterCallbackHandler(t)
			fc.EXPECT().GetProperty("request.host").Return(tc.value, tc.valueErr)

			c, err := NewContext(fc, contextOptions{config: &internalConfig{}})
			if !errors.Is(err, errInternalConfigNotFound) {
				require.NoError(t, err)
			}

			got, err := c.Property("request.host", tc.defaultVal)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
