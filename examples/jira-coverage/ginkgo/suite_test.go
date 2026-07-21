package jiracoverage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Run with:  go mod tidy && go test ./...
// See README.md for the env vars that make it upload to a live Fern.
func TestJiraCoverageExample(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JIRA Coverage Example Suite")
}
