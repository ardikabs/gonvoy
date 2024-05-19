package suite

import (
	"flag"
	"strings"
)

var (
	SkipTests = flag.String("skip-tests", "", "Comma-separated list of tests to skip")
	RunTest   = flag.String("run-test", "", "Name of a single test to run, instead of the whole suite")
)

func parseSkipTests(t string) []string {
	if t == "" {
		return nil
	}
	return strings.Split(t, ",")
}
