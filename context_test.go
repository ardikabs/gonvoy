package gonvoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func fakeDummyContext(t *testing.T, opts ...ContextOption) Context {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	c, err := NewContext(fc, opts...)
	require.NoError(t, err)
	return c
}

func TestContext(t *testing.T) {

	t.Run("Request Body Accessibility", func(t *testing.T) {
		testcases := []struct {
			name           string
			config         *globalConfig
			contentType    string
			contentLength  string
			expectedAccess bool
			expectedRead   bool
			expectedWrite  bool
		}{
			{
				name: "non-writeable| content-type: application/json, content-length: 100",
				config: &globalConfig{
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
				config: &globalConfig{
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
				config: &globalConfig{
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
				config: &globalConfig{
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
				config: &globalConfig{
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
				config: &globalConfig{
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
				config: &globalConfig{
					strictBodyAccess:     false,
					allowRequestBodyRead: true,
				},
				contentType:    "application/grpc+proto",
				expectedAccess: false,
				expectedRead:   false,
				expectedWrite:  false,
			},
			{
				name: "body accessible for non-grpc| content-type: application/grpc-web, content-length: 100",
				config: &globalConfig{
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

				ctx := fakeDummyContext(t, WithContextConfig(tc.config))
				ctx.SetRequestHeader(reqHeaderMapMock)

				assert.Equal(t, tc.expectedAccess, ctx.IsRequestBodyAccessible())
				assert.Equal(t, tc.expectedRead, ctx.IsRequestBodyReadable())
				assert.Equal(t, tc.expectedWrite, ctx.IsRequestBodyWritable())
			})
		}
	})
}
