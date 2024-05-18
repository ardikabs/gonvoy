//go:build e2e
// +build e2e

package e2e

import (
	"flag"
	"testing"

	"github.com/ardikabs/gonvoy/test/e2e/suite"
	"github.com/ardikabs/gonvoy/test/e2e/tests"
)

func TestE2E(t *testing.T) {
	flag.Parse()

	tSuite := suite.NewTestSuite(suite.TestSuiteOptions{
		EnvoyVersion:       "envoyproxy/envoy:contrib-v1.29-latest",
		EnvoyPortStartFrom: 10000,
		AdminPortStartFrom: 8000,
	})

	tSuite.Run(t, tests.TestCases)
}
