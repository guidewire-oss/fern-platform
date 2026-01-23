package performance_test

import (
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
)

// GraphQL Query Size Regression Tests
// 
// These tests prevent regression of the 486MB monolithic query problem.
// See PERFORMANCE_FIX.md for details on the original issue.
//
// Test data requirements:
// - Run scripts/insert-test-data.sh to create ~100 test runs
// - This generates enough data to expose query inefficiencies

var _ = Describe("GraphQL Query Performance", Label("performance"), func() {
	var (
		page         playwright.Page
		context      playwright.BrowserContext
		totalBytes   int64
		responseURLs []string
	)

	BeforeEach(func() {
		var err error
		
		// Create new context and page
		context, err = browser.NewContext()
		Expect(err).NotTo(HaveOccurred())
		
		// Add authentication cookies if auth is enabled
		if authEnabled && len(savedCookies) > 0 {
			err = context.AddCookies(savedCookies)
			Expect(err).NotTo(HaveOccurred())
		}

		page, err = context.NewPage()
		Expect(err).NotTo(HaveOccurred())

		// Reset counters
		totalBytes = 0
		responseURLs = []string{}

		// Monitor GraphQL responses
		page.On("response", func(response playwright.Response) {
			url := response.URL()
			
			if strings.Contains(url, "/query") {
				// Track response size
				var responseSize int64
				headers := response.Headers()
				
				// Try to get size from Content-Length header first
				if contentLength, ok := headers["content-length"]; ok {
					bytes, err := strconv.ParseInt(contentLength, 10, 64)
					if err == nil {
						responseSize = bytes
					}
				}
				
				// Fallback: Get actual body length for chunked responses
				// This ensures we catch oversized payloads even without Content-Length header
				if responseSize == 0 {
					body, err := response.Body()
					if err == nil {
						responseSize = int64(len(body))
					}
				}
				
				if responseSize > 0 {
					totalBytes += responseSize
					responseURLs = append(responseURLs, url)
					
					// Log large responses for debugging
					if responseSize > 1024*1024 { // > 1MB
						GinkgoWriter.Printf("⚠️  Large GraphQL response: %.2f MB from %s\n", 
							float64(responseSize)/(1024*1024), url)
					}
				}
			}
		})
	})

	AfterEach(func() {
		if page != nil {
			page.Close()
		}
		if context != nil {
			context.Close()
		}
	})

	Describe("Page Load Performance", func() {
		Context("Projects Page", func() {
			It("should load in under 50KB total", func() {
				_, err := page.Goto(baseURL + "/web/#/projects")
				Expect(err).NotTo(HaveOccurred())

				// Wait for network to be idle (static assets loaded)
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for React to initialize and make GraphQL calls
				// The SPA loads static JS/CSS quickly and reaches "network idle",
				// but React needs additional time to parse, execute, render components,
				// and make GraphQL queries. Without this wait, we measure 0 responses.
				page.WaitForTimeout(2000) // 2 seconds for React initialization

				GinkgoWriter.Printf("Projects page - Total GraphQL data: %.2f KB (%d responses)\n",
					float64(totalBytes)/1024, len(responseURLs))

				// CRITICAL: Must have received at least one GraphQL response
				Expect(len(responseURLs)).To(BeNumerically(">", 0),
					"Should have received at least one GraphQL response - check if server is running and routes are correct")

				// With optimized GET_PROJECTS_LIST query, should be ~20KB
				// With old monolithic query, would be 486MB
				Expect(totalBytes).To(BeNumerically("<", 50*1024),
					"Projects page should load under 50KB (was 486MB with old query)")
			})
		})

		Context("Dashboard Page", func() {
			It("should load in under 200KB total", func() {
				_, err := page.Goto(baseURL + "/web/")
				Expect(err).NotTo(HaveOccurred())

				// Wait for network to be idle (static assets loaded)
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for React to initialize and make GraphQL calls
				// Dashboard makes multiple queries (summary, projects, recent runs)
				// which happen after React renders components
				page.WaitForTimeout(2000)

				GinkgoWriter.Printf("Dashboard - Total GraphQL data: %.2f KB (%d responses)\n",
					float64(totalBytes)/1024, len(responseURLs))

				// CRITICAL: Dashboard should make multiple GraphQL queries
				Expect(len(responseURLs)).To(BeNumerically(">=", 1),
					"Dashboard should make at least one GraphQL query - check if server is running")

				// With optimized queries (GET_DASHBOARD_SUMMARY + GET_PROJECTS_LIST + GET_RECENT_TEST_RUNS_SUMMARY)
				// should be ~122KB total
				// With old monolithic query, would be 486MB
				Expect(totalBytes).To(BeNumerically("<", 200*1024),
					"Dashboard should load under 200KB (was 486MB with old query)")
			})
		})

		Context("Test Runs Page", func() {
			It("should load in under 200KB total", func() {
				_, err := page.Goto(baseURL + "/web/#/test-runs")
				Expect(err).NotTo(HaveOccurred())

				// Wait for network to be idle (static assets loaded)
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for React to initialize and make GraphQL calls
				// Test runs page queries for list of test runs after React renders
				page.WaitForTimeout(2000)

				GinkgoWriter.Printf("Test Runs page - Total GraphQL data: %.2f KB (%d responses)\n",
					float64(totalBytes)/1024, len(responseURLs))

				// CRITICAL: Must have received GraphQL responses
				Expect(len(responseURLs)).To(BeNumerically(">", 0),
					"Should have received GraphQL responses - check if server is running and test data exists")

				// With optimized GET_RECENT_TEST_RUNS_SUMMARY query, should be ~100KB
				// With old monolithic query, would be 486MB
				Expect(totalBytes).To(BeNumerically("<", 200*1024),
					"Test Runs page should load under 200KB (was 486MB with old query)")
			})

			It("should NOT preload nested suite/spec data on initial load", func() {
				_, err := page.Goto(baseURL + "/web/#/test-runs")
				Expect(err).NotTo(HaveOccurred())

				// Wait for network to be idle (static assets loaded)
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for React to initialize and make initial GraphQL calls
				// We're checking that these initial calls don't include nested data
				page.WaitForTimeout(2000)

				// CRITICAL: Must have received GraphQL responses
				Expect(len(responseURLs)).To(BeNumerically(">", 0),
					"Should have received GraphQL responses")

				// If we're preloading nested data for 100 test runs, we'd see 5-10MB
				// The summary query should only load test run fields, not suiteRuns/specRuns
				Expect(totalBytes).To(BeNumerically("<", 500*1024),
					"Initial load should not include nested suite/spec data")
			})
		})
	})

	Describe("Regression Prevention", func() {
		It("should never return responses over 10MB on any page", func() {
			// Test all main pages
			pages := []string{"/web/", "/web/#/projects", "/web/#/test-runs", "/web/#/test-summaries"}
			
			for _, pagePath := range pages {
				totalBytes = 0 // Reset counter
				
				_, err := page.Goto(baseURL + pagePath)
				Expect(err).NotTo(HaveOccurred())
				
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for React to initialize and make GraphQL calls
				// The SPA loads static JS/CSS quickly and reaches "network idle",
				// but React needs additional time to parse, execute, render components,
				// and make GraphQL queries. Without this wait, we measure 0 responses.
				page.WaitForTimeout(2000) // 2 seconds for React initialization
				
				// No single page should ever load > 10MB
				// (The old bug loaded 486MB)
				Expect(totalBytes).To(BeNumerically("<", 10*1024*1024),
					"Page %s loaded %d MB - possible query regression!", 
					pagePath, totalBytes/(1024*1024))
			}
		})
	})
})

func init() {
	// Override baseURL from environment if set
	if url := os.Getenv("FERN_BASE_URL"); url != "" {
		baseURL = url
	}
}
