package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPISuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}