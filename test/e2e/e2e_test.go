//go:build e2e
// +build e2e

package e2e

import (
	"flag"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/suite"
	"github.com/ardikabs/gonvoy/test/e2e/tests"
)

func TestE2E(t *testing.T) {
	flag.Parse()

	currentDir := suite.GetCurrentDirectory(t)

	tSuite := suite.NewTestSuite(suite.TestSuiteOptions{
		FilterDirectoryPattern: currentDir + "/filters/{filter}",
		EnvoyImageVersion:      suite.DefaultEnvoyImageVersion,
		EnvoyPortStartFrom:     10000,
		AdminPortStartFrom:     8000,
	})

	tSuite.Run(t, tests.TestCases)
}
