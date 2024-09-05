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
	TestCases = append(TestCases, HTTPRerouteTestCase)
}

var HTTPRerouteTestCase = suite.TestCase{
	Name:        "HTTPRerouteTest",
	FilterName:  "http_reroute",
	Description: "Simulate on how to reroute HTTP request based on the request header dynamically using filter",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("good response", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/")
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
		})

		t.Run("route to root.staticreply", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("x-route-to", "staticreply")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, "staticreply", resp.Header.Get("x-response-by"))
			require.Equal(t, "index", resp.Header.Get("x-response-path-name"))
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("route to path.staticreply", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("x-route-to", "staticreply")
			req.Header.Set("x-path-changed-to", "staticreply")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, "staticreply", resp.Header.Get("x-response-by"))
			require.Equal(t, "staticreply", resp.Header.Get("x-response-path-name"))
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("route to staticreply.svc", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/", nil)
			require.NoError(t, err)

			req.Header.Set("x-route-to", "staticreply")
			req.Header.Set("x-changed-host", "true")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, "staticreply.svc", resp.Header.Get("x-response-host"))
			require.Equal(t, "staticreply", resp.Header.Get("x-response-by"))
			require.Equal(t, "index", resp.Header.Get("x-response-path-name"))
			require.Equal(t, http.StatusCreated, resp.StatusCode)
		})
	},
}
