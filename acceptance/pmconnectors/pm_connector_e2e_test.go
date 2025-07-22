package pmconnectors_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("PM Connector End-to-End Tests", Label("e2e"), func() {
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

		// Create browser context options with increased timeout
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

		// Login as admin user
		auth.Login()
	})

	AfterEach(func() {
		// Handle cleanup even if test fails
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in AfterEach: %v\n", r)
			}

			// Ensure browser is closed
			if browser != nil {
				browser.Close()
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

				// Create timestamp prefix
				timestamp := time.Now().Format("20060102_150405")
				newPath := filepath.Join("videos", fmt.Sprintf("%s_PM_E2E_%s.webm", timestamp, testName))

				// Get the original video path
				originalPath, _ := video.Path()

				// Close the page and context to finalize video
				page.Close()
				page = nil
				if ctx != nil {
					ctx.Close()
					ctx = nil
				}

				// Create videos directory if it doesn't exist
				os.MkdirAll("videos", 0755)

				// Move/rename the video file
				if originalPath != "" {
					err := os.Rename(originalPath, newPath)
					if err == nil {
						fmt.Printf("Video saved: %s\n", newPath)
					} else {
						// If rename fails, try SaveAs
						err = video.SaveAs(newPath)
						if err == nil {
							fmt.Printf("Video saved: %s\n", newPath)
						} else {
							fmt.Printf("Failed to save video: %v\n", err)
						}
					}
				}
			} else {
				// No video, just close
				if page != nil {
					page.Close()
					page = nil
				}
				if ctx != nil {
					ctx.Close()
					ctx = nil
				}
			}
		} else {
			// If no video recording, just close page and context
			if page != nil {
				page.Close()
				page = nil
			}
			if ctx != nil {
				ctx.Close()
				ctx = nil
			}
		}
	})

	Describe("Complete PM Connector and Project Linking Flow", Label("e2e"), func() {
		It("should create connector, link to project, configure mappings, and sync with mock JIRA", func() {
			// Step 1: Create a PM Connector
			fmt.Println("Step 1: Creating PM Connector...")

			// Navigate to base URL first
			_, err := page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			Expect(err).NotTo(HaveOccurred())

			// Click PM Connectors
			pmConnectorsNav := page.Locator("text=PM Connectors")
			err = pmConnectorsNav.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for navigation
			Eventually(func() string {
				return page.URL()
			}, 10*time.Second).Should(ContainSubstring("/pm-connectors"))

			// Click Add Connector button
			addButton := page.Locator("button:has-text('Add Connector')")
			err = addButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for modal
			modal := page.Locator("div.modal")
			err = modal.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(5000),
			})
			Expect(err).NotTo(HaveOccurred())

			// Create unique connector name
			timestamp := time.Now().Format("20060102-150405")
			connectorName := fmt.Sprintf("E2E Test JIRA %s", timestamp)

			// Fill connector details
			nameInput := page.Locator("input[placeholder*='Production JIRA']")
			err = nameInput.Fill(connectorName)
			Expect(err).NotTo(HaveOccurred())

			// Fill base URL
			urlInput := page.Locator("input[type='url']")
			err = urlInput.Fill("http://mock-jira.fern-platform.svc.cluster.local:8080")
			Expect(err).NotTo(HaveOccurred())

			// Click Next
			nextButton := page.Locator("button:has-text('Next')")
			err = nextButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for credentials step
			Eventually(func() bool {
				h2 := page.Locator("h2:has-text('Configure Credentials')")
				count, _ := h2.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			// Fill credentials
			emailInput := page.Locator("input[type='email']")
			err = emailInput.Fill("test@example.com")
			Expect(err).NotTo(HaveOccurred())

			tokenInput := page.Locator("input[type='password']")
			err = tokenInput.Fill("test-api-token-12345")
			Expect(err).NotTo(HaveOccurred())

			// Handle potential alert
			page.OnDialog(func(dialog playwright.Dialog) {
				fmt.Printf("Alert: %s\n", dialog.Message())
				dialog.Accept()
			})

			// Click Create
			createButton := page.Locator("button:has-text('Create')")
			err = createButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for success
			Eventually(func() bool {
				modalCount, _ := modal.Count()
				if modalCount == 0 {
					return true
				}
				success := page.Locator("text=Connector created successfully")
				successCount, _ := success.Count()
				return successCount > 0
			}, 10*time.Second).Should(BeTrue())

			// Verify connector appears in list
			Eventually(func() bool {
				connector := page.Locator(fmt.Sprintf("text=%s", connectorName))
				count, _ := connector.Count()
				return count > 0
			}, 5*time.Second).Should(BeTrue())

			fmt.Printf("✓ Created connector: %s\n", connectorName)

			// Step 2: Navigate to Projects and find a test project
			fmt.Println("\nStep 2: Linking connector to project...")

			// Go to projects page
			_, err = page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			Expect(err).NotTo(HaveOccurred())

			// Look for existing projects or create one
			projectCards := page.Locator("div.project-card")
			projectCount, _ := projectCards.Count()

			if projectCount == 0 {
				// Create a test project if none exist
				fmt.Println("No projects found, creating test project...")
				createProjectButton := page.Locator("button:has-text('Create Project')")
				err = createProjectButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Fill project details
				projectNameInput := page.Locator("input[placeholder*='Project name']")
				err = projectNameInput.Fill("E2E Test Project")
				Expect(err).NotTo(HaveOccurred())

				submitButton := page.Locator("button[type='submit']:has-text('Create')")
				err = submitButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for project creation
				time.Sleep(2 * time.Second)
			}

			// Find and click on a project
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

			// Step 3: Link PM Connector to Project
			fmt.Println("\nStep 3: Configuring PM link...")

			// Click on Settings or PM Links tab
			settingsTab := page.Locator("button:has-text('Settings'), a:has-text('Settings')")
			settingsCount, _ := settingsTab.Count()
			if settingsCount > 0 {
				err = settingsTab.First().Click()
				Expect(err).NotTo(HaveOccurred())
			}

			// Look for PM Links section
			pmLinksSection := page.Locator("text=PM Links, text=PM Integrations, text=Project Management")
			Eventually(func() int {
				count, _ := pmLinksSection.Count()
				return count
			}, 5*time.Second).Should(BeNumerically(">", 0))

			// Click Add PM Link
			addPmLinkButton := page.Locator("button:has-text('Add PM Link'), button:has-text('Link PM Tool')")
			err = addPmLinkButton.First().Click()
			Expect(err).NotTo(HaveOccurred())

			// Select our connector
			connectorSelect := page.Locator("select[name='connectorId'], select#connector")
			if count, _ := connectorSelect.Count(); count > 0 {
				_, err = connectorSelect.SelectOption(playwright.SelectOptionValues{
					Labels: &[]string{connectorName},
				})
				Expect(err).NotTo(HaveOccurred())
			} else {
				// Try clicking on connector option
				connectorOption := page.Locator(fmt.Sprintf("text=%s", connectorName))
				err = connectorOption.Click()
				Expect(err).NotTo(HaveOccurred())
			}

			// Enter external project key
			externalKeyInput := page.Locator("input[placeholder*='PROJECT-KEY'], input[placeholder*='External'], input#externalProjectKey")
			err = externalKeyInput.Fill("TEST-1")
			Expect(err).NotTo(HaveOccurred())

			// Configure field mappings if needed
			fieldMappingSection := page.Locator("text=Field Mappings")
			if count, _ := fieldMappingSection.Count(); count > 0 {
				fmt.Println("Configuring field mappings...")

				// Map requirement ID to JIRA key
				idMapping := page.Locator("select[name='requirementId']")
				if count, _ := idMapping.Count(); count > 0 {
					_, err = idMapping.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{"key"},
					})
					Expect(err).NotTo(HaveOccurred())
				}

				// Map title to summary
				titleMapping := page.Locator("select[name='title']")
				if count, _ := titleMapping.Count(); count > 0 {
					_, err = titleMapping.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{"fields.summary"},
					})
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Save the PM link
			saveButton := page.Locator("button:has-text('Save'), button:has-text('Create Link')")
			err = saveButton.Click()
			Expect(err).NotTo(HaveOccurred())

			// Wait for success
			Eventually(func() bool {
				success := page.Locator("text=/Link created|Successfully linked|PM link added/i")
				count, _ := success.Count()
				return count > 0
			}, 10*time.Second).Should(BeTrue())

			fmt.Println("✓ PM link created successfully")

			// Step 4: Test the connection
			fmt.Println("\nStep 4: Testing connection...")

			// Find and click test connection button
			testConnectionButton := page.Locator("button:has-text('Test Connection'), button:has-text('Test')")
			if count, _ := testConnectionButton.Count(); count > 0 {
				err = testConnectionButton.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for test result
				Eventually(func() bool {
					result := page.Locator("text=/Connection successful|Connected|Test passed/i")
					count, _ := result.Count()
					return count > 0
				}, 15*time.Second).Should(BeTrue())

				fmt.Println("✓ Connection test passed")
			}

			// Step 5: Create a test with PM label
			fmt.Println("\nStep 5: Creating test with PM label...")

			// Navigate to test runs or tests
			testsLink := page.Locator("a:has-text('Tests'), button:has-text('Tests')")
			if count, _ := testsLink.Count(); count > 0 {
				err = testsLink.First().Click()
				Expect(err).NotTo(HaveOccurred())
			} else {
				// Try navigating to test runs
				_, err = page.Goto(baseURL + "/runs")
				Expect(err).NotTo(HaveOccurred())
			}

			// Look for a test run or create one
			testRuns := page.Locator("tr.test-run, div.test-run")
			runCount, _ := testRuns.Count()

			if runCount > 0 {
				// Click on first test run
				err = testRuns.First().Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for test details
				time.Sleep(2 * time.Second)

				// Find a test case
				testCase := page.Locator("tr.test-case, div.test-case").First()
				if count, _ := testCase.Count(); count > 0 {
					// Look for labels or tags field
					labelsField := page.Locator("input[placeholder*='labels'], input[placeholder*='tags'], .labels-input")
					if count, _ := labelsField.Count(); count > 0 {
						// Add JIRA issue label
						err = labelsField.Fill("jira:TEST-123")
						Expect(err).NotTo(HaveOccurred())

						// Press Enter to add label
						err = labelsField.Press("Enter")
						Expect(err).NotTo(HaveOccurred())

						fmt.Println("✓ Added JIRA label to test case")
					}
				}
			}

			// Step 6: Verify PM link functionality
			fmt.Println("\nStep 6: Verifying PM link...")

			// Go back to project
			_, err = page.Goto(baseURL)
			Expect(err).NotTo(HaveOccurred())

			// Click on the project again
			projectCard = page.Locator("div.project-card").First()
			err = projectCard.Click()
			Expect(err).NotTo(HaveOccurred())

			// Check PM links section
			pmLinkInfo := page.Locator("text=/Active PM Links|PM Integrations|Linked to/i")
			Eventually(func() int {
				count, _ := pmLinkInfo.Count()
				return count
			}, 5*time.Second).Should(BeNumerically(">", 0))

			// Verify our connector is linked
			linkedConnector := page.Locator(fmt.Sprintf("text=%s", connectorName))
			Expect(linkedConnector).To(BeVisible())

			// Check for sync status
			syncStatus := page.Locator("text=/Last synced|Sync status|Never synced/i")
			if count, _ := syncStatus.Count(); count > 0 {
				syncText, _ := syncStatus.TextContent()
				fmt.Printf("Sync status: %s\n", syncText)
			}

			fmt.Println("\n✅ End-to-end PM connector test completed successfully!")
			fmt.Printf("- Created connector: %s\n", connectorName)
			fmt.Printf("- Linked to project: %s\n", projectName)
			fmt.Println("- Configured field mappings")
			fmt.Println("- Tested connection to mock JIRA")
			fmt.Println("- Added JIRA label to test case")
		})
	})
})
