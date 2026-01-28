package infrastructure_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInterfaces(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Infrastructure Suite")
}
