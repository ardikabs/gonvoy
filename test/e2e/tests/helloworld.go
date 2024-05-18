//go:build e2e
// +build e2e

package tests

import (
	"net/http"
	"testing"

	"github.com/ardikabs/gonvoy/test/e2e/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, HelloWorldTestCase)
}

var HelloWorldTestCase = suite.TestCase{
	Name:        "HelloWorldTest",
	FilterName:  "helloworld",
	Description: "Initial check to see framework working properly.",
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		kill := kit.StartEnvoy(t)
		defer kill()

		req, err := http.NewRequest("GET", kit.GetEnvoyHost(), nil)
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Eventually(t, func() bool {
			return kit.CheckEnvoyLog("Hello World from the helloworld HTTP filter")
		}, kit.DefaultWaitDuration, kit.DefaultTickDuration, "failed to find log message in envoy log")
	},
}
