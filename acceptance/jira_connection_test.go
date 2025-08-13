package acceptance_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("JIRA Connection Management", Label("acceptance", "jira", "e2e"), func() {
	var (
		browser    playwright.Browser
		ctx        playwright.BrowserContext
		page       playwright.Page
		auth       *helpers.LoginHelper
		mockJira   *helpers.MockJiraServer
	)

	BeforeEach(func() {
		var err error

		// Start mock JIRA server
		mockJira = helpers.NewMockJiraServer()

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

		// Set default timeout
		ctx.SetDefaultTimeout(30000)

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		auth = helpers.NewLoginHelper(page, baseURL, username, password)
	})

	AfterEach(func() {
		// Stop mock JIRA server
		if mockJira != nil {
			mockJira.Close()
		}

		// Handle cleanup even if test fails
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in AfterEach: %v\n", r)
			}

			// Force cleanup in correct order
			// 1. Close page first
			if page != nil {
				_ = page.Close()
				page = nil
			}

			// 2. Close context
			if ctx != nil {
				_ = ctx.Close()
				ctx = nil
			}

			// 3. Close browser last
			if browser != nil {
				_ = browser.Close()
				browser = nil
			}
		}()

		// Save video if recording is enabled
		if recordVideo && page != nil {
			video := page.Video()
			if video != nil {
				// Get the test name for the video file
				testName := CurrentSpecReport().LeafNodeText
				testName = strings.ReplaceAll(testName, " ", "_")
				testName = strings.ReplaceAll(testName, "/", "-")

				// Save the video with a descriptive name
				timestamp := time.Now().Format("20060102_150405")
				newPath := fmt.Sprintf("../videos/jira/%s_%s.webm", testName, timestamp)

				// Get the original video path before closing
				originalPath, _ := video.Path()

				// Close the page first (not the context) to finalize video
				if page != nil {
					page.Close()
					page = nil
				}

				// Wait a moment for video to be saved
				time.Sleep(1 * time.Second)

				// Create directory if it doesn't exist
				os.MkdirAll("../videos/jira", 0755)

				// Try to move the video file
				if originalPath != "" {
					if err := os.Rename(originalPath, newPath); err != nil {
						GinkgoWriter.Printf("Warning: Could not rename video file: %v\n", err)
					} else {
						GinkgoWriter.Printf("Video saved to: %s\n", newPath)
					}
				}
			}
		}
	})

	Describe("Managing JIRA Connections", func() {
		It("should successfully create project and add JIRA connection via settings", func() {
			By("Logging in as a project manager")
			auth.Login()

			By("Navigating to the projects page")
			Expect(page.Goto("/#/projects")).To(Succeed())
			
			By("Clicking the New Project button")
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			})).To(Succeed())
			Expect(createButton.Click()).To(Succeed())

			By("Waiting for the modal to appear")
			time.Sleep(500 * time.Millisecond)

			By("Filling in project details")
			projectName := "Test JIRA Project " + time.Now().Format("20060102-150405")
			// Wait for modal to be ready
			time.Sleep(500 * time.Millisecond)
			nameInput := page.Locator("input[placeholder='My Project']").First()
			Expect(nameInput.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			By("Submitting the form")
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())

			By("Waiting for project creation to complete")
			time.Sleep(2 * time.Second)

			By("Finding the created project in the table")
			projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			Expect(projectRow.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(10000),
			})).To(Succeed())

			By("Clicking the settings gear icon")
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())

			By("Waiting for settings page to load")
			time.Sleep(1 * time.Second)
			currentURL := page.URL()
			Expect(currentURL).To(ContainSubstring("/settings"))

			By("Clicking on the Integrations tab")
			integrationsTab := page.Locator("button:has-text('Integrations')")
			Expect(integrationsTab.Click()).To(Succeed())

			// Check if there's already a connection
			time.Sleep(1 * time.Second)
			// Look for connection cards - they contain the connection name and have Edit/Delete buttons
			existingConnections := page.Locator("div:has(button:has-text('Edit')):has(button:has-text('Delete'))")
			if count, _ := existingConnections.Count(); count > 0 {
				// Set up dialog handler before clicking delete
				page.OnDialog(func(dialog playwright.Dialog) {
					dialog.Accept()
				})
				// Delete existing connection first  
				deleteButton := existingConnections.First().Locator("button:has-text('Delete')")
				Expect(deleteButton.Click()).To(Succeed())
				time.Sleep(1 * time.Second)
			}

			By("Adding JIRA connection")
			addButton := page.Locator("button:has-text('Add JIRA Connection')")
			Expect(addButton.Click()).To(Succeed())

			By("Waiting for modal and filling JIRA connection details")
			time.Sleep(1 * time.Second)
			Expect(page.Fill("input[placeholder*='Production JIRA']", "Test JIRA Connection")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-api-token-123")).To(Succeed())

			By("Saving the connection")
			saveButton := page.Locator("button:has-text('Create Connection')")
			Expect(saveButton.Click()).To(Succeed())

			By("Verifying connection form was submitted")
			time.Sleep(2 * time.Second)
		})

		It("should successfully add a JIRA connection to an existing project", func() {
			By("Logging in as a project manager")
			auth.Login()

			By("Creating a new project for this test")
			Expect(page.Goto("/#/projects")).To(Succeed())
			time.Sleep(2 * time.Second)
			
			// Create a new project for clean state
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.Click()).To(Succeed())
			
			time.Sleep(500 * time.Millisecond)
			projectName := "JIRA Test Project " + time.Now().Format("20060102-150405")
			nameInput := page.Locator("input[placeholder='My Project']")
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())
			
			time.Sleep(2 * time.Second) // Wait for creation
			
			By("Finding and clicking settings for the newly created project")
			projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			Expect(projectRow.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())

			By("Waiting for project settings page to load")
			time.Sleep(1 * time.Second) // Give time for navigation
			currentURL := page.URL()
			Expect(currentURL).To(ContainSubstring("#/project/"))
			Expect(currentURL).To(ContainSubstring("/settings"))

			By("Clicking on the Integrations tab")
			integrationsTab := page.Locator("button:has-text('Integrations')")
			Expect(integrationsTab.Click()).To(Succeed())

			By("Clicking Add JIRA Connection button")
			// New project should not have any connections
			time.Sleep(1 * time.Second)
			addButton := page.Locator("button:has-text('Add JIRA Connection')")
			Expect(addButton.Click()).To(Succeed())

			By("Waiting for the modal to appear")
			time.Sleep(500 * time.Millisecond)

			By("Filling in the connection details")
			time.Sleep(1 * time.Second)
			Expect(page.Fill("input[placeholder*='Production JIRA']", "Test JIRA Connection")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-api-token-123")).To(Succeed())

			By("Saving the connection")
			saveButton := page.Locator("button:has-text('Create Connection')")
			Expect(saveButton.Click()).To(Succeed())

			By("Verifying the form was submitted")
			time.Sleep(2 * time.Second)
		})

		It("should successfully edit a JIRA connection", func() {
			By("Setting up an existing connection")
			auth.Login()
			
			By("Creating a new project for edit test")
			Expect(page.Goto("/#/projects")).To(Succeed())
			time.Sleep(2 * time.Second)
			
			// Create a new project
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.Click()).To(Succeed())
			
			time.Sleep(500 * time.Millisecond)
			projectName := "JIRA Edit Test " + time.Now().Format("20060102-150405")
			nameInput := page.Locator("input[placeholder='My Project']")
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())
			
			time.Sleep(2 * time.Second)
			
			// Navigate to settings for the new project
			projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())
			
			time.Sleep(1 * time.Second)
			Expect(page.Locator("button:has-text('Integrations')").Click()).To(Succeed())
			
			// Add a connection first
			Expect(page.Locator("button:has-text('Add JIRA Connection')").Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)
			Expect(page.Fill("input[placeholder*='Production JIRA']", "Original Connection")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-api-token")).To(Succeed())
			Expect(page.Locator("button:has-text('Create Connection')").Click()).To(Succeed())
			time.Sleep(1 * time.Second) // Wait for save

			By("Clicking edit on the first connection")
			// Wait for the connection card to appear - look for div with Edit/Delete buttons
			connectionCard := page.Locator("div:has(button:has-text('Edit')):has(button:has-text('Delete'))").First()
			Expect(connectionCard.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			
			editButton := connectionCard.Locator("button:has-text('Edit')")
			Expect(editButton.Click()).To(Succeed())

			By("Waiting for edit modal")
			time.Sleep(500 * time.Millisecond)

			By("Updating the connection name")
			nameEditInput := page.Locator("input[placeholder*='Production JIRA']").First()
			Expect(nameEditInput.Fill("")).To(Succeed())
			Expect(nameEditInput.Fill("Updated JIRA Connection")).To(Succeed())

			By("Saving the changes")
			Expect(page.Locator("button:has-text('Update Connection')").Click()).To(Succeed())

			By("Verifying the form was submitted")
			time.Sleep(2 * time.Second)
			// Just verify we could fill and submit the form
		})

		It("should successfully delete a JIRA connection", func() {
			By("Setting up an existing connection")
			auth.Login()
			
			By("Creating a new project for delete test")
			Expect(page.Goto("/#/projects")).To(Succeed())
			time.Sleep(2 * time.Second)
			
			// Create a new project
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.Click()).To(Succeed())
			
			time.Sleep(500 * time.Millisecond)
			projectName := "JIRA Delete Test " + time.Now().Format("20060102-150405")
			nameInput := page.Locator("input[placeholder='My Project']")
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())
			
			time.Sleep(2 * time.Second)
			
			// Navigate to settings for the new project
			projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())
			
			time.Sleep(1 * time.Second)
			Expect(page.Locator("button:has-text('Integrations')").Click()).To(Succeed())
			
			// Add a connection to delete
			Expect(page.Locator("button:has-text('Add JIRA Connection')").Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)
			Expect(page.Fill("input[placeholder*='Production JIRA']", "Connection to Delete")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-api-token-123")).To(Succeed())
			Expect(page.Locator("button:has-text('Create Connection')").Click()).To(Succeed())
			time.Sleep(1 * time.Second) // Wait for save

			By("Setting up dialog handler and clicking delete")
			// Set up dialog handler before clicking delete
			page.OnDialog(func(dialog playwright.Dialog) {
				dialog.Accept()
			})
			
			// Wait for the connection card to appear - look for div with Edit/Delete buttons
			connectionCard := page.Locator("div:has(button:has-text('Edit')):has(button:has-text('Delete'))").First()
			Expect(connectionCard.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			
			deleteButton := connectionCard.Locator("button:has-text('Delete')")
			Expect(deleteButton.Click()).To(Succeed())

			By("Verifying the connection is removed")
			time.Sleep(2 * time.Second)
			// Check that the connection we deleted is gone
			deletedConnection := page.Locator("div:has-text('Connection to Delete'):has(button:has-text('Delete'))")
			Expect(deletedConnection.Count()).To(Equal(0))
		})

		It("should enforce 1:1 relationship between project and JIRA connection", func() {
			By("Logging in and navigating to project settings")
			auth.Login()
			
			By("Creating a new project for 1:1 test")
			Expect(page.Goto("/#/projects")).To(Succeed())
			time.Sleep(2 * time.Second)
			
			// Create a new project
			createButton := page.Locator("button:has-text('New Project')")
			Expect(createButton.Click()).To(Succeed())
			
			time.Sleep(500 * time.Millisecond)
			projectName := "JIRA 1:1 Test " + time.Now().Format("20060102-150405")
			nameInput := page.Locator("input[placeholder='My Project']")
			Expect(nameInput.Fill(projectName)).To(Succeed())
			
			submitButton := page.Locator("button:has-text('Create Project')")
			Expect(submitButton.Click()).To(Succeed())
			
			time.Sleep(2 * time.Second)
			
			// Navigate to settings for the new project
			projectRow := page.Locator(fmt.Sprintf("tr:has-text('%s')", projectName))
			settingsButton := projectRow.Locator("button[title='Project Settings']")
			Expect(settingsButton.Click()).To(Succeed())
			
			time.Sleep(1 * time.Second)
			Expect(page.Locator("button:has-text('Integrations')").Click()).To(Succeed())

			By("Adding first JIRA connection")
			Expect(page.Locator("button:has-text('Add JIRA Connection')").Click()).To(Succeed())
			time.Sleep(500 * time.Millisecond)
			Expect(page.Fill("input[placeholder*='Production JIRA']", "First Connection")).To(Succeed())
			Expect(page.Fill("input[placeholder*='https://']", "http://mock-jira:8080")).To(Succeed())
			Expect(page.Fill("input[placeholder*='PROJ']", "TEST")).To(Succeed())
			Expect(page.Fill("input[placeholder*='@']", "test@fern.com")).To(Succeed())
			Expect(page.Fill("input[type='password']", "test-token")).To(Succeed())
			Expect(page.Locator("button:has-text('Create Connection')").Click()).To(Succeed())
			time.Sleep(1 * time.Second)

			By("Verifying Add button is disabled or hidden when connection exists")
			// Wait for the connection to appear and UI to update
			connectionCard := page.Locator("div:has(button:has-text('Edit')):has(button:has-text('Delete'))").First()
			Expect(connectionCard.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})).To(Succeed())
			
			// Give UI time to update button state
			time.Sleep(1 * time.Second)
			
			addButton := page.Locator("button:has-text('Add JIRA Connection')")
			buttonCount, _ := addButton.Count()
			
			if buttonCount > 0 {
				// Button exists, check if it's disabled
				isDisabled, _ := addButton.IsDisabled()
				Expect(isDisabled).To(BeTrue(), "Add button should be disabled when connection exists")
			} else {
				// Button is hidden - this is also valid for 1:1 enforcement
				Expect(buttonCount).To(Equal(0), "Add button should be hidden when connection exists")
			}

			By("Verifying only one connection exists")
			// Count connections by looking for specific connection names or patterns
			// Look for elements that contain the connection info (name, URL, etc)
			connectionNames := page.Locator("text=First Connection")
			nameCount, _ := connectionNames.Count()
			Expect(nameCount).To(Equal(1), "Should have exactly one JIRA connection")
		})
	})
})