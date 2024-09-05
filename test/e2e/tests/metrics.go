//go:build e2e
// +build e2e

package tests

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ardikabs/gaetway/pkg/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, MetricsTestCase)
}

var MetricsTestCase = suite.TestCase{
	Name:        "MetricsTest",
	FilterName:  "metrics",
	Description: "Generate user-generated metrics from filter.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		t.Cleanup(stop)

		t.Run("counter metrics on all requests", func(t *testing.T) {
			t.Parallel()

			expectedCalls := 10

			var actualCall int
			require.Eventually(t, func() bool {
				resp, err := http.Get(kit.GetEnvoyHost() + "/get")
				require.NoError(t, err)
				defer resp.Body.Close()
				actualCall++
				return expectedCalls == actualCall
			}, kit.WaitDuration, kit.TickDuration, "failed to generate metrics")

			require.Eventually(t, func() bool {
				resp, err := http.Get(kit.GetAdminHost() + "/stats/prometheus")
				require.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				expectedPayload := fmt.Sprintf("envoy_mymetrics_requests_total{host=\"localhost\",method=\"get\",response_code=\"200\",upstream_name=\"staticreply\",route_name=\"staticreply\"} %d", expectedCalls)
				return strings.Contains(string(body), expectedPayload)
			}, kit.WaitDuration, kit.TickDuration, "failed to find the metrics")
		})

		t.Run("counter metrics measured based on header", func(t *testing.T) {
			t.Parallel()

			expectedCalls := 10

			var actualCall int

			require.Eventually(t, func() bool {
				req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
				require.NoError(t, err)

				req.Header.Set("x-metric-counter-on", "foobar")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				actualCall++
				return expectedCalls == actualCall
			}, kit.WaitDuration, kit.TickDuration, "failed to generate metrics based on header")

			require.Eventually(t, func() bool {
				resp, err := http.Get(kit.GetAdminHost() + "/stats/prometheus")
				require.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				expectedPayload := fmt.Sprintf("envoy_mymetrics_header_appears_total{header_value=\"foobar\",reporter=\"gonvoy\"} %d", expectedCalls)

				return strings.Contains(string(body), expectedPayload)
			}, kit.WaitDuration, kit.TickDuration, "failed to find the metrics")
		})
	},
}
