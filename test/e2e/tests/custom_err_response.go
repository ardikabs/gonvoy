//go:build e2e
// +build e2e

package tests

import (
	"io"
	"net/http"
	"testing"

	"github.com/ardikabs/gaetway/pkg/suite"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func init() {
	TestCases = append(TestCases, CustomErrResponseTestCase)
}

var CustomErrResponseTestCase = suite.TestCase{
	Name:        "CustomErrResponseTest",
	FilterName:  "custom_err_response",
	Description: "Override a considered error response with custom response payload. It overrides based on status codes, including 401, 429, and 503.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("401 Response", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/401")
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, payload.Get("code").Str, "UNAUTHORIZED")
			require.Equal(t, payload.Get("message").Str, "UNAUTHORIZED")
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("429 Response", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/429")
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, payload.Get("code").Str, "TOO_MANY_REQUESTS")
			require.Equal(t, payload.Get("message").Str, "TOO_MANY_REQUESTS")
			require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		})

		t.Run("503 Response", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/503")
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, payload.Get("code").Str, "SERVICE_UNAVAILABLE")
			require.Equal(t, payload.Get("message").Str, "SERVICE_UNAVAILABLE")
			require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		})
	},
}
