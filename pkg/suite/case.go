package suite

import "testing"

// TestCase represents a test case for the test suite.
type TestCase struct {
	// Name of the test case.
	Name string
	// Name of the filter.
	FilterName string
	// Description of the test case.
	Description string
	// Absolute directory path of the Envoy filter.
	EnvoyFilterAbsDir string
	// Absolute directory path of the Envoy configuration.
	EnvoyConfigAbsDir string
	// Name of the Envoy filter.
	EnvoyFilterName string
	// Name of the Envoy configuration.
	EnvoyConfigName string
	// Indicates whether the test case should be run in parallel.
	Parallel bool
	// Indicates whether the test case should explicitly be skipped.
	Skip bool
	// Test function for the test case.
	Test func(t *testing.T, kit *TestSuiteKit)

	// portSuffix is the suffix to be added to the Envoy Host and Admin Host port number.
	portSuffix int
}

func (tc *TestCase) Run(t *testing.T, suite *TestSuite) {
	if suite.SkipTests.Has(tc.Name) || tc.Skip || (suite.RunTest != "" && suite.RunTest != tc.Name) {
		t.Skipf("Skipping %s: test explicitly skipped", tc.Name)
	}

	if tc.Parallel {
		t.Parallel()
	}

	kit := &TestSuiteKit{
		WaitDuration:      DefaultWaitDuration,
		TickDuration:      DefaultTickDuration,
		envoyImageVersion: suite.EnvoyImageVersion,
		adminPort:         suite.AdminPort + tc.portSuffix,
		envoyPort:         suite.EnvoyPort + tc.portSuffix,
	}

	if tc.EnvoyConfigName == "" {
		tc.EnvoyConfigName = "envoy.yaml"
	}

	if tc.EnvoyFilterName == "" {
		tc.EnvoyFilterName = "filter.so"
	}

	if suite.FilterLocation != "" {
		kit.envoyConfigAbsPath = formatPath(suite.FilterLocation, "{filter}", tc.FilterName, "{filename}", tc.EnvoyConfigName)
		kit.envoyFilterAbsPath = formatPath(suite.FilterLocation, "{filter}", tc.FilterName, "{filename}", tc.EnvoyFilterName)
	}

	if tc.EnvoyConfigAbsDir != "" {
		kit.envoyConfigAbsPath = formatPath(trimLastSlash(tc.EnvoyConfigAbsDir)+"/{filename}", "{filename}", tc.EnvoyConfigName)
	}

	if tc.EnvoyFilterAbsDir != "" {
		kit.envoyFilterAbsPath = formatPath(trimLastSlash(tc.EnvoyFilterAbsDir)+"/{filename}", "{filename}", tc.EnvoyFilterName)
	}

	if err := ensureRequiredFileExists(kit.envoyConfigAbsPath, kit.envoyFilterAbsPath); err != nil {
		t.Logf("Test case '%s' failed with error: %v", tc.Name, err)
		t.Log("Please ensure you have built the filter and have the envoy configuration file in the filter directory.")
		t.FailNow()
	}

	tc.Test(t, kit)
}
