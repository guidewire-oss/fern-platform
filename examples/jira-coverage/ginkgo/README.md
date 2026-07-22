# Example: a Ginkgo suite tagged for JIRA coverage

A copy-paste Ginkgo example of "tag a test with `jira:<KEY>` and watch coverage
light up." Drop these two files into *your* test suite (not this repo — see the
note at the bottom on why this isn't a standalone module here).

Full walkthrough: [../../../docs/developers/linking-tests-to-jira.md](../../../docs/developers/linking-tests-to-jira.md).

## What it shows

- The `jira:<KEY>` **label** is the only thing you add to a test.
- The `ReportAfterSuite` block is the standard `fern-ginkgo-client` wiring that
  uploads the run; Fern turns the `jira:` labels into coverage.

## `checkout_test.go`

```go
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
```

## `suite_test.go`

```go
package jiracoverage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestJiraCoverageExample(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JIRA Coverage Example Suite")
}
```

## Run it in your project

```bash
go get github.com/guidewire-oss/fern-ginkgo-client/v2

export FERN_PROJECT_ID=<your-fern-project-id>   # SELECT project_id FROM projects;
export FERN_BASE_URL=http://fern-platform.local:8080
# If ingest is authenticated (the default), also set the client-credentials the
# reporter reads: FERN_AUTH_CLIENT_ID / FERN_AUTH_CLIENT_SECRET / AUTH_URL

go test ./...
```

Then connect JIRA, map the **Release Version** field, and open **Project →
Coverage**: `GWCP-1234` and `GWCP-1236` show covered & passing, `GWCP-1235`
covered & failing. Replace the `GWCP-####` keys with real issue keys from one of
your releases.

## Why this is a snippet, not a module here

This code is verified to compile and run, but it's kept as a reference snippet
rather than a buildable module in this repo: as a module it would pull
`fern-ginkgo-client`'s full dependency tree (go-git, x/crypto, …) into
fern-platform's scanned dependency graph, which repeatedly tripped dependency
scanners for what is a teaching example. You run it in your own project, where
you own those dependencies.
