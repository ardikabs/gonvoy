package suite

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DefaultEnvoyImageVersion = "envoyproxy/envoy:contrib-v1.29-latest"
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

	// FilterDirectoryPattern defines the pattern for locating the filter directory.
	// Available substitutions:
	// - {filter} -> filter name
	// Example usage:
	//   FilterDirectoryPattern: "/path/to/PROJECT/e2e/filters/{filter}" ->
	//       1. Given 'filter' as the filter name, for a filter named 'helloworld', it translates to /path/to/PROJECT/e2e/filters/helloworld
	//       2. For the envoy.yaml file, it translates to /path/to/PROJECT/e2e/filters/helloworld/envoy.yaml
	//       3. For the filter.so file, it translates to /path/to/PROJECT/e2e/filters/helloworld/filter.so
	FilterDirectoryPattern string
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
		EnvoyImageVersion: opts.EnvoyImageVersion,
		EnvoyPort:         opts.EnvoyPortStartFrom,
		AdminPort:         opts.AdminPortStartFrom,
		SkipTests:         sets.New(opts.SkipTests...),
		RunTest:           *RunTest,

		FilterDirectoryPattern: opts.FilterDirectoryPattern,
	}
}

// TestSuite represents a test suite configuration.
type TestSuite struct {
	// FilterDirectoryPattern specifies the pattern for locating the filter directory.
	FilterDirectoryPattern string
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

func (s *TestSuite) Run(t *testing.T, cases []TestCase) {
	s.pullEnvoyImage(t)

	for i, c := range cases {
		c := c
		suitekit := &TestSuiteKit{
			DefaultWaitDuration: 5 * time.Second,
			DefaultTickDuration: 100 * time.Millisecond,
			envoyImageVersion:   s.EnvoyImageVersion,
			adminPort:           s.AdminPort + i,
			envoyPort:           s.EnvoyPort + i,
		}

		if c.EnvoyConfigName == "" {
			c.EnvoyConfigName = "envoy.yaml"
		}

		if c.EnvoyFilterName == "" {
			c.EnvoyFilterName = "filter.so"
		}

		if s.FilterDirectoryPattern != "" {
			suitekit.envoyConfigAbsPath = formatPath(s.FilterDirectoryPattern+"/{filename}", "{filter}", c.FilterName, "{filename}", c.EnvoyConfigName)
			suitekit.envoyFilterAbsPath = formatPath(s.FilterDirectoryPattern+"/{filename}", "{filter}", c.FilterName, "{filename}", c.EnvoyFilterName)
		}

		if c.EnvoyConfigAbsDir != "" {
			suitekit.envoyConfigAbsPath = formatPath(c.EnvoyConfigAbsDir+"/{filename}", "{filename}", c.EnvoyConfigName)
		}

		if c.EnvoyFilterAbsDir != "" {
			suitekit.envoyFilterAbsPath = formatPath(c.EnvoyFilterAbsDir+"/{filename}", "{filename}", c.EnvoyFilterName)
		}

		t.Run(c.Name, func(t *testing.T) {
			if s.SkipTests.Has(c.Name) || c.Skip {
				t.Skipf("Skipping %s: test explicitly skipped", c.Name)
			}

			if c.Parallel {
				t.Parallel()
			}

			c.Test(t, suitekit)
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

func formatPath(base string, args ...string) string {
	replacer := strings.NewReplacer(args...)
	return replacer.Replace(base)
}

// GetCurrentDirectory returns the current directory of the file that calls this function.
func GetCurrentDirectory(t *testing.T) string {
	_, file, _, ok := runtime.Caller(1)
	require.True(t, ok)

	absPath, err := filepath.Abs(file)
	require.NoError(t, err)

	return filepath.Dir(absPath)
}
