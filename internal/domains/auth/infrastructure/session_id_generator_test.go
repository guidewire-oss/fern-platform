package infrastructure_test

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/infrastructure"
)

func TestAuthInfrastructure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Infrastructure Suite")
}

var _ = Describe("GenerateSessionID", func() {
	It("should generate a non-empty session ID", func() {
		id, err := infrastructure.GenerateSessionID()
		Expect(err).NotTo(HaveOccurred())
		Expect(id).NotTo(BeEmpty())
	})

	It("should generate unique IDs across calls", func() {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := infrastructure.GenerateSessionID()
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).NotTo(HaveKey(id), "duplicate session ID generated")
			ids[id] = true
		}
	})

	It("should produce base64 URL-encoded output", func() {
		id, err := infrastructure.GenerateSessionID()
		Expect(err).NotTo(HaveOccurred())

		// Decoding should succeed with URL encoding
		decoded, err := base64.URLEncoding.DecodeString(id)
		Expect(err).NotTo(HaveOccurred())
		Expect(decoded).To(HaveLen(32))
	})

	It("should produce IDs of consistent length", func() {
		id1, err1 := infrastructure.GenerateSessionID()
		id2, err2 := infrastructure.GenerateSessionID()

		Expect(err1).NotTo(HaveOccurred())
		Expect(err2).NotTo(HaveOccurred())
		Expect(len(id1)).To(Equal(len(id2)))
	})

	It("should produce IDs of expected length for 32 bytes base64-encoded", func() {
		id, err := infrastructure.GenerateSessionID()
		Expect(err).NotTo(HaveOccurred())
		// 32 bytes -> base64 URL encoding = ceil(32/3)*4 = 44 characters
		Expect(len(id)).To(Equal(44))
	})
})
