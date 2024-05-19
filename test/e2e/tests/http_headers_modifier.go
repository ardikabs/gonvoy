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
	TestCases = append(TestCases, HttpHeadersModifierTestCase)
}

var HttpHeadersModifierTestCase = suite.TestCase{
	Name:        "HTTPHeadersModifierTest",
	FilterName:  "http_headers_modifier",
	Description: "Running test to simulate HTTP headers modification both Request and Response, while also showing how to use child config for a specific route.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		kill := kit.StartEnvoy(t)
		defer kill()

		t.Run("invoke to index route", func(t *testing.T) {
			req, err := http.NewRequest("GET", kit.GetEnvoyHost(), nil)
			require.NoError(t, err)

			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			require.Equal(t, res.Header.Get("x-header-modified-at"), "parent")
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("request header ---> X-Foo=[\"bar\"]")
			}, kit.DefaultWaitDuration, kit.DefaultTickDuration, "failed to find log message in envoy log")
		})

		t.Run("invoke to details route, expect to use child config", func(t *testing.T) {
			req, err := http.NewRequest("GET", kit.GetEnvoyHost()+"/details", nil)
			require.NoError(t, err)

			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			require.Equal(t, res.Header.Get("x-header-modified-at"), "child")
			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("request header ---> X-Boo=[\"far\"]")
			}, kit.DefaultWaitDuration, kit.DefaultTickDuration, "failed to find log message in envoy log")
		})
	},
}
