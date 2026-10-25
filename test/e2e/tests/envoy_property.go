//go:build e2e
// +build e2e

package tests

import (
	"net/http"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, EnvoyPropertyTestCase)
}

var EnvoyPropertyTestCase = suite.TestCase{
	Name:        "EnvoyPropertyTest",
	FilterName:  "envoy_property",
	Description: "Verify internal Envoy attributes are exposed via Attr and echo back as response headers.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
		require.NoError(t, err)

		req.Host = "envoy-property.test"

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "envoy-property.test", resp.Header.Get("x-request-host"))
		require.Equal(t, http.MethodGet, resp.Header.Get("x-request-method"))
		require.Equal(t, "/", resp.Header.Get("x-request-path"))
		require.Equal(t, "staticreply", resp.Header.Get("x-xds-cluster"))
		require.Equal(t, "property-route", resp.Header.Get("x-xds-route"))
		require.Equal(t, "200", resp.Header.Get("x-response-code"))
		require.Equal(t, "fallback", resp.Header.Get("x-missing-value"))
		require.Equal(t, "false", resp.Header.Get("x-missing-ok"))

		require.Eventually(t, func() bool {
			return kit.CheckEnvoyLog("envoy property response attrs")
		}, kit.WaitDuration, kit.TickDuration, "failed to find envoy property log message in envoy log")
	},
}
