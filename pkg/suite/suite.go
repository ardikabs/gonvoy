package suite

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DefaultEnvoyImageVersion = "envoyproxy/envoy:contrib-v1.30-latest"
	DefaultWaitDuration      = 5 * time.Second
	DefaultTickDuration      = 100 * time.Millisecond
)

// TestSuiteOptions represents the options for configuring a test suite.
type TestSuiteOptions struct {
	// EnvoyImageVersion specifies the version of the Envoy image to use.
	EnvoyImageVersion string
	// EnvoyPortStartFrom specifies the starting port number for Envoy instances.
	EnvoyPortStartFrom int
	// AdminPortStartFrom specifies the starting port number for Envoy admin interfaces.
	AdminPortStartFrom int
	// SkipTests is a list of test names to skip.
	SkipTests []string

	// FilterLocation defines the pattern for locating the filter directory.
	// If it is not provided, the default pattern would be "$PWD/filters/{filter}/{filename}".
	// Available substitutions:
	// - {filter} -> filter name
	// - {filename} -> file name, which are envoy.yaml and filter.so. Both can be overriden through the TestCase.
	// Example usage:
	//   FilterLocation: "/home/ardikabs/Workspaces/gaetway/e2e/filters/{filter}/{filename}" ->
	//       1. Given 'filter' as the filter name, for a filter named 'helloworld', it translates to /home/ardikabs/Workspaces/gaetway/e2e/filters/helloworld
	//       2. For the envoy.yaml file, it translates to /home/ardikabs/Workspaces/gaetway/e2e/filters/helloworld/envoy.yaml
	//       3. For the filter.so file, it translates to /home/ardikabs/Workspaces/gaetway/e2e/filters/helloworld/filter.so
	FilterLocation string
}

func NewTestSuite(opts TestSuiteOptions) *TestSuite {
	if opts.EnvoyImageVersion == "" {
		opts.EnvoyImageVersion = DefaultEnvoyImageVersion
	}

	if opts.EnvoyPortStartFrom == 0 {
		opts.EnvoyPortStartFrom = 10000
	}

	if opts.AdminPortStartFrom == 0 {
		opts.AdminPortStartFrom = 8000
	}

	if SkipTests != nil {
		opts.SkipTests = append(opts.SkipTests, parseSkipTests(*SkipTests)...)
	}

	return &TestSuite{
		FilterLocation:    opts.FilterLocation,
		EnvoyImageVersion: opts.EnvoyImageVersion,
		EnvoyPort:         opts.EnvoyPortStartFrom,
		AdminPort:         opts.AdminPortStartFrom,
		SkipTests:         sets.New(opts.SkipTests...),
		RunTest:           *RunTest,
	}
}

// TestSuite represents a test suite configuration.
type TestSuite struct {
	// FilterLocation defines the pattern for locating the filter directory.
	// It defaults to "$PWD/filters/{filter}/{filename}".
	FilterLocation string
	// EnvoyImageVersion specifies the version of the Envoy image to be used.
	EnvoyImageVersion string
	// EnvoyPort specifies the port number on which Envoy will listen.
	EnvoyPort int
	// AdminPort specifies the port number for Envoy's admin interface.
	AdminPort int
	// RunTest specifies the specific test to run.
	RunTest string
	// SkipTests specifies a set of tests to be skipped.
	SkipTests sets.Set[string]
}

func (suite *TestSuite) Run(t *testing.T, cases []TestCase) {
	if suite.FilterLocation == "" {
		curdir, err := os.Getwd()
		require.NoError(t, err)
		suite.FilterLocation = fmt.Sprintf("%s/filters/{filter}/{filename}", curdir)
	}

	suite.pullEnvoyImage(t)

	for i, c := range cases {
		c := c
		c.portSuffix = i

		t.Run(c.Name, func(t *testing.T) {
			c.Run(t, suite)
		})
	}
}

func (suite *TestSuite) pullEnvoyImage(t *testing.T) {
	cmd := exec.Command("docker", "pull", suite.EnvoyImageVersion)
	require.NoError(t, cmd.Run())
}

func formatPath(base string, args ...string) string {
	replacer := strings.NewReplacer(args...)
	return replacer.Replace(base)
}

func trimLastSlash(s string) string {
	return strings.TrimSuffix(s, "/")
}

func ensureRequiredFileExists(absPaths ...string) error {
	for _, path := range absPaths {
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("file %s does not exist", path)
			} else {
				return err
			}
		}
	}

	return nil
}
