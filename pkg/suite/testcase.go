package suite

import "testing"

type TestCase struct {
	Name        string
	FilterName  string
	Description string

	Parallel bool
	Skip     bool

	Test func(t *testing.T, kit *TestSuiteKit)
}
