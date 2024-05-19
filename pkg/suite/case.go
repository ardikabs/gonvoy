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
}
