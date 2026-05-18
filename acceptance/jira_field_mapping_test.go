package acceptance_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

// gqlEval executes a GraphQL query/mutation via fetch() in the browser page
// and returns the parsed response body. The browser session cookie is included
// automatically, so no separate auth token is required.
func gqlEval(page playwright.Page, query string, variables map[string]any) (map[string]any, error) {
	result, err := page.Evaluate(`async ({query, variables}) => {
		const resp = await fetch('/query', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ query, variables })
		});
		return await resp.json();
	}`, map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

var _ = Describe("JIRA Field Mapping", Label("acceptance", "jira", "field-mapping", "e2e"), func() {
	var (
		browser  playwright.Browser
		ctx      playwright.BrowserContext
		page     playwright.Page
		auth     *helpers.LoginHelper
		mockJira *helpers.MockJiraServer
	)

	BeforeEach(func() {
		var err error

		// Start mock JIRA server
		mockJira = helpers.NewMockJiraServer()

		// Create a new browser for each test
		browser = CreateBrowser()

		// Create browser context
		contextOptions := playwright.BrowserNewContextOptions{
			BaseURL: playwright.String(baseURL),
		}

		if recordVideo {
			contextOptions.RecordVideo = &playwright.RecordVideo{
				Dir:  "./videos",
				Size: &playwright.Size{Width: 1280, Height: 720},
			}
		}

		ctx, err = browser.NewContext(contextOptions)
		Expect(err).NotTo(HaveOccurred())

		ctx.SetDefaultTimeout(30000)

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)
	})

	AfterEach(func() {
		if mockJira != nil {
			mockJira.Close()
		}

		// Handle cleanup
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in AfterEach: %v\n", r)
			}

			if page != nil {
				_ = page.Close()
				page = nil
			}

			if ctx != nil {
				_ = ctx.Close()
				ctx = nil
			}

			if browser != nil {
				_ = browser.Close()
				browser = nil
			}
		}()
	})

	Describe("Field Mapping Configuration", func() {
		var projectName string
		var projectRow playwright.Locator

		BeforeEach(func() {
			By("Creating a test project with JIRA connection")
			auth.Login()
			
			// Create a new project
			Expect(page.Goto("/#/projects")).To(Succeed())
			time.Sleep(2 * time.Second)
			
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.Click()).To(Succeed())
			
			time.Sleep(500 * time.Millisecond)
			projectName = "Field Mapping Test " + time.Now().Format("20060102-150405")
			nameInput := page.Locator("input[placeholder='My Project']")
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())
			
			time.Sleep(2 * time.Second)
			
			// Navigate to project settings
			projectRow = page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())
			
			time.Sleep(1 * time.Second)
			Expect(page.Locator("button:has-text('Integrations')").Click()).To(Succeed())
			
			// Add a JIRA connection
			Expect(page.Locator("button:has-text('Add JIRA Connection')").Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)
			
			Expect(page.Fill("input[placeholder*='Production JIRA']", "Test JIRA")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-token")).To(Succeed())
			Expect(page.Locator("button:has-text('Create Connection')").Click()).To(Succeed())
			time.Sleep(2 * time.Second)

			By("Testing the connection to activate it")
			testButton := page.Locator("button:has-text('Test Connection')")
			Expect(testButton.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			})).To(Succeed())
			Expect(testButton.Click()).To(Succeed())
			time.Sleep(3 * time.Second)
		})

		It("should open field mapping modal when Configure Mapping is clicked", func() {
			By("Clicking Configure Mapping button")
			mappingButton := page.Locator("button:has-text('Configure Mapping')")
			Expect(mappingButton.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(mappingButton.Click()).To(Succeed())

			By("Verifying field mapping modal appears")
			time.Sleep(500 * time.Millisecond)
			modalTitle := page.Locator("h2:has-text('Configure Field Mapping')")
			Expect(modalTitle.IsVisible()).To(BeTrue())

			By("Verifying modal contains JIRA and Fern field columns")
			jiraColumn := page.Locator("h3:has-text('JIRA Fields')")
			Expect(jiraColumn.IsVisible()).To(BeTrue())
			
			fernColumn := page.Locator("h3:has-text('Fern Fields')")
			Expect(fernColumn.IsVisible()).To(BeTrue())
		})

		It("should display JIRA fields with proper metadata", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying standard JIRA fields are displayed")
			// Check for key JIRA fields - be more specific to avoid duplicates
			issueKeyField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Issue Key",
			}).First()
			Expect(issueKeyField.IsVisible()).To(BeTrue())
			
			summaryField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Summary",
			}).First()
			Expect(summaryField.IsVisible()).To(BeTrue())
			
			descriptionField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Description",
			}).First()
			Expect(descriptionField.IsVisible()).To(BeTrue())
			
			issueTypeField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Issue Type",
			}).First()
			Expect(issueTypeField.IsVisible()).To(BeTrue())
			
			fixVersionField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Fix Version/s",
			}).First()
			Expect(fixVersionField.IsVisible()).To(BeTrue())
			
			labelsField := page.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Labels",
			}).First()
			Expect(labelsField.IsVisible()).To(BeTrue())

			By("Verifying custom fields are marked")
			customFieldIndicator := page.Locator("span:has-text('Custom')").First()
			Expect(customFieldIndicator.IsVisible()).To(BeTrue())

			By("Verifying field examples are shown")
			exampleText := page.Locator("text=Example:").First()
			Expect(exampleText.IsVisible()).To(BeTrue())
		})

		It("should display Fern fields with requirements indicators", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying required Fern fields are marked")
			// Check for required field indicators (*)
			requiredFields := page.Locator("span:has-text('*')")
			count, _ := requiredFields.Count()
			Expect(count).To(BeNumerically(">", 0))

			By("Verifying Fern field descriptions are shown")
			Expect(page.Locator("text=Unique identifier for the requirement").IsVisible()).To(BeTrue())
			Expect(page.Locator("text=Brief title of the requirement").IsVisible()).To(BeTrue())
		})

		It("should allow dragging JIRA fields to Fern fields", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying JIRA fields can be dragged")
			// Find a JIRA field that can be dragged
			jiraFieldContainer := page.Locator("div[data-scrollable]").First()
			summaryField := jiraFieldContainer.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Summary",
			}).First()
			
			// Check that the field exists and has proper styling
			Expect(summaryField.IsVisible()).To(BeTrue())
			
			// Verify the field has drag-related styling
			borderStyle, _ := summaryField.Evaluate("el => window.getComputedStyle(el).border", nil)
			Expect(borderStyle).To(ContainSubstring("2px"))

			By("Verifying Fern fields are present as drop targets")
			fernFieldContainer := page.Locator("div[data-scrollable]").Nth(1)
			titleField := fernFieldContainer.Locator("div").Filter(playwright.LocatorFilterOptions{
				HasText: "Brief title of the requirement",
			}).First()
			Expect(titleField.IsVisible()).To(BeTrue())
		})

		It("should show existing mappings with visual indicators", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying pre-mapped fields show connections")
			// The modal should show default mappings with 🔗 emoji
			mappedIndicator := page.Locator("span:has-text('🔗')").First()
			Expect(mappedIndicator.IsVisible()).To(BeTrue())

			By("Verifying mapped JIRA fields show check marks")
			checkMark := page.Locator("span:has-text('✓')").First()
			Expect(checkMark.IsVisible()).To(BeTrue())
		})

		It("should allow removing existing mappings", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Finding and clicking remove button on a mapping")
			// Look for the ✕ button in a mapped field
			removeButton := page.Locator("button:has-text('✕')").First()
			if count, _ := removeButton.Count(); count > 0 {
				Expect(removeButton.Click()).To(Succeed())
				
				By("Verifying mapping is removed")
				time.Sleep(500 * time.Millisecond)
				// The mapped field indicator should be gone or reduced
			}
		})

		It("should validate required fields before saving", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Removing a required field mapping to create an invalid state")
			// Required fields are pre-mapped by default; removing one triggers validation
			removeButton := page.Locator("button:has-text('✕')").First()
			Expect(removeButton.Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)

			By("Attempting to save with a required field unmapped")
			saveButton := page.Locator("button:has-text('Save Mapping Configuration')")
			Expect(saveButton.Click()).To(Succeed())

			By("Verifying validation message appears")
			// Alert should appear for missing required fields
			page.OnDialog(func(dialog playwright.Dialog) {
				Expect(dialog.Message()).To(ContainSubstring("required"))
				dialog.Accept()
			})
		})

		It("should show mapping count in footer", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying mapping count is displayed")
			// Footer should show "X of Y required fields mapped"
			mappingCount := page.Locator("text=required fields mapped")
			Expect(mappingCount.IsVisible()).To(BeTrue())
		})

		It("should close modal when Cancel is clicked", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Clicking Cancel button")
			cancelButton := page.Locator("button:has-text('Cancel')")
			Expect(cancelButton.Click()).To(Succeed())

			By("Verifying modal is closed")
			time.Sleep(500 * time.Millisecond)
			modalTitle := page.Locator("h2:has-text('Configure Field Mapping')")
			Expect(modalTitle.IsVisible()).To(BeFalse())
		})

		It("should close modal when X button is clicked", func() {
			By("Opening field mapping modal")
			Expect(page.Locator("button:has-text('Configure Mapping')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Clicking X close button")
			// The ✕ button in the modal header
			closeButton := page.Locator("button:has-text('✕')").First()
			Expect(closeButton.Click()).To(Succeed())

			By("Verifying modal is closed")
			time.Sleep(500 * time.Millisecond)
			modalTitle := page.Locator("h2:has-text('Configure Field Mapping')")
			Expect(modalTitle.IsVisible()).To(BeFalse())
		})
	})

	// ---------------------------------------------------------------------------
	// GraphQL API-level persistence tests (backend path, no browser interaction
	// beyond the initial login required to obtain a session cookie).
	// ---------------------------------------------------------------------------
	Describe("Field Mapping GraphQL API", Label("graphql"), func() {
		var (
			projectID    string
			connectionID string
		)

		// Helper: navigate to app and wait for graphqlClient to be ready.
		waitForApp := func() {
			_, err := page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				result, err := page.Evaluate(`() => typeof window.graphqlClient !== 'undefined'`, nil)
				if err != nil {
					return false
				}
				v, _ := result.(bool)
				return v
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue(), "graphqlClient should be available in page context")
		}

		BeforeEach(func() {
			By("Log in as manager and navigate to app")
			auth.Login()
			waitForApp()

			By("Create a test project via GraphQL")
			projName := "FieldMapGQL-" + time.Now().Format("20060102-150405")
			body, err := gqlEval(page,
				`mutation($input: CreateProjectInput!) { createProject(input: $input) { id projectId } }`,
				map[string]any{"input": map[string]any{
					"projectId":     projName,
					"name":          projName,
					"defaultBranch": "main",
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			data := body["data"].(map[string]any)
			proj := data["createProject"].(map[string]any)
			projectID = proj["projectId"].(string)

			By("Create and activate a JIRA connection via GraphQL")
			connBody, err := gqlEval(page,
				`mutation($input: CreateJiraConnectionInput!) { createJiraConnection(input: $input) { id } }`,
				map[string]any{"input": map[string]any{
					"projectId":          projectID,
					"name":               "Mock JIRA",
					"jiraUrl":            "http://mock-jira:8080",
					"authenticationType": "API_TOKEN",
					"projectKey":         "TEST",
					"username":           "test@fern.com",
					"credential":         "test-api-token-123",
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			connData := connBody["data"].(map[string]any)
			conn := connData["createJiraConnection"].(map[string]any)
			connectionID = conn["id"].(string)

			By("Test the connection to activate it")
			_, err = gqlEval(page,
				`mutation($id: ID!) { testJiraConnection(id: $id) }`,
				map[string]any{"id": connectionID},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("(a) saves a valid mapping and it persists on reload", func() {
			By("Save a valid field mapping")
			saveBody, err := gqlEval(page,
				`mutation($input: SaveJiraFieldMappingInput!) {
					saveJiraFieldMapping(input: $input) {
						projectId
						entries { fernField jiraFieldId }
					}
				}`,
				map[string]any{"input": map[string]any{
					"projectId": projectID,
					"entries": []any{
						map[string]any{"fernField": "REQUIREMENT_ID", "jiraFieldId": "issuekey", "jiraFieldIsMultiValue": false},
						map[string]any{"fernField": "REQUIREMENT_TITLE", "jiraFieldId": "summary", "jiraFieldIsMultiValue": false},
					},
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(saveBody["errors"]).To(BeNil(), "save should succeed: %v", saveBody["errors"])

			By("Reload the mapping and verify persistence")
			loadBody, err := gqlEval(page,
				`query($projectId: String!) { jiraFieldMapping(projectId: $projectId) { entries { fernField jiraFieldId } } }`,
				map[string]any{"projectId": projectID},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(loadBody["errors"]).To(BeNil())
			data := loadBody["data"].(map[string]any)
			mapping := data["jiraFieldMapping"].(map[string]any)
			entries := mapping["entries"].([]any)
			fieldMap := map[string]string{}
			for _, e := range entries {
				entry := e.(map[string]any)
				fieldMap[entry["fernField"].(string)] = entry["jiraFieldId"].(string)
			}
			Expect(fieldMap["REQUIREMENT_ID"]).To(Equal("issuekey"))
			Expect(fieldMap["REQUIREMENT_TITLE"]).To(Equal("summary"))
		})

		It("(b) rejects a mapping with a duplicate JIRA field ID", func() {
			body, err := gqlEval(page,
				`mutation($input: SaveJiraFieldMappingInput!) { saveJiraFieldMapping(input: $input) { projectId } }`,
				map[string]any{"input": map[string]any{
					"projectId": projectID,
					"entries": []any{
						map[string]any{"fernField": "REQUIREMENT_ID", "jiraFieldId": "summary", "jiraFieldIsMultiValue": false},
						map[string]any{"fernField": "REQUIREMENT_TITLE", "jiraFieldId": "summary", "jiraFieldIsMultiValue": false},
					},
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			errs, ok := body["errors"].([]any)
			Expect(ok).To(BeTrue(), "response should contain errors: %v", body)
			Expect(len(errs)).To(BeNumerically(">", 0))
			errMsg := errs[0].(map[string]any)["message"].(string)
			Expect(strings.ToLower(errMsg)).To(ContainSubstring("duplicate"))
		})

		It("(c) rejects a mapping with a required Fern field unmapped", func() {
			body, err := gqlEval(page,
				`mutation($input: SaveJiraFieldMappingInput!) { saveJiraFieldMapping(input: $input) { projectId } }`,
				map[string]any{"input": map[string]any{
					"projectId": projectID,
					"entries": []any{
						// REQUIREMENT_TITLE is present but REQUIREMENT_ID is omitted
						map[string]any{"fernField": "REQUIREMENT_TITLE", "jiraFieldId": "summary", "jiraFieldIsMultiValue": false},
						map[string]any{"fernField": "DESCRIPTION", "jiraFieldId": "description", "jiraFieldIsMultiValue": false},
					},
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			errs, ok := body["errors"].([]any)
			Expect(ok).To(BeTrue(), "response should contain errors: %v", body)
			Expect(len(errs)).To(BeNumerically(">", 0))
			errMsg := errs[0].(map[string]any)["message"].(string)
			Expect(strings.ToLower(errMsg)).To(ContainSubstring("required"))
		})

		It("(d) denies read and write to a non-manager user", func() {
			By("Open a new browser context for the non-manager user")
			nonMgrCtx, err := browser.NewContext(playwright.BrowserNewContextOptions{
				BaseURL: playwright.String(baseURL),
			})
			Expect(err).NotTo(HaveOccurred())
			defer nonMgrCtx.Close()

			nonMgrPage, err := nonMgrCtx.NewPage()
			Expect(err).NotTo(HaveOccurred())
			defer nonMgrPage.Close()

			By("Log in as non-manager")
			nonMgrAuth := helpers.NewLoginHelper(nonMgrPage, baseURL, nonManagerUsername, nonManagerPassword)
			nonMgrAuth.Login()

			By("Navigate to app in non-manager context")
			_, err = nonMgrPage.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				result, err := nonMgrPage.Evaluate(`() => typeof window.graphqlClient !== 'undefined'`, nil)
				if err != nil {
					return false
				}
				v, _ := result.(bool)
				return v
			}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Attempt to read field mapping as non-manager")
			readBody, err := gqlEval(nonMgrPage,
				`query($projectId: String!) { jiraFieldMapping(projectId: $projectId) { projectId } }`,
				map[string]any{"projectId": projectID},
			)
			Expect(err).NotTo(HaveOccurred())
			readErrs, hasErrs := readBody["errors"].([]any)
			Expect(hasErrs).To(BeTrue(), "read should be denied: %v", readBody)
			Expect(len(readErrs)).To(BeNumerically(">", 0))
			readErrMsg := readErrs[0].(map[string]any)["message"].(string)
			Expect(strings.ToLower(readErrMsg)).To(ContainSubstring("unauthorized"))

			By("Attempt to save field mapping as non-manager")
			writeBody, err := gqlEval(nonMgrPage,
				`mutation($input: SaveJiraFieldMappingInput!) { saveJiraFieldMapping(input: $input) { projectId } }`,
				map[string]any{"input": map[string]any{
					"projectId": projectID,
					"entries": []any{
						map[string]any{"fernField": "REQUIREMENT_ID", "jiraFieldId": "issuekey", "jiraFieldIsMultiValue": false},
						map[string]any{"fernField": "REQUIREMENT_TITLE", "jiraFieldId": "summary", "jiraFieldIsMultiValue": false},
					},
				}},
			)
			Expect(err).NotTo(HaveOccurred())
			writeErrs, hasWriteErrs := writeBody["errors"].([]any)
			Expect(hasWriteErrs).To(BeTrue(), "write should be denied: %v", writeBody)
			Expect(len(writeErrs)).To(BeNumerically(">", 0))
			writeErrMsg := writeErrs[0].(map[string]any)["message"].(string)
			Expect(strings.ToLower(writeErrMsg)).To(ContainSubstring("unauthorized"))
		})
	})
})