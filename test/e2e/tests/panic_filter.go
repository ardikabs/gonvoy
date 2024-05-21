//go:build e2e
// +build e2e

package tests

import (
	"io"
	"net/http"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func init() {
	TestCases = append(TestCases, PanicFilterTestCase)
}

var PanicFilterTestCase = suite.TestCase{
	Name:        "PanicFilterTest",
	FilterName:  "panic_filter",
	Description: "Simulate panic in the filter and return 500 response.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("panic on request", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("x-panic-at", "header")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, "RUNTIME_ERROR", payload.Get("code").Str)
			require.Equal(t, "RUNTIME_ERROR", payload.Get("message").Str)
			require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("panic during request header handling")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")
		})

		t.Run("panic on response", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/panic")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := gjson.ParseBytes(body)

			require.Equal(t, "RUNTIME_ERROR", payload.Get("code").Str)
			require.Equal(t, "RUNTIME_ERROR", payload.Get("message").Str)
			require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("panic during response header handling")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")

		})
	},
}
