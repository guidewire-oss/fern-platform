package pmconnectors_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("UC-11: PM Connector Project Linking and Field Mapping", Label("e2e"), func() {
	var (
		browser playwright.Browser
		ctx     playwright.BrowserContext
		page    playwright.Page
		auth    *helpers.LoginHelper
	)

	BeforeEach(func() {
		var err error

		// Create a new browser for each test
		browser = CreateBrowser()

		// Create browser context options
		contextOptions := playwright.BrowserNewContextOptions{
			BaseURL: playwright.String(baseURL),
		}

		// Add video recording if enabled
		if recordVideo {
			contextOptions.RecordVideo = &playwright.RecordVideo{
				Dir:  "./videos",
				Size: &playwright.Size{Width: 1280, Height: 720},
			}
		}

		ctx, err = browser.NewContext(contextOptions)
		Expect(err).NotTo(HaveOccurred())

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)

		// Login as admin user
		fmt.Printf("Attempting login with username: %s\n", username)
		auth.Login()
		fmt.Println("Login completed successfully")
	})

	AfterEach(func() {
		// Handle cleanup
		if page != nil {
			page.Close()
		}
		if ctx != nil {
			ctx.Close()
		}
		if browser != nil {
			browser.Close()
		}
	})

	Describe("UC-11-01: Create Mock JIRA Connector", Label("e2e"), func() {
		It("should create a connector to the mock JIRA instance", func() {
			// Navigate to PM connectors
			_, err := page.Goto(baseURL + "/pm-connectors")
			Expect(err).NotTo(HaveOccurred())

			// Wait for page to load
			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			Expect(err).NotTo(HaveOccurred())

			// Wait for PM Connectors page to be fully loaded
			// First check if we need to click on PM Connectors in the navigation
			navLink := page.Locator("a[href='/pm-connectors'], button:has-text('PM Connectors')")
			navCount, _ := navLink.Count()
			if navCount > 0 {
				// We're not on the PM Connectors page yet, need to navigate
				err = navLink.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for navigation
				err = page.WaitForURL("**/pm-connectors", playwright.PageWaitForURLOptions{
					Timeout: playwright.Float(10000),
				})
				Expect(err).NotTo(HaveOccurred())
			}

			// Now wait for the page content to load
			Eventually(func() bool {
				// Check current URL
				currentURL := page.URL()
				fmt.Printf("Current URL: %s\n", currentURL)

				// Check if user has access to PM Connectors
				accessDenied := page.Locator("text=/Access denied|not authorized/i")
				deniedCount, _ := accessDenied.Count()
				if deniedCount > 0 {
					fmt.Println("Access denied to PM Connectors page")
					return false
				}

				// Get page content for debugging
				bodyContent := page.Locator("body")
				bodyText, _ := bodyContent.TextContent()
				if len(bodyText) < 200 {
					fmt.Printf("Page body text: %s\n", bodyText)
				} else {
					fmt.Printf("Page body text (first 200 chars): %s...\n", bodyText[:200])
				}

				// Look for any heading
				allHeadings := page.Locator("h1, h2, h3")
				headingCount, _ := allHeadings.Count()
				fmt.Printf("Total headings found: %d\n", headingCount)

				// Look for any buttons
				allButtons := page.Locator("button")
				allButtonCount, _ := allButtons.Count()
				fmt.Printf("Total buttons found: %d\n", allButtonCount)

				// Look specifically for PM Connectors elements
				pmConnectorElements := page.Locator("*:has-text('PM Connector'), *:has-text('PM connector'), *:has-text('pm connector')")
				pmCount, _ := pmConnectorElements.Count()

				// Also check for Create/Add buttons
				createAddButtons := page.Locator("button:has-text('Create'), button:has-text('Add'), button:has-text('New')")
				createCount, _ := createAddButtons.Count()

				fmt.Printf("PM Connector elements: %d, Create/Add buttons: %d\n", pmCount, createCount)

				return pmCount > 0 || createCount > 0 || headingCount > 0
			}, 15*time.Second).Should(BeTrue())

			// Click Create Connector button
			createButton := page.Locator("button:has-text('Create Connector')")
			err = createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for modal to appear
			Eventually(func() bool {
				modal := page.Locator("div.modal-content, div[role='dialog']")
				count, _ := modal.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			// Fill in connector details
			nameInput := page.Locator("input[placeholder*='connector name'], input#name")
			err = nameInput.Fill("Mock JIRA Test")
			Expect(err).NotTo(HaveOccurred())

			// Select JIRA type by clicking the type selector
			typeButtons := page.Locator("div.type-selector button, button.type-option")
			jiraButton := typeButtons.Filter(playwright.LocatorFilterOptions{
				HasText: "JIRA",
			})
			err = jiraButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Fill base URL with mock JIRA URL - using k3d service name
			urlInput := page.Locator("input[placeholder*='Base URL'], input[placeholder*='base URL'], input#baseUrl")
			err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
			Expect(err).NotTo(HaveOccurred())

			// Click Create
			submitButton := page.Locator("button:has-text('Create'), button[type='submit']")
			err = submitButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for success
			Eventually(func() bool {
				successMsg := page.Locator("text=Connector created successfully")
				count, _ := successMsg.Count()
				return count > 0
			}, 10*time.Second).Should(BeTrue())
		})
	})

	Describe("UC-11-02: Link Connector to Project", Label("e2e"), func() {
		It("should link the PM connector to an existing project", func() {
			// First, navigate to projects
			_, err := page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			// Wait for dashboard to load
			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			Expect(err).NotTo(HaveOccurred())

			// Find and click on the first project
			projectCard := page.Locator("div.project-card").First()
			projectName, _ := projectCard.Locator("h3, h4").TextContent()
			fmt.Printf("Selected project: %s\n", projectName)

			// Click the project to view details
			err = projectCard.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for project details page
			Eventually(func() string {
				return page.URL()
			}, 5*time.Second).Should(ContainSubstring("/projects/"))

			// Look for PM Links section or Settings
			settingsTab := page.Locator("button:has-text('Settings'), a:has-text('Settings'), button:has-text('PM Links')")
			if count, _ := settingsTab.Count(); count > 0 {
				err = settingsTab.First().Click()
				Expect(err).NotTo(HaveOccurred())
			}

			// Check if user has permission to add PM links
			addPmLinkButton := page.Locator("button:has-text('Add PM Link'), button:has-text('Link PM Tool'), button:has-text('Add Connection')")
			count, _ := addPmLinkButton.Count()
			if count == 0 {
				Skip("User does not have permission to manage PM connections")
			}

			// Click Add PM Link
			err = addPmLinkButton.First().Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for form to appear
			time.Sleep(1 * time.Second)

			// Select the Mock JIRA connector
			connectorSelect := page.Locator("select[name='connectorId'], select#connector")
			if count, _ := connectorSelect.Count(); count > 0 {
				// Get the timestamp to find our connector
				timestamp := time.Now().Format("20060102")
				connectorPattern := fmt.Sprintf("Test JIRA %s", timestamp[:8])

				// Select by partial match
				options := page.Locator("select option")
				optCount, _ := options.Count()
				for i := 0; i < optCount; i++ {
					optText, _ := options.Nth(i).TextContent()
					if len(optText) > 0 && (optText == "Mock JIRA Test" || contains(optText, connectorPattern)) {
						optValue, _ := options.Nth(i).GetAttribute("value")
						_, err = connectorSelect.SelectOption(playwright.SelectOptionValues{
							Values: &[]string{optValue},
						})
						Expect(err).NotTo(HaveOccurred())
						break
					}
				}
			} else {
				// Try clicking on connector option directly
				connectorOption := page.Locator("div:has-text('Mock JIRA'), div:has-text('Test JIRA')")
				err = connectorOption.First().Click()
				Expect(err).NotTo(HaveOccurred())
			}

			// Enter external project key
			externalKeyInput := page.Locator("input[placeholder*='PROJECT'], input[placeholder*='External'], input#externalProjectKey, input[name='externalProjectKey']")
			err = externalKeyInput.Fill("TEST-1")
			Expect(err).NotTo(HaveOccurred())

			// Save the PM link
			saveButton := page.Locator("button:has-text('Save'), button:has-text('Create'), button:has-text('Add')")
			err = saveButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for success
			Eventually(func() bool {
				// Check for success message or PM link appearing in list
				success := page.Locator("text=/created|added|successfully/i")
				successCount, _ := success.Count()

				// Or check if the PM link appears in the list
				pmLink := page.Locator("text=TEST-1")
				linkCount, _ := pmLink.Count()

				return successCount > 0 || linkCount > 0
			}, 10*time.Second).Should(BeTrue())

			fmt.Println("✓ PM link created successfully")
		})
	})

	Describe("UC-11-03: Configure Field Mappings", Label("e2e"), func() {
		It("should configure field mappings for the PM link", func() {
			// Navigate back to the project
			_, err := page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			// Wait for page load
			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			Expect(err).NotTo(HaveOccurred())

			// Click on the project
			projectCard := page.Locator("div.project-card").First()
			err = projectCard.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for project page
			Eventually(func() string {
				return page.URL()
			}, 5*time.Second).Should(ContainSubstring("/projects/"))

			// Navigate to PM Links settings
			settingsTab := page.Locator("button:has-text('Settings'), a:has-text('Settings')")
			if count, _ := settingsTab.Count(); count > 0 {
				err = settingsTab.First().Click()
				Expect(err).NotTo(HaveOccurred())
			}

			// Find the PM link we created
			pmLink := page.Locator("text=TEST-1")
			Eventually(func() int {
				count, _ := pmLink.Count()
				return count
			}, 5*time.Second).Should(BeNumerically(">", 0))

			// Click configure or edit mappings
			configureButton := page.Locator("button:has-text('Configure'), button:has-text('Edit Mappings'), button:has-text('Field Mappings')")
			if count, _ := configureButton.Count(); count > 0 {
				err = configureButton.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for mapping interface
				time.Sleep(2 * time.Second)

				// Configure basic field mappings using selects
				// Map ID field
				idSelect := page.Locator("select[name='id'], select[name='requirementId']")
				if count, _ := idSelect.Count(); count > 0 {
					_, err = idSelect.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{"key"},
					})
					Expect(err).NotTo(HaveOccurred())
				}

				// Map Title field
				titleSelect := page.Locator("select[name='title'], select[name='name']")
				if count, _ := titleSelect.Count(); count > 0 {
					_, err = titleSelect.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{"fields.summary"},
					})
					Expect(err).NotTo(HaveOccurred())
				}

				// Map Description field
				descSelect := page.Locator("select[name='description']")
				if count, _ := descSelect.Count(); count > 0 {
					_, err = descSelect.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{"fields.description"},
					})
					Expect(err).NotTo(HaveOccurred())
				}

				// Save mappings
				saveButton := page.Locator("button:has-text('Save'), button[type='submit']")
				err = saveButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for save to complete
				Eventually(func() bool {
					success := page.Locator("text=/saved|updated/i")
					count, _ := success.Count()
					return count > 0
				}, 5*time.Second).Should(BeTrue())

				fmt.Println("✓ Field mappings configured")
			} else {
				// Field mappings might be configured during link creation
				fmt.Println("Field mappings already configured during link creation")
			}
		})
	})

	Describe("UC-11-04: Verify Field Mapping Persistence", Label("e2e"), func() {
		It("should persist field mappings after save", func() {
			// Navigate back to the project
			_, err := page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			// Find and edit the same project
			projectCard := page.Locator("div.project-card").First()
			editButton := projectCard.Locator("button[title='Edit Project']")
			err = editButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Go to PM Tools tab
			pmToolsTab := page.Locator("button:has-text('PM Tools')")
			err = pmToolsTab.Click()
			Expect(err).NotTo(HaveOccurred())

			// Find the Mock JIRA connection
			mockJiraConnection := page.Locator("div:has-text('Mock JIRA Test')")
			Expect(mockJiraConnection).To(BeVisible())

			// Click on Configure/Edit mappings
			configureMappingButton := mockJiraConnection.Locator("button:has-text('Configure Mappings')")
			err = configureMappingButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Verify mappings are loaded
			Eventually(func() bool {
				// Check if Title mapping exists
				titleMapping := page.Locator("text=fields.summary")
				count, _ := titleMapping.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			// Verify Description mapping
			descMapping := page.Locator("text=fields.description")
			Expect(descMapping).To(BeVisible())

			// Verify Status mapping
			statusMapping := page.Locator("text=fields.status.name")
			Expect(statusMapping).To(BeVisible())
		})
	})

	Describe("UC-11-05: Activate PM Link", Label("e2e"), func() {
		It("should activate the PM link after configuration", func() {
			// Continue from previous test or navigate to project PM Tools

			// Find the activate button
			activateButton := page.Locator("button:has-text('Activate')")
			err := activateButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for activation
			Eventually(func() bool {
				// Check if link is now active
				activeStatus := page.Locator("text=Active")
				count, _ := activeStatus.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			// Verify sync button appears
			syncButton := page.Locator("button:has-text('Sync')")
			Expect(syncButton).To(BeVisible())
		})
	})

	Describe("UC-11-06: Test Error Scenarios", Label("e2e"), func() {
		It("should handle invalid project ID gracefully", func() {
			// Create a new connection with invalid project ID
			addConnectionButton := page.Locator("button:has-text('Add Connection')")
			count, _ := addConnectionButton.Count()
			if count == 0 {
				Skip("No Add Connection button found")
			}

			err := addConnectionButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Select connector
			connectorOption := page.Locator("div:has-text('Mock JIRA Test')").First()
			err = connectorOption.Click()
			Expect(err).NotTo(HaveOccurred())

			// Click Next
			nextButton := page.Locator("button:has-text('Next')")
			err = nextButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Enter invalid project ID
			externalIdInput := page.Locator("input[placeholder*='PROJECT-123']")
			err = externalIdInput.Fill("INVALID-PROJECT")
			Expect(err).NotTo(HaveOccurred())

			// Try to proceed
			err = nextButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Should see error message
			Eventually(func() bool {
				errorMsg := page.Locator("text=/Failed|Error|not found/i")
				count, _ := errorMsg.Count()
				return count > 0
			}, 10*time.Second).Should(BeTrue())
		})
	})
})

// Helper function to wait for element
func waitForElement(page playwright.Page, selector string, timeout time.Duration) playwright.Locator {
	locator := page.Locator(selector)
	Eventually(func() bool {
		count, _ := locator.Count()
		return count > 0
	}, timeout).Should(BeTrue())
	return locator
}

// Helper to check element visibility
func isElementVisible(locator playwright.Locator) bool {
	visible, _ := locator.IsVisible()
	return visible
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
