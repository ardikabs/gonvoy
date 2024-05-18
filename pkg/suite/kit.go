package suite

import (
	"bytes"
	"fmt"
	"net"
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

type TestSuiteKit struct {
	DefaultWaitDuration time.Duration
	DefaultTickDuration time.Duration

	filterName        string
	envoyImageVersion string
	envoyPort         int
	adminPort         int
	envoyLogBuffer    *bytes.Buffer
}

func (s *TestSuiteKit) StartEnvoy(t *testing.T) (kill func()) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	filterdir := fmt.Sprintf("%s/filters/%s", cwd, s.filterName)

	cmd := exec.Command("docker",
		"run",
		"--rm",
		"-p", fmt.Sprintf("%d:8000", s.adminPort),
		"-p", fmt.Sprintf("%d:10000", s.envoyPort),
		"-v", filterdir+"/envoy.yaml:/etc/envoy.yaml",
		"-v", filterdir+"/filter.so:/filter.so",
		s.envoyImageVersion,
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
		res, err := http.Get(s.GetAdminHost() + "/listeners")
		if err != nil {
			return false
		}

		defer res.Body.Close()

		return res.StatusCode == http.StatusOK
	}, s.DefaultWaitDuration, s.DefaultTickDuration, "Envoy failed to start") {
		t.Fatalf("Envoy startup: %s", buf.String())
	}

	if !assert.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", s.envoyPort), s.DefaultWaitDuration)
		if err != nil {
			return false
		}
		defer conn.Close()
		return true
	}, s.DefaultWaitDuration, s.DefaultTickDuration, "Envoy is not ready yet") {
		t.Fatalf("Envoy unhealthy: %s", buf.String())
	}

	s.envoyLogBuffer = buf
	return func() { require.NoError(t, cmd.Process.Signal(syscall.SIGINT)) }
}

func (s *TestSuiteKit) GetEnvoyHost() string {
	return fmt.Sprintf("http://localhost:%d", s.envoyPort)
}

func (s *TestSuiteKit) GetAdminHost() string {
	return fmt.Sprintf("http://localhost:%d", s.adminPort)
}

func (s *TestSuiteKit) CheckEnvoyLog(expressions ...string) bool {
	for _, exp := range expressions {
		if strings.Contains(s.envoyLogBuffer.String(), exp) {
			return true
		}
	}

	return false
}
