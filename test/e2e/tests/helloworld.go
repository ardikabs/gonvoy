//go:build e2e
// +build e2e

package tests

import (
	"net/http"
	"testing"

	"github.com/ardikabs/gaetway/pkg/suite"
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
	},
}
