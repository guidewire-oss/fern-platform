package acceptance_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("JIRA Requirement Coverage", Label("acceptance", "jira", "coverage", "e2e"), func() {
	var (
		browser   playwright.Browser
		bctx      playwright.BrowserContext
		page      playwright.Page
		auth      *helpers.LoginHelper
		mockJira  *helpers.MockJiraServer
		projectID string
	)

	BeforeEach(func() {
		var err error

		mockJira = helpers.NewMockJiraServer()

		browser = CreateBrowser()

		bctx, err = browser.NewContext(playwright.BrowserNewContextOptions{
			BaseURL: playwright.String(baseURL),
		})
		Expect(err).NotTo(HaveOccurred())
		bctx.SetDefaultTimeout(30000)

		page, err = bctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)
		auth.Login()

		// Wait for graphqlClient to be available.
		Eventually(func() bool {
			result, err := page.Evaluate(`() => typeof window.graphqlClient !== 'undefined'`, nil)
			if err != nil {
				return false
			}
			v, _ := result.(bool)
			return v
		}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())

		// Create a fresh project for each test.
		projName := fmt.Sprintf("CovTest-%s", time.Now().Format("20060102-150405"))
		body, err := gqlEval(page,
			`mutation($input: CreateProjectInput!) { createProject(input: $input) { id projectId } }`,
			map[string]any{"input": map[string]any{
				"projectId":     projName,
				"name":          projName,
				"defaultBranch": "main",
			}},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(body["errors"]).To(BeNil(), "createProject failed: %v", body["errors"])
		data := body["data"].(map[string]any)
		projectID = data["createProject"].(map[string]any)["projectId"].(string)
	})

	AfterEach(func() {
		if mockJira != nil {
			mockJira.Close()
		}
		defer func() { recover() }()
		if page != nil {
			_ = page.Close()
		}
		if bctx != nil {
			_ = bctx.Close()
		}
		if browser != nil {
			_ = browser.Close()
		}
	})

	// navigateToIntegrations opens the project settings Integrations tab.
	navigateToIntegrations := func() {
		_, err := page.Goto(fmt.Sprintf("%s/#/projects", baseURL))
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(2 * time.Second)
		projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectID))
		Expect(projectRow.Locator("button[title='Project Settings']").Click()).To(Succeed())
		time.Sleep(time.Second)
		Expect(page.Locator("button:has-text('Integrations')").Click()).To(Succeed())
		time.Sleep(500 * time.Millisecond)
	}

	// addJiraConnection creates a JIRA connection pointing at the local mock.
	addJiraConnection := func() {
		connBody, err := gqlEval(page,
			`mutation($input: CreateJiraConnectionInput!) { createJiraConnection(input: $input) { id } }`,
			map[string]any{"input": map[string]any{
				"projectId":          projectID,
				"name":               "Mock JIRA",
				"jiraUrl":            mockJira.URL,
				"authenticationType": "API_TOKEN",
				"projectKey":         "PROJ",
				"username":           "test@fern.com",
				"credential":         "test-api-token-123",
			}},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(connBody["errors"]).To(BeNil(), "createJiraConnection failed: %v", connBody["errors"])
		connID := connBody["data"].(map[string]any)["createJiraConnection"].(map[string]any)["id"].(string)

		_, err = gqlEval(page,
			`mutation($id: ID!) { testJiraConnection(id: $id) }`,
			map[string]any{"id": connID},
		)
		Expect(err).NotTo(HaveOccurred())
	}

	// configureVersionsAndIssues seeds the mock with one unreleased and one released version,
	// plus a simple epic → story hierarchy.
	configureVersionsAndIssues := func() {
		mockJira.SetVersions("PROJ", []helpers.MockVersion{
			{ID: "20002", Name: "v2.0", Released: false},
			{ID: "10001", Name: "v1.0", Released: true, ReleaseDate: "2025-01-15"},
		})
		mockJira.AddIssueByKey(helpers.MockIssue{
			Key: "PROJ-1", Summary: "Epic One", IssueType: "Epic",
		})
		mockJira.SetIssuesForVersion("v2.0", []helpers.MockIssue{
			{Key: "PROJ-1", Summary: "Epic One", IssueType: "Epic"},
			{Key: "PROJ-10", Summary: "Story Alpha", IssueType: "Story", Parent: &helpers.MockIssueParent{Key: "PROJ-1", IssueType: "Epic"}},
			{Key: "PROJ-11", Summary: "Story Beta", IssueType: "Story", Parent: &helpers.MockIssueParent{Key: "PROJ-1", IssueType: "Epic"}},
			{Key: "PROJ-99", Summary: "Orphan Story", IssueType: "Story"},
		})
	}

	// --- Scenario 1: Coverage section visibility ---

	Describe("Coverage section visibility", func() {
		It("shows a Coverage section when the project has a JIRA connection", func() {
			By("Setting up a JIRA connection")
			addJiraConnection()
			navigateToIntegrations()

			By("Asserting the Coverage section heading is visible")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			Expect(coverageHeading.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(coverageHeading.IsVisible()).To(BeTrue())
		})

		It("does not show a Coverage section when the project has no JIRA connection", func() {
			By("Navigating to Integrations without adding a connection")
			navigateToIntegrations()
			time.Sleep(time.Second)

			By("Asserting the Coverage section heading is absent")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			count, _ := coverageHeading.Count()
			Expect(count).To(Equal(0))
		})
	})

	// --- Scenario 2: Fix version picker ---

	Describe("Fix version picker", func() {
		BeforeEach(func() {
			configureVersionsAndIssues()
			addJiraConnection()
			navigateToIntegrations()

			By("Waiting for Coverage section to appear")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			Expect(coverageHeading.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
		})

		It("loads fix versions with unreleased listed before released", func() {
			By("Opening the fix version picker")
			picker := page.Locator("#coverage-version-picker")
			Expect(picker.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			By("Getting option text")
			options := picker.Locator("option")
			count, err := options.Count()
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">=", 2))

			first, err := options.Nth(0).TextContent()
			Expect(err).NotTo(HaveOccurred())
			second, err := options.Nth(1).TextContent()
			Expect(err).NotTo(HaveOccurred())

			By("Asserting unreleased version appears before released")
			Expect(first).To(ContainSubstring("v2.0"), "first option should be unreleased v2.0")
			Expect(second).To(ContainSubstring("v1.0"), "second option should be released v1.0")
		})

		It("filters versions as the user types", func() {
			By("Opening the fix version search/filter input")
			filterInput := page.Locator("#coverage-version-filter")
			Expect(filterInput.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			By("Typing 'v1' to filter")
			Expect(filterInput.Fill("v1")).To(Succeed())
			time.Sleep(300 * time.Millisecond)

			By("Verifying only v1.0 is shown in the picker")
			picker := page.Locator("#coverage-version-picker option")
			count, err := picker.Count()
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			text, err := picker.First().TextContent()
			Expect(err).NotTo(HaveOccurred())
			Expect(text).To(ContainSubstring("v1.0"))
		})
	})

	// --- Scenario 3: Coverage hierarchy ---

	Describe("Coverage hierarchy", func() {
		BeforeEach(func() {
			configureVersionsAndIssues()
			addJiraConnection()
			navigateToIntegrations()

			By("Waiting for Coverage section and selecting v2.0")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			Expect(coverageHeading.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			picker := page.Locator("#coverage-version-picker")
			Expect(picker.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)})).To(Succeed())
			Expect(picker.SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("v2.0")})).To(Succeed())
			time.Sleep(2 * time.Second)
		})

		It("renders epics as collapsible rows", func() {
			epicRow := page.Locator(".coverage-epic-row").First()
			Expect(epicRow.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(epicRow.IsVisible()).To(BeTrue())

			epicText, err := epicRow.TextContent()
			Expect(err).NotTo(HaveOccurred())
			Expect(epicText).To(ContainSubstring("PROJ-1"))
		})

		It("renders stories nested under their epic", func() {
			storyRow := page.Locator(".coverage-story-row").First()
			Expect(storyRow.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			stories := page.Locator(".coverage-story-row")
			count, err := stories.Count()
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">=", 2), "at least PROJ-10 and PROJ-11 should appear")
		})

		It("renders an Unassigned section for stories without an epic", func() {
			unassigned := page.Locator(".coverage-unassigned-section")
			Expect(unassigned.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			unassignedText, err := unassigned.TextContent()
			Expect(err).NotTo(HaveOccurred())
			Expect(unassignedText).To(ContainSubstring("PROJ-99"))
		})
	})

	// --- Scenario 4: Show uncovered only toggle ---

	Describe("Show uncovered only toggle", func() {
		BeforeEach(func() {
			configureVersionsAndIssues()
			addJiraConnection()
			navigateToIntegrations()

			By("Waiting for Coverage section and selecting a version")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			Expect(coverageHeading.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			picker := page.Locator("#coverage-version-picker")
			Expect(picker.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)})).To(Succeed())
			Expect(picker.SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("v2.0")})).To(Succeed())
			time.Sleep(2 * time.Second)

			By("Waiting for story rows to appear")
			Expect(page.Locator(".coverage-story-row").First().WaitFor(
				playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)},
			)).To(Succeed())
		})

		It("has a Show uncovered only checkbox above the tree", func() {
			toggle := page.Locator("#coverage-uncovered-only")
			Expect(toggle.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(3000),
			})).To(Succeed())
			Expect(toggle.IsVisible()).To(BeTrue())

			label := page.Locator("label[for='coverage-uncovered-only']")
			Expect(label.IsVisible()).To(BeTrue())
			labelText, err := label.TextContent()
			Expect(err).NotTo(HaveOccurred())
			Expect(labelText).To(ContainSubstring("uncovered"))
		})

		It("hides covered story rows when the toggle is checked", func() {
			By("Counting stories before toggle")
			storiesBefore, err := page.Locator(".coverage-story-row").Count()
			Expect(err).NotTo(HaveOccurred())
			Expect(storiesBefore).To(BeNumerically(">", 0))

			By("Checking the toggle")
			toggle := page.Locator("#coverage-uncovered-only")
			Expect(toggle.Check()).To(Succeed())
			time.Sleep(300 * time.Millisecond)

			By("Verifying covered rows are hidden")
			covered := page.Locator(".coverage-story-row.covered")
			coveredCount, err := covered.Count()
			Expect(err).NotTo(HaveOccurred())
			if coveredCount > 0 {
				for i := range coveredCount {
					visible, _ := covered.Nth(i).IsVisible()
					Expect(visible).To(BeFalse(), "covered story row %d should be hidden", i)
				}
			}
		})
	})

	// --- Scenario 5: JIRA unavailable error state ---

	Describe("JIRA unavailable error state", func() {
		BeforeEach(func() {
			configureVersionsAndIssues()
			addJiraConnection()
			navigateToIntegrations()

			By("Waiting for Coverage section")
			coverageHeading := page.Locator("h3:has-text('Requirement Coverage'), h2:has-text('Requirement Coverage')")
			Expect(coverageHeading.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())

			By("Making JIRA unavailable before the user selects a version")
			mockJira.SimulateUnavailable(true)
		})

		It("shows an error message when JIRA is unavailable during version list load", func() {
			By("Re-navigating to trigger a fresh jiraFixVersions load")
			navigateToIntegrations()

			By("Waiting for an error message in the Coverage section")
			errMsg := page.Locator("#coverage-panel .coverage-error, #coverage-panel [role='alert']")
			Expect(errMsg.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(errMsg.IsVisible()).To(BeTrue())
		})

		It("shows an error message when JIRA is unavailable after version selection", func() {
			By("Temporarily restoring JIRA so the picker loads")
			mockJira.SimulateUnavailable(false)

			picker := page.Locator("#coverage-version-picker")
			Expect(picker.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)})).To(Succeed())

			By("Making JIRA unavailable before selecting the version")
			mockJira.SimulateUnavailable(true)
			Expect(picker.SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("v2.0")})).To(Succeed())

			By("Waiting for error message")
			errMsg := page.Locator("#coverage-panel .coverage-error, #coverage-panel [role='alert']")
			Expect(errMsg.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(errMsg.IsVisible()).To(BeTrue())
		})

		It("does not crash the tab when JIRA is unavailable", func() {
			By("Re-navigating with JIRA down")
			navigateToIntegrations()
			time.Sleep(2 * time.Second)

			By("Verifying the Integrations tab is still rendered (not crashed)")
			integrationsPanel := page.Locator("button:has-text('Integrations')")
			Expect(integrationsPanel.IsVisible()).To(BeTrue())

			By("Verifying no unhandled error page is shown")
			errorPage := page.Locator("text=Something went wrong, text=Unhandled error, text=Cannot read properties")
			count, _ := errorPage.Count()
			Expect(count).To(Equal(0))
		})
	})
})
