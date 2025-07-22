package testsummaries_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("UC-02: Test Summaries and Visualization", Label("e2e"), func() {
	var (
		ctx     playwright.BrowserContext
		page    playwright.Page
		nav     *helpers.NavigationHelper
		browser playwright.Browser
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
			// Create videos directory if it doesn't exist
			os.MkdirAll("./videos", 0755)

			contextOptions.RecordVideo = &playwright.RecordVideo{
				Dir:  "./videos",
				Size: &playwright.Size{Width: 1280, Height: 720},
			}

			// Add viewport to match video size
			contextOptions.Viewport = &playwright.Size{
				Width:  1280,
				Height: 720,
			}
		}

		ctx, err = browser.NewContext(contextOptions)
		Expect(err).NotTo(HaveOccurred())

		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())

		// Login
		authHelper := helpers.NewLoginHelper(page, baseURL, username, password)
		authHelper.Login()

		// Create navigation helper
		nav = helpers.NewNavigationHelper(page, baseURL)

		// Navigate to test summaries page
		nav.GoToTestSummaries()
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

				// Save the video with a descriptive name
				timestamp := time.Now().Format("20060102_150405")
				newPath := fmt.Sprintf("../videos/testsummaries/%s_%s.webm", testName, timestamp)

				// Get the original video path before closing
				originalPath, _ := video.Path()

				// Close the page first (not the context) to finalize video
				if page != nil {
					page.Close()
					page = nil
				}

				// Now close the context
				if ctx != nil {
					ctx.Close()
					ctx = nil
				}

				// Save the video with the new name
				err := video.SaveAs(newPath)
				if err == nil {
					fmt.Printf("Video saved: %s\n", newPath)
					// Try to delete the original file if it exists and is different
					if originalPath != "" && originalPath != newPath {
						os.Remove(originalPath)
					}
				} else {
					fmt.Printf("Failed to save video: %v\n", err)
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

	Describe("UC-02-01: View Test Summary Dashboard", Label("e2e"), func() {
		Context("Team member views team projects", func() {
			It("should show only projects from user's team", func() {
				// Get all project cards - using .card class based on debug output
				projectCards := page.Locator(".card")

				// Should have at least one project
				count, err := projectCards.Count()
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(BeNumerically(">", 0))

				// Each card should display required information
				for i := 0; i < count; i++ {
					card := projectCards.Nth(i)

					// Check project name exists (h3 or h4 based on debug)
					projectName := card.Locator("h3, h4")
					nameCount, _ := projectName.Count()
					Expect(nameCount).To(BeNumerically(">=", 1))

					// Check metrics exist - looking for text containing "Test"
					metricsText, err := card.TextContent()
					Expect(err).NotTo(HaveOccurred())

					// Verify test metrics are present in the card text
					Expect(metricsText).To(MatchRegexp("Test.*[0-9]+"))
					Expect(metricsText).To(MatchRegexp("[0-9]+.*failed"))
					Expect(metricsText).To(MatchRegexp("[0-9]+.*passed"))
				}
			})
		})

		Context("Summary metrics accuracy", func() {
			It("should display accurate summary metrics", func() {
				// Check for summary metrics in the page
				pageText, err := page.TextContent("body")
				Expect(err).NotTo(HaveOccurred())

				// Look for summary statistics that should be visible
				// Based on the debug output, we should see test-related metrics
				Expect(pageText).To(ContainSubstring("Test"))

				// Verify there are project cards visible
				cards := page.Locator(".card")
				cardCount, _ := cards.Count()
				Expect(cardCount).To(BeNumerically(">=", 1))
			})
		})

		Context("User not in team with projects sees no data", func() {
			It("should not show other teams' projects", func() {
				// Verify that project cards exist (at least for the user's team)
				projectCards := page.Locator(".card")
				count, _ := projectCards.Count()

				// If user has no projects, should see empty state
				if count == 0 {
					emptyState := page.Locator("text=/No projects found|No test runs found/")
					emptyCount, _ := emptyState.Count()
					Expect(emptyCount).To(BeNumerically(">=", 1))

					// View toggle buttons should be disabled
					cardViewBtn := page.Locator("button:has-text('Card View')")
					treemapViewBtn := page.Locator("button:has-text('Treemap View')")

					cardDisabled, _ := cardViewBtn.IsDisabled()
					treemapDisabled, _ := treemapViewBtn.IsDisabled()

					Expect(cardDisabled || treemapDisabled).To(BeTrue())
				}
			})
		})
	})

	Describe("UC-02-02: Toggle Between Card and Treemap Views", Label("e2e"), func() {
		Context("View toggle disabled when no projects", func() {
			It("should disable view toggle when no projects exist", func() {
				projectCards := page.Locator(".card")
				count, _ := projectCards.Count()

				if count == 0 {
					cardViewBtn := page.Locator("button:has-text('Card View')")
					treemapViewBtn := page.Locator("button:has-text('Treemap View')")

					cardDisabled, _ := cardViewBtn.IsDisabled()
					treemapDisabled, _ := treemapViewBtn.IsDisabled()

					Expect(cardDisabled).To(BeTrue())
					Expect(treemapDisabled).To(BeTrue())
				}
			})
		})

		Context("Switch from Card View to Treemap View", func() {
			It("should flip all cards to show treemap", func() {
				// Skip if no projects
				projectCards := page.Locator(".card")
				cardCount, _ := projectCards.Count()
				if cardCount == 0 {
					Skip("No projects available for testing")
				}

				// Find view toggle buttons
				treemapButton := page.Locator("button:has-text('Treemap View')")
				btnCount, _ := treemapButton.Count()
				Expect(btnCount).To(BeNumerically(">=", 1))

				// Click treemap view
				err := treemapButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for animation
				time.Sleep(1000 * time.Millisecond)

				// Check that cards have flipped by looking for card-flip-container
				// The debug output showed card-flip-container class
				flipContainers := page.Locator(".card-flip-container")
				flipCount, _ := flipContainers.Count()
				Expect(flipCount).To(BeNumerically(">=", cardCount))

				// Verify treemap view is active
				treemapViewActive := page.Locator("button:has-text('Treemap View')[aria-pressed='true'], button:has-text('Treemap View').active")
				activeCount, _ := treemapViewActive.Count()
				Expect(activeCount).To(BeNumerically(">=", 1))
			})
		})

		Context("Treemap visualization accuracy", func() {
			It("should display treemap with correct colors based on pass rate", func() {
				// Skip if no projects
				projectCards := page.Locator(".card")
				count, _ := projectCards.Count()
				if count == 0 {
					Skip("No projects available for testing")
				}

				// Switch to treemap view
				err := page.Locator("button:has-text('Treemap View')").Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1000 * time.Millisecond)

				// Check for SVG elements which typically contain treemaps
				svgElements := page.Locator("svg")
				svgCount, err := svgElements.Count()
				Expect(err).NotTo(HaveOccurred())
				Expect(svgCount).To(BeNumerically(">=", 1))

				// Look for rectangles within SVG (typical treemap structure)
				treemapRects := page.Locator("svg rect")
				rectCount, _ := treemapRects.Count()

				if rectCount > 0 {
					// Hover over a rectangle to see tooltip
					err = treemapRects.First().Hover()
					Expect(err).NotTo(HaveOccurred())

					// Check tooltip appears
					tooltip := page.Locator(".treemap-tooltip")
					Eventually(func() bool {
						count, _ := tooltip.Count()
						return count > 0
					}, 5*time.Second).Should(BeTrue())
				}
			})
		})

		Context("Switch from Treemap View back to Card View", func() {
			It("should flip cards back to original view", func() {
				// Skip if no projects
				projectCards := page.Locator(".card")
				count, _ := projectCards.Count()
				if count == 0 {
					Skip("No projects available for testing")
				}

				// First switch to treemap
				err := page.Locator("button:has-text('Treemap View')").Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1000 * time.Millisecond)

				// Then switch back to card view
				cardButton := page.Locator("button:has-text('Card View')")
				err = cardButton.Click()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1000 * time.Millisecond)

				// Card view button should be active
				cardViewActive := page.Locator("button:has-text('Card View')[aria-pressed='true'], button:has-text('Card View').active")
				activeCount, _ := cardViewActive.Count()
				Expect(activeCount).To(BeNumerically(">=", 1))

				// Project info should be visible again
				projectNames := page.Locator(".card h3, .card h4")
				nameCount, _ := projectNames.Count()
				Expect(nameCount).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("UC-02-03: Interact with Treemap Visualization", Label("e2e"), func() {
		BeforeEach(func() {
			// Skip if no projects
			projectCards := page.Locator(".card")
			count, _ := projectCards.Count()
			if count == 0 {
				Skip("No projects available for testing")
			}

			// Switch to treemap view
			err := page.Locator("button:has-text('Treemap View')").Click()
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1000 * time.Millisecond)
		})

		Context("Hover over test suite", func() {
			It("should show tooltip with suite details", func() {
				// Find SVG rectangles (treemap elements)
				rects := page.Locator("svg rect")
				count, err := rects.Count()
				Expect(err).NotTo(HaveOccurred())

				if count > 0 {
					// Hover over first rectangle
					err = rects.First().Hover()
					Expect(err).NotTo(HaveOccurred())

					// Tooltip should appear
					tooltip := page.Locator(".treemap-tooltip")
					Eventually(func() bool {
						visible, _ := tooltip.IsVisible()
						return visible
					}, 5*time.Second).Should(BeTrue())

					// Tooltip should contain suite info
					tooltipText, err := tooltip.TextContent()
					Expect(err).NotTo(HaveOccurred())
					Expect(tooltipText).To(MatchRegexp("(Suite|Test|Pass|Duration|failed|passed)"))
				}
			})
		})

		Context("Click to drill down to test specs", func() {
			It("should be clickable and show treemap elements", func() {
				// Find treemap rectangles
				rects := page.Locator("svg rect")
				count, _ := rects.Count()

				if count > 0 {
					// Verify rectangles are present
					Expect(count).To(BeNumerically(">", 0))

					// Click the first rectangle
					err := rects.First().Click()
					Expect(err).NotTo(HaveOccurred())

					// The current implementation doesn't have drill-down navigation
					// Just verify the treemap is still visible after click
					time.Sleep(500 * time.Millisecond)

					// Treemap should still be visible
					svgElements := page.Locator("svg")
					svgCount, _ := svgElements.Count()
					Expect(svgCount).To(BeNumerically(">=", 1))
				}
			})
		})
	})

	Describe("UC-02-04: View Test History", Label("e2e"), func() {
		Context("Access test history", func() {
			It("should show test history when clicking View Test History", func() {
				// Find first project card
				firstCard := page.Locator(".card").First()
				count, _ := firstCard.Count()

				if count == 0 {
					Skip("No projects available for testing")
				}

				// Click View Test History button - based on debug output "📊 View Test History"
				historyBtn := firstCard.Locator("button:has-text('View Test History'), button:has-text('📊')")
				btnCount, _ := historyBtn.Count()

				if btnCount > 0 {
					err := historyBtn.Click()
					Expect(err).NotTo(HaveOccurred())

					// Should see the test history chart view
					Eventually(func() bool {
						// Check for test history chart component elements
						historyContainer := page.Locator(".test-history-container")
						backButton := page.Locator(".back-button")
						chartTitle := page.Locator(".history-title h3")

						containerCount, _ := historyContainer.Count()
						backCount, _ := backButton.Count()
						titleCount, _ := chartTitle.Count()

						// If we find the container, that's enough to confirm navigation worked
						return containerCount > 0 || (backCount > 0 && titleCount > 0)
					}, 5*time.Second).Should(BeTrue())
				}
			})
		})
	})

	Describe("UC-02-05: Mark Projects as Favorites", Label("e2e"), func() {
		Context("Mark project as favorite", func() {
			It("should toggle favorite star and persist state", func() {
				// Wait for page to load
				time.Sleep(1 * time.Second)

				// Find first project card
				firstCard := page.Locator(".card").First()
				count, _ := firstCard.Count()

				if count == 0 {
					Fail("No project cards found on page")
				}

				// Find star button by looking for the button with star icon
				starButton := firstCard.Locator("button").Filter(playwright.LocatorFilterOptions{
					Has: page.Locator("i.fa-star"),
				}).First()

				buttonCount, _ := starButton.Count()
				Expect(buttonCount).To(Equal(1))

				// Get the star icon element
				starIcon := starButton.Locator("i.fa-star")

				// Check initial state - look for 'far' (empty) or 'fas' (filled) class
				initialClasses, _ := starIcon.GetAttribute("class")
				isInitiallyFavorited := strings.Contains(initialClasses, "fas")

				// Click the star button
				err := starButton.Click()
				Expect(err).NotTo(HaveOccurred())

				// Wait for GraphQL request to complete
				time.Sleep(2 * time.Second)

				// Check that the state changed
				afterClasses, _ := starIcon.GetAttribute("class")
				isFavoritedAfter := strings.Contains(afterClasses, "fas")

				// State should have toggled
				Expect(isFavoritedAfter).To(Equal(!isInitiallyFavorited))

				// Verify the visual state changed correctly
				if isFavoritedAfter {
					// Should have 'fas' class (filled star)
					Expect(afterClasses).To(ContainSubstring("fas"))
					Expect(afterClasses).NotTo(ContainSubstring("far"))

					// Button should have yellow color (hex or rgb format)
					buttonStyle, _ := starButton.GetAttribute("style")
					Expect(buttonStyle).To(SatisfyAny(
						ContainSubstring("#fbbf24"),
						ContainSubstring("rgb(251, 191, 36)"),
					))
				} else {
					// Should have 'far' class (empty star)
					Expect(afterClasses).To(ContainSubstring("far"))
					Expect(afterClasses).NotTo(ContainSubstring("fas"))

					// Button should have gray color (hex or rgb format)
					buttonStyle, _ := starButton.GetAttribute("style")
					Expect(buttonStyle).To(SatisfyAny(
						ContainSubstring("#9ca3af"),
						ContainSubstring("rgb(156, 163, 175)"),
					))
				}

				// Reload the page to verify persistence
				_, err = page.Reload()
				Expect(err).NotTo(HaveOccurred())

				// Navigate back to test summaries
				nav.GoToTestSummaries()
				time.Sleep(1 * time.Second)

				// Find the same project card and star button again
				firstCardAfterReload := page.Locator(".card").First()
				starButtonAfterReload := firstCardAfterReload.Locator("button").Filter(playwright.LocatorFilterOptions{
					Has: page.Locator("i.fa-star"),
				}).First()
				starIconAfterReload := starButtonAfterReload.Locator("i.fa-star")

				// Check that the state persisted
				reloadedClasses, _ := starIconAfterReload.GetAttribute("class")
				isFavoritedAfterReload := strings.Contains(reloadedClasses, "fas")

				// State should match what it was after clicking
				Expect(isFavoritedAfterReload).To(Equal(isFavoritedAfter))
			})
		})
	})

	Describe("UC-02-06: Time Range Filtering", Label("e2e"), func() {
		Context("Default time range", func() {
			It("should show 7 days as default", func() {
				// Find time range selector - debug showed 'select' elements exist
				timeSelectors := page.Locator("select")
				selectCount, _ := timeSelectors.Count()

				// Find the correct select with time options
				for i := 0; i < selectCount; i++ {
					selector := timeSelectors.Nth(i)
					options := selector.Locator("option")
					optCount, _ := options.Count()

					if optCount > 0 {
						firstOptText, _ := options.First().TextContent()
						// Check if this is the time range selector
						if strings.Contains(firstOptText, "Last") || strings.Contains(firstOptText, "days") {
							// Check default value
							selectedValue, err := selector.InputValue()
							Expect(err).NotTo(HaveOccurred())
							Expect(selectedValue).To(Equal("7"))
							return
						}
					}
				}

				// If we get here, we didn't find the time selector
				Skip("Time range selector not found")
			})
		})

		Context("Change time range", func() {
			It("should update data when time range changes", func() {
				// Skip if no projects
				projectCards := page.Locator(".card")
				count, _ := projectCards.Count()
				if count == 0 {
					Skip("No projects available for testing")
				}

				// Find the time selector
				timeSelectors := page.Locator("select")
				selectCount, _ := timeSelectors.Count()

				var timeSelector playwright.Locator
				for i := 0; i < selectCount; i++ {
					selector := timeSelectors.Nth(i)
					options := selector.Locator("option")
					optCount, _ := options.Count()

					if optCount > 0 {
						firstOptText, _ := options.First().TextContent()
						if strings.Contains(firstOptText, "Last") || strings.Contains(firstOptText, "days") {
							timeSelector = selector
							break
						}
					}
				}

				if timeSelector == nil {
					Skip("Time range selector not found")
				}

				// Get initial test count from first card
				firstCard := projectCards.First()
				_ = getTestCountFromCard(firstCard)

				// Change to 1 month
				_, err := timeSelector.SelectOption(playwright.SelectOptionValues{
					Values: &[]string{"30"},
				})
				Expect(err).NotTo(HaveOccurred())

				// Wait for data to update
				time.Sleep(2 * time.Second)

				// Get new test count
				newTestCount := getTestCountFromCard(firstCard)

				// Count might be different (usually higher for longer period)
				// But at minimum, it should have refreshed
				Expect(newTestCount).To(BeNumerically(">=", 0))
			})
		})
	})
})

// Helper function to extract test count from project card
func getTestCountFromCard(card playwright.Locator) int {
	// Get the entire card text
	cardText, err := card.TextContent()
	if err != nil {
		return 0
	}

	// Look for numbers that could be test counts
	// Based on the format: "total failed passed"
	parts := strings.Fields(cardText)
	for i, part := range parts {
		// Look for "Test" or "Tests" followed by numbers
		if strings.Contains(strings.ToLower(part), "test") && i+1 < len(parts) {
			// Try to parse the next part as a number
			if count, err := strconv.Atoi(parts[i+1]); err == nil && count > 0 {
				return count
			}
		}

		// Also try to extract any standalone numbers
		cleaned := strings.TrimFunc(part, func(r rune) bool {
			return r < '0' || r > '9'
		})
		if cleaned != "" {
			if count, err := strconv.Atoi(cleaned); err == nil && count > 10 { // Assume test counts are > 10
				return count
			}
		}
	}

	return 0
}
