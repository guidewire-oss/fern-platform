package jiracoverage_test

import (
	"os"

	fern "github.com/guidewire-oss/fern-ginkgo-client/v2/pkg/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ReportAfterSuite forwards the Ginkgo report — including every spec's Labels —
// to Fern once the suite finishes. Labels of the form jira:<KEY> arrive as the
// tags that drive release coverage; nothing else in a test needs to change.
//
// It only uploads when FERN_PROJECT_ID and FERN_BASE_URL are set, so a plain
// `go test` (e.g. in CI, with no live Fern) still runs the specs and passes.
var _ = ReportAfterSuite("fern", func(report Report) {
	projectID := os.Getenv("FERN_PROJECT_ID")
	baseURL := os.Getenv("FERN_BASE_URL")
	if projectID == "" || baseURL == "" {
		GinkgoWriter.Println("[fern] FERN_PROJECT_ID / FERN_BASE_URL not set — skipping coverage upload")
		return
	}

	// fern.New reads auth (FERN_AUTH_CLIENT_ID/SECRET, AUTH_URL) from the env
	// when the target deployment requires it.
	client, err := fern.New(projectID, fern.WithBaseURL(baseURL))
	Expect(err).NotTo(HaveOccurred())
	Expect(client.Report(report)).To(Succeed())
})

// The ONLY thing that links a test to a JIRA issue is the jira:<KEY> label.
// Replace the GWCP-#### keys with real issue keys from one of your releases,
// or nothing will match the release tree.
var _ = Describe("Checkout", func() {
	It("completes a guest checkout", Label("jira:GWCP-1234"), func() {
		Expect(1 + 1).To(Equal(2))
	})

	It("applies a discount code", Label("jira:GWCP-1235"), func() {
		Expect("discount").NotTo(BeEmpty())
	})

	// A single test can cover more than one issue — list several labels.
	It("shows tax for a multi-item cart", Label("jira:GWCP-1234", "jira:GWCP-1236"), func() {
		Expect([]int{1, 2}).To(HaveLen(2))
	})
})
