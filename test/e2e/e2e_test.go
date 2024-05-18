//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	EnvoyVersion = "envoyproxy/envoy:contrib-v1.29-latest"
)

func TestHelloworld(t *testing.T) {
	adminport := 8001
	envoyport := 10001

	stdErr, kill := runEnvoy(t, "helloworld", adminport, envoyport)
	defer kill()

	req, err := http.NewRequest("GET", getEnvoyHost(envoyport), nil)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Eventually(t, func() bool {
		return checkEnvoyAccessLog(stdErr.String(), "Hello World from the helloworld HTTP filter")
	}, 5*time.Second, 100*time.Millisecond, stdErr.String())
}

func TestHttpHeadersModifier(t *testing.T) {
	adminport := 8002
	envoyport := 10002
	stdErr, kill := runEnvoy(t, "http_headers_modifier", adminport, envoyport)
	defer kill()

	t.Run("index route | config on parent", func(t *testing.T) {
		req, err := http.NewRequest("GET", getEnvoyHost(envoyport), nil)
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, res.Header.Get("x-header-modified-at"), "parent")
		require.Eventually(t, func() bool {
			return checkEnvoyAccessLog(stdErr.String(), "request header ---> X-Foo=[\"bar\"]")
		}, 5*time.Second, 100*time.Millisecond, stdErr.String())
	})

	t.Run("details route | config on child", func(t *testing.T) {
		req, err := http.NewRequest("GET", getEnvoyHost(envoyport)+"/details", nil)
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, res.Header.Get("x-header-modified-at"), "child")
		require.Eventually(t, func() bool {
			return checkEnvoyAccessLog(stdErr.String(), "request header ---> X-Boo=[\"far\"]")
		}, 5*time.Second, 100*time.Millisecond, stdErr.String())
	})
}

func runEnvoy(t *testing.T, target string, adminPort, envoyPort int) (stdErr *bytes.Buffer, kill func()) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	targetdir := fmt.Sprintf("%s/%s", cwd, target)

	cmd := exec.Command("docker",
		"run",
		"--rm",
		"-p", fmt.Sprintf("%d:8000", adminPort),
		"-p", fmt.Sprintf("%d:10000", envoyPort),
		"-v", targetdir+"/envoy.yaml:/etc/envoy.yaml",
		"-v", targetdir+"/filter.so:/filter.so",
		EnvoyVersion,
		"/usr/local/bin/envoy",
		"-c", "/etc/envoy.yaml",
		"--log-level", "warn",
		"--component-log-level", "main:critical,misc:critical,golang:info",
		"--concurrency", "1",
	)

	buf := new(bytes.Buffer)
	cmd.Stderr = buf
	require.NoError(t, cmd.Start())
	if !assert.Eventually(t, func() bool {
		res, err := http.Get(fmt.Sprintf("http://localhost:%d/listeners", adminPort))
		if err != nil {
			return false
		}

		defer res.Body.Close()

		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 100*time.Millisecond, "Envoy failed to start") {
		t.Fatalf("Envoy stderr: %s", buf.String())
	}

	return buf, func() { require.NoError(t, cmd.Process.Signal(syscall.SIGINT)) }
}

func checkEnvoyAccessLog(out string, expressions ...string) bool {
	for _, exp := range expressions {
		if strings.Contains(out, exp) {
			return true
		}
	}

	return false
}

func getEnvoyHost(envoyPort int) string {
	return fmt.Sprintf("http://localhost:%d", envoyPort)
}
