//go:build e2e
// +build e2e

package tests

import (
	"io"
	"net/http"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, SharedCacheTestCase)
}

// SharedCacheTestCase verifies the filter-wide cache persists across requests.
var SharedCacheTestCase = suite.TestCase{
	Name:        "SharedCacheTest",
	FilterName:  "shared_cache",
	Description: "Running test to simulate shared cache globally throughout Envoy's lifespan.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("store then load persists across requests", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/store?key=persist-key&value=hello", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("shared cache store")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")

			loadResp, err := http.Get(kit.GetEnvoyHost() + "/load?key=persist-key")
			require.NoError(t, err)
			defer loadResp.Body.Close()

			body, err := io.ReadAll(loadResp.Body)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, loadResp.StatusCode)
			require.Equal(t, "hello", string(body))

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("shared cache hit")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")
		})

		t.Run("overwrite existing key", func(t *testing.T) {
			store := func(value string) {
				req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/store?key=overwrite-key&value="+value, nil)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, http.StatusOK, resp.StatusCode)
			}

			load := func() (int, string) {
				resp, err := http.Get(kit.GetEnvoyHost() + "/load?key=overwrite-key")
				require.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				return resp.StatusCode, string(body)
			}

			store("v1")

			code, body := load()
			require.Equal(t, http.StatusOK, code)
			require.Equal(t, "v1", body)

			store("v2")

			code, body = load()
			require.Equal(t, http.StatusOK, code)
			require.Equal(t, "v2", body)
		})

		t.Run("miss returns 404", func(t *testing.T) {
			resp, err := http.Get(kit.GetEnvoyHost() + "/load?key=missing-key")
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("type mismatch returns 500", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/store-struct?key=struct-key", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			badResp, err := http.Get(kit.GetEnvoyHost() + "/load-struct-bad?key=struct-key")
			require.NoError(t, err)
			defer badResp.Body.Close()

			require.Equal(t, http.StatusInternalServerError, badResp.StatusCode)

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog("shared cache load type mismatch")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")
		})
	},
}
