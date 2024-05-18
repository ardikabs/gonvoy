package suite

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DefaultEnvoyImageVersion = "envoyproxy/envoy:contrib-v1.29-latest"
)

type TestSuiteOptions struct {
	EnvoyImageVersion  string
	EnvoyPortStartFrom int
	AdminPortStartFrom int
	SkipTests          []string
	Parallel           bool
}

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
		SkipTests:         sets.New(opt.SkipTests...),
		RunTest:           *RunTest,
	}
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
		c := c
		suitekit := &TestSuiteKit{
			DefaultWaitDuration: 5 * time.Second,
			DefaultTickDuration: 100 * time.Millisecond,

			filterName:        c.FilterName,
			envoyImageVersion: s.EnvoyImageVersion,
			adminPort:         s.AdminPort + i,
			envoyPort:         s.EnvoyPort + i,
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
