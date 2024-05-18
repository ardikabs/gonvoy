//go:build e2e
// +build e2e

package suite

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
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DefaultEnvoyImageVersion = "envoyproxy/envoy:contrib-v1.29-latest"
)

func NewTestSuite(opt TestSuiteOptions) *TestSuite {
	if opt.EnvoyImageVersion == "" {
		opt.EnvoyImageVersion = DefaultEnvoyImageVersion
	}

	if opt.EnvoyPortStartFrom == 0 {
		opt.EnvoyPortStartFrom = 10000
	}

	if opt.AdminPortStartFrom == 0 {
		opt.AdminPortStartFrom = 8000
	}

	if SkipTests != nil {
		opt.SkipTests = append(opt.SkipTests, parseSkipTests(*SkipTests)...)
	}

	return &TestSuite{
		EnvoyImageVersion: opt.EnvoyImageVersion,
		EnvoyPort:         opt.EnvoyPortStartFrom,
		AdminPort:         opt.AdminPortStartFrom,
		RunTest:           *RunTest,
		SkipTests:         sets.New(opt.SkipTests...),
	}
}

type TestSuiteOptions struct {
	EnvoyImageVersion  string
	EnvoyPortStartFrom int
	AdminPortStartFrom int
	SkipTests          []string
}

type TestSuite struct {
	EnvoyImageVersion string
	EnvoyPort         int
	AdminPort         int
	RunTest           string
	SkipTests         sets.Set[string]
}

func (s *TestSuite) Run(t *testing.T, cases []TestCase) {
	s.pullEnvoyImage(t)

	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if s.SkipTests.Has(c.Name) {
				t.Skipf("Skipping %s: test explicitly skipped", c.Name)
			}

			c.Test(t, &TestSuiteKit{
				filterName:        c.FilterName,
				envoyImageVersion: s.EnvoyImageVersion,
				adminPort:         s.AdminPort + i,
				envoyPort:         s.EnvoyPort + i,
			})
		})

		if s.RunTest == c.Name {
			break
		}
	}
}

func (s *TestSuite) pullEnvoyImage(t *testing.T) {
	cmd := exec.Command("docker", "pull", s.EnvoyImageVersion)
	require.NoError(t, cmd.Run())
}

type TestSuiteKit struct {
	filterName string

	envoyImageVersion string
	envoyPort         int
	adminPort         int

	accessLogBuffer *bytes.Buffer
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
		res, err := http.Get(fmt.Sprintf("http://localhost:%d/listeners", s.adminPort))
		if err != nil {
			return false
		}

		defer res.Body.Close()

		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 100*time.Millisecond, "Envoy failed to start") {
		t.Fatalf("Envoy stderr: %s", buf.String())
	}

	s.accessLogBuffer = buf
	return func() { require.NoError(t, cmd.Process.Signal(syscall.SIGINT)) }
}

func (s *TestSuiteKit) GetEnvoyHost() string {
	return fmt.Sprintf("http://localhost:%d", s.envoyPort)
}

func (s *TestSuiteKit) GetAdminHost() string {
	return fmt.Sprintf("http://localhost:%d", s.adminPort)
}

func (s *TestSuiteKit) CheckEnvoyAccessLog(expressions ...string) bool {
	for _, exp := range expressions {
		if strings.Contains(s.accessLogBuffer.String(), exp) {
			return true
		}
	}

	return false
}

type TestCase struct {
	Name        string
	FilterName  string
	Description string

	Test func(t *testing.T, kit *TestSuiteKit)
}
