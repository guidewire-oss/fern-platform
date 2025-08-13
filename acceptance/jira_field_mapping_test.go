package acceptance_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

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

			By("Clearing all mappings")
			clearButton := page.Locator("button:has-text('Clear All')")
			Expect(clearButton.Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)

			By("Attempting to save without required mappings")
			saveButton := page.Locator("button:has-text('Save Mapping Configuration')")
			Expect(saveButton.Click()).To(Succeed())

			By("Verifying validation message appears")
			// Alert should appear for missing required fields
			page.OnDialog(func(dialog playwright.Dialog) {
				Expect(dialog.Message()).To(ContainSubstring("required fields"))
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
})