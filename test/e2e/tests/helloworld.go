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
	TestCases = append(TestCases, HelloWorldTestCase)
}

var HelloWorldTestCase = suite.TestCase{
	Name:        "HelloWorldTest",
	FilterName:  "helloworld",
	Description: "Initial check to see framework working properly.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		resp, err := http.Get(kit.GetEnvoyHost())
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Eventually(t, func() bool {
			return kit.CheckEnvoyLog("Hello World from the helloworld HTTP filter")
		}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")

		t.Run("drain connection on request", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("x-drain", "true")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.True(t, resp.Close, "expected the connection to be marked for drain")
		})

	},
}
