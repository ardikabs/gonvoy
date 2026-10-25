//go:build e2e
// +build e2e

package tests

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, HttpQueryValidationTestCase)
}

var HttpQueryValidationTestCase = suite.TestCase{
	Name:        "HttpQueryValidationTest",
	FilterName:  "http_query_validation",
	Description: "Validate HTTP query parameters on GET /search using QueryParam and Path.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("valid q returns 200", func(t *testing.T) {
			values := url.Values{}
			values.Set("q", "golang")
			values.Set("mode", "READ")
			values.Set("limit", "10")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("query validated q=golang path=/search")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log: %s", kit.ShowEnvoyLog())
		})

		t.Run("missing q returns 400", func(t *testing.T) {
			values := url.Values{}
			values.Set("mode", "READ")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("empty q returns 400", func(t *testing.T) {
			values := url.Values{}
			values.Set("q", "")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("invalid mode returns 400", func(t *testing.T) {
			values := url.Values{}
			values.Set("q", "golang")
			values.Set("mode", "DELETE")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("invalid limit returns 400", func(t *testing.T) {
			for _, limit := range []string{"0", "101", "abc"} {
				values := url.Values{}
				values.Set("q", "golang")
				values.Set("limit", limit)

				req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				resp.Body.Close()

				require.Equal(t, http.StatusBadRequest, resp.StatusCode, "limit=%s should be rejected", limit)
			}
		})

		t.Run("multi-value tag returns first value", func(t *testing.T) {
			// QueryParam uses r.URL.Query().Get which returns the first value.
			values := url.Values{}
			values.Set("q", "multitag")
			values.Add("tag", "a")
			values.Add("tag", "b")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("tag=a")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log: %s", kit.ShowEnvoyLog())
		})

		t.Run("encoded q decodes", func(t *testing.T) {
			values := url.Values{}
			values.Set("q", "hello world")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			// Same payload with explicit %20 encoding.
			rawReq, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?q=hello%20world", nil)
			require.NoError(t, err)

			rawResp, err := http.DefaultClient.Do(rawReq)
			require.NoError(t, err)
			defer rawResp.Body.Close()

			require.Equal(t, http.StatusOK, rawResp.StatusCode)
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("hello world")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log: %s", kit.ShowEnvoyLog())
		})

		t.Run("unknown extra params ignored", func(t *testing.T) {
			values := url.Values{}
			values.Set("q", "extra")
			values.Set("foo", "bar")
			values.Set("unknown", "1")

			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/search?"+values.Encode(), nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("query validated q=extra path=/search")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log: %s", kit.ShowEnvoyLog())
		})
	},
}
