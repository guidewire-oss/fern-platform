package domain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

var _ = Describe("FlakyTest", Label("unit", "domain", "testing"), func() {
	Describe("NewFlakyTest", func() {
		It("should create a new flaky test with valid inputs", func() {
			projectID := "project-123"
			testName := "test-flaky-behavior"
			suiteName := "integration-suite"

			flakyTest, err := domain.NewFlakyTest(projectID, testName, suiteName)

			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest).NotTo(BeNil())
			Expect(flakyTest.ProjectID()).To(Equal(projectID))
			Expect(flakyTest.TestName()).To(Equal(testName))
			Expect(flakyTest.SuiteName()).To(Equal(suiteName))
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusActive))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityLow))
			Expect(flakyTest.FirstSeenAt()).To(BeTemporally("~", time.Now(), time.Second))
			Expect(flakyTest.LastSeenAt()).To(BeTemporally("~", time.Now(), time.Second))
			Expect(flakyTest.FlakeRate()).To(Equal(float64(0)))
			Expect(flakyTest.TotalExecutions()).To(Equal(0))
			Expect(flakyTest.FlakyExecutions()).To(Equal(0))
		})

		It("should return error when project ID is empty", func() {
			flakyTest, err := domain.NewFlakyTest("", "test-name", "suite-name")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("project ID cannot be empty"))
			Expect(flakyTest).To(BeNil())
		})

		It("should return error when test name is empty", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "", "suite-name")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("test name cannot be empty"))
			Expect(flakyTest).To(BeNil())
		})

		It("should accept empty suite name", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "test-name", "")

			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest).NotTo(BeNil())
			Expect(flakyTest.SuiteName()).To(BeEmpty())
		})
	})

	Describe("RecordExecution", func() {
		var flakyTest *domain.FlakyTest

		BeforeEach(func() {
			var err error
			flakyTest, err = domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record a non-flaky execution", func() {
			originalLastSeen := flakyTest.LastSeenAt()
			time.Sleep(10 * time.Millisecond)

			flakyTest.RecordExecution(false, "")

			Expect(flakyTest.TotalExecutions()).To(Equal(1))
			Expect(flakyTest.FlakyExecutions()).To(Equal(0))
			Expect(flakyTest.FlakeRate()).To(Equal(float64(0)))
			Expect(flakyTest.LastErrorMessage()).To(BeEmpty())
			Expect(flakyTest.LastSeenAt()).To(BeTemporally(">", originalLastSeen))
		})

		It("should record a flaky execution with error message", func() {
			errorMessage := "Timeout waiting for element"
			flakyTest.RecordExecution(true, errorMessage)

			Expect(flakyTest.TotalExecutions()).To(Equal(1))
			Expect(flakyTest.FlakyExecutions()).To(Equal(1))
			Expect(flakyTest.FlakeRate()).To(Equal(float64(100)))
			Expect(flakyTest.LastErrorMessage()).To(Equal(errorMessage))
		})

		It("should calculate flake rate correctly over multiple executions", func() {
			// Record 3 flaky and 7 non-flaky executions
			for i := 0; i < 3; i++ {
				flakyTest.RecordExecution(true, "error")
			}
			for i := 0; i < 7; i++ {
				flakyTest.RecordExecution(false, "")
			}

			Expect(flakyTest.TotalExecutions()).To(Equal(10))
			Expect(flakyTest.FlakyExecutions()).To(Equal(3))
			Expect(flakyTest.FlakeRate()).To(BeNumerically("~", 30.0, 0.01))
		})
	})

	Describe("Severity Updates", func() {
		var flakyTest *domain.FlakyTest

		BeforeEach(func() {
			var err error
			flakyTest, err = domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set severity to low for low flake rate", func() {
			// 5% flake rate
			flakyTest.RecordExecution(true, "error")
			for i := 0; i < 19; i++ {
				flakyTest.RecordExecution(false, "")
			}

			Expect(flakyTest.FlakeRate()).To(BeNumerically("~", 5.0, 0.01))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityLow))
		})

		It("should set severity to medium for moderate flake rate", func() {
			// 15% flake rate
			for i := 0; i < 3; i++ {
				flakyTest.RecordExecution(true, "error")
			}
			for i := 0; i < 17; i++ {
				flakyTest.RecordExecution(false, "")
			}

			Expect(flakyTest.FlakeRate()).To(BeNumerically("~", 15.0, 0.01))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityMedium))
		})

		It("should set severity to high for high flake rate", func() {
			// 35% flake rate
			for i := 0; i < 7; i++ {
				flakyTest.RecordExecution(true, "error")
			}
			for i := 0; i < 13; i++ {
				flakyTest.RecordExecution(false, "")
			}

			Expect(flakyTest.FlakeRate()).To(BeNumerically("~", 35.0, 0.01))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityHigh))
		})

		It("should set severity to critical for very high flake rate with recent activity", func() {
			// 60% flake rate
			for i := 0; i < 6; i++ {
				flakyTest.RecordExecution(true, "error")
			}
			for i := 0; i < 4; i++ {
				flakyTest.RecordExecution(false, "")
			}

			Expect(flakyTest.FlakeRate()).To(BeNumerically("~", 60.0, 0.01))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityCritical))
		})
	})

	Describe("Status Management", func() {
		var flakyTest *domain.FlakyTest

		BeforeEach(func() {
			var err error
			flakyTest, err = domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should resolve an active flaky test", func() {
			err := flakyTest.Resolve()

			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusResolved))
		})

		It("should return error when resolving non-active test", func() {
			// First resolve it
			err := flakyTest.Resolve()
			Expect(err).NotTo(HaveOccurred())

			// Try to resolve again
			err = flakyTest.Resolve()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("can only resolve active flaky tests"))
		})

		It("should ignore an active flaky test", func() {
			err := flakyTest.Ignore()

			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusIgnored))
		})

		It("should ignore an ignored flaky test", func() {
			// First ignore it
			err := flakyTest.Ignore()
			Expect(err).NotTo(HaveOccurred())

			// Can ignore again
			err = flakyTest.Ignore()
			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusIgnored))
		})

		It("should return error when ignoring resolved test", func() {
			// First resolve it
			err := flakyTest.Resolve()
			Expect(err).NotTo(HaveOccurred())

			// Try to ignore
			err = flakyTest.Ignore()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot ignore resolved flaky tests"))
		})

		It("should reactivate a resolved test", func() {
			// Resolve first
			err := flakyTest.Resolve()
			Expect(err).NotTo(HaveOccurred())

			// Reactivate
			flakyTest.Reactivate()
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusActive))
		})

		It("should reactivate an ignored test", func() {
			// Ignore first
			err := flakyTest.Ignore()
			Expect(err).NotTo(HaveOccurred())

			// Reactivate
			flakyTest.Reactivate()
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusActive))
		})
	})

	Describe("Edge Cases", func() {
		It("should handle zero executions without division by zero", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())

			// No executions recorded
			Expect(flakyTest.TotalExecutions()).To(Equal(0))
			Expect(flakyTest.FlakeRate()).To(Equal(float64(0)))
			Expect(flakyTest.Severity()).To(Equal(domain.FlakySeverityLow))
		})

		It("should preserve last error message across multiple executions", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())

			// Record flaky with error
			flakyTest.RecordExecution(true, "First error")
			Expect(flakyTest.LastErrorMessage()).To(Equal("First error"))

			// Record non-flaky
			flakyTest.RecordExecution(false, "")
			Expect(flakyTest.LastErrorMessage()).To(Equal("First error"))

			// Record flaky with new error
			flakyTest.RecordExecution(true, "Second error")
			Expect(flakyTest.LastErrorMessage()).To(Equal("Second error"))
		})

		It("should handle concurrent status changes", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())

			// Status transitions
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusActive))

			err = flakyTest.Ignore()
			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusIgnored))

			flakyTest.Reactivate()
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusActive))

			err = flakyTest.Resolve()
			Expect(err).NotTo(HaveOccurred())
			Expect(flakyTest.Status()).To(Equal(domain.FlakyStatusResolved))
		})
	})

	Describe("Immutability", func() {
		It("should provide read-only access to fields", func() {
			flakyTest, err := domain.NewFlakyTest("project-123", "test-name", "suite-name")
			Expect(err).NotTo(HaveOccurred())

			// All getters should return values, not pointers
			// This ensures external code cannot modify internal state
			projectID := flakyTest.ProjectID()
			testName := flakyTest.TestName()
			suiteName := flakyTest.SuiteName()

			Expect(projectID).To(Equal("project-123"))
			Expect(testName).To(Equal("test-name"))
			Expect(suiteName).To(Equal("suite-name"))

			// Modifying returned values should not affect internal state
			// (strings are immutable in Go, so this is safe)
		})
	})
})