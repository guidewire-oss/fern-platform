package interfaces_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInterfaces(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Interfaces Suite")
}
