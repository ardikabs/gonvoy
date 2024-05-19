package suite

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuiteKit represents a test suite kit that contains various configuration options.
type TestSuiteKit struct {
	// DefaultWaitDuration is the default duration to wait for certain operations.
	DefaultWaitDuration time.Duration
	// DefaultTickDuration is the default duration between ticks.
	DefaultTickDuration time.Duration

	// envoyConfigAbsPath is the absolute path to the Envoy configuration file.
	envoyConfigAbsPath string
	// envoyFilterAbsPath is the absolute path to the Envoy filter file.
	envoyFilterAbsPath string
	// envoyImageVersion is the version of the Envoy image to use.
	envoyImageVersion string
	// envoyPort is the port number on which Envoy listens.
	envoyPort int
	// adminPort is the port number for Envoy's admin interface.
	adminPort int
	// envoyLogBuffer is a buffer to store Envoy's log output.
	envoyLogBuffer *bytes.Buffer
}

func (s *TestSuiteKit) StartEnvoy(t *testing.T) (kill func()) {
	cmd := exec.Command("docker",
		"run",
		"--rm",
		"-p", fmt.Sprintf("%d:8000", s.adminPort),
		"-p", fmt.Sprintf("%d:10000", s.envoyPort),
		"-v", s.envoyConfigAbsPath+":/etc/envoy.yaml",
		"-v", s.envoyFilterAbsPath+":/filter.so",
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
