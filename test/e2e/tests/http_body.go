//go:build e2e
// +build e2e

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/stretchr/testify/require"
)

func init() {
	TestCases = append(TestCases, HttpBodyTestCase)
}

var HttpBodyTestCase = suite.TestCase{
	Name:        "HTTPBodyTest",
	FilterName:  "http_body",
	Description: "Running test to simulate HTTP body modification both Request and Response. This test run with DisableStrictBodyAccess option enabled.",
	Parallel:    true,
	Test: func(t *testing.T, kit *suite.TestSuiteKit) {
		stop := kit.StartEnvoy(t)
		defer stop()

		t.Run("ReadOnly - inspect request payload", func(t *testing.T) {
			require.Eventually(t, func() bool {
				req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/listeners", bytes.NewReader([]byte("foo bar")))
				require.NoError(t, err)

				req.Header.Set("content-type", gonvoy.MIMEApplicationForm)
				req.Header.Set("x-inspect-body", "true")
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				return kit.CheckEnvoyLog(`request body payload ---> data="foo bar" mode=READ`)
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log: %s", kit.ShowEnvoyLog())
		})

		t.Run("ReadOnly - try to write on request, got 502", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/listeners", bytes.NewReader([]byte("foo bar")))
			require.NoError(t, err)

			req.Header.Set("content-type", gonvoy.MIMEApplicationForm)
			req.Header.Set("x-inspect-body", "true")
			req.Header.Set("x-try-write", "request")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		})

		t.Run("ReadOnly - inspect response payload", func(t *testing.T) {
			require.Eventually(t, func() bool {
				req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/server_info", nil)
				require.NoError(t, err)

				req.Header.Set("x-inspect-body", "true")
				req.Header.Set("x-mode", "READ")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				return kit.CheckEnvoyLog("response body payload ---> mode=READ state=LIVE")
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")
		})

		t.Run("ReadOnly - try to write on response, got 502", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/server_info", nil)
			require.NoError(t, err)

			req.Header.Set("x-inspect-body", "true")
			req.Header.Set("x-try-write", "response")
			req.Header.Set("x-mode", "READ")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		})

		t.Run("ReadWrite - modify request payload", func(t *testing.T) {
			signature := strconv.Itoa(int(time.Now().UnixMilli()))
			req, err := http.NewRequest(http.MethodPost, kit.GetEnvoyHost()+"/server_info", bytes.NewReader([]byte("foo bar")))
			require.NoError(t, err)

			req.Header.Set("content-type", gonvoy.MIMEApplicationForm)
			req.Header.Set("x-modify-body", "true")
			req.Header.Set("x-signature", signature)
			req.Header.Set("x-mode", "WRITE")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Eventually(t, func() bool {
				return kit.CheckEnvoyLog(fmt.Sprintf(`request payload should be modified ---> data="foo bar" signature=%s`, signature))
			}, kit.WaitDuration, kit.TickDuration, "failed to find log message in envoy log")
		})

		t.Run("ReadWrite - modify response payload", func(t *testing.T) {
			signature := strconv.Itoa(int(time.Now().UnixMilli()))
			req, err := http.NewRequest(http.MethodGet, kit.GetEnvoyHost()+"/server_info", nil)
			require.NoError(t, err)

			req.Header.Set("x-modify-body", "true")
			req.Header.Set("x-signature", signature)
			req.Header.Set("x-mode", "WRITE")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			payload := make(map[string]interface{})
			require.NoError(t, json.Unmarshal(bodyBytes, &payload))

			require.Equal(t, signature, payload["signature"])
			require.Contains(t, payload, "isModified")
			require.Contains(t, payload, "modifiedAt")
		})
	},
}
