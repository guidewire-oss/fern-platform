package timestamp_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"

	"github.com/guidewire-oss/fern-platform/acceptance/helpers"
)

var _ = Describe("Timestamp Component", Label("acceptance", "ui", "timestamp"), func() {
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
		auth.Login()
	})

	AfterEach(func() {
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

	Context("Component Loading", func() {
		It("should load TimestampComponent globally", func() {
			// Execute JavaScript to check if component is loaded
			result, err := page.Evaluate("typeof window.TimestampComponent !== 'undefined'")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(true), "TimestampComponent should be available globally")
		})

		It("should have required methods available", func() {
			script := `() => {
				return window.TimestampComponent && 
				       typeof window.TimestampComponent.formatTimestamp === 'function' &&
				       typeof window.TimestampComponent.createElement === 'function' &&
				       typeof window.TimestampComponent.createRelativeElement === 'function';
			}`
			result, err := page.Evaluate(script)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(true), "TimestampComponent should have all required methods")
		})
	})

	Context("Timestamp Formatting", func() {
		It("should format timestamps correctly", func() {
			testTimestamp := "2025-01-25T14:30:00Z"
			script := fmt.Sprintf(`() => {
				var result = window.TimestampComponent.formatTimestamp('%s');
				return {
					hasLocalFormatted: !!result.localFormatted,
					hasUtcFormatted: !!result.utcFormatted,
					localLength: result.localFormatted.length,
					utcLength: result.utcFormatted.length
				};
			}`, testTimestamp)

			result, err := page.Evaluate(script)
			Expect(err).NotTo(HaveOccurred())

			resultMap := result.(map[string]interface{})
			Expect(resultMap["hasLocalFormatted"]).To(Equal(true))
			Expect(resultMap["hasUtcFormatted"]).To(Equal(true))
			Expect(resultMap["localLength"]).To(BeNumerically(">", 10))
			Expect(resultMap["utcLength"]).To(BeNumerically(">", 10))
		})

		It("should handle different timezone inputs", func() {
			testCases := []string{
				"2025-01-25T14:30:00Z",      // UTC
				"2025-01-25T09:30:00-05:00", // EST
				"2025-01-25T06:30:00-08:00", // PST
			}

			for _, testCase := range testCases {
				script := fmt.Sprintf(`() => {
					try {
						var result = window.TimestampComponent.formatTimestamp('%s');
						return { success: true, hasData: !!result.localFormatted };
					} catch (error) {
						return { success: false, error: error.message };
					}
				}`, testCase)

				result, err := page.Evaluate(script)
				Expect(err).NotTo(HaveOccurred())

				resultMap := result.(map[string]interface{})
				Expect(resultMap["success"]).To(Equal(true), fmt.Sprintf("Failed for timestamp: %s", testCase))
				Expect(resultMap["hasData"]).To(Equal(true))
			}
		})
	})

	Context("DOM Element Creation", func() {
		It("should create timestamp DOM elements", func() {
			script := `() => {
				var element = window.TimestampComponent.createElement('2025-01-25T14:30:00Z');
				document.body.appendChild(element);
				return {
					hasWrapper: element.classList.contains('timestamp-wrapper'),
					hasTooltip: !!element.querySelector('.timestamp-tooltip'),
					hasIcon: !!element.querySelector('.timestamp-icon')
				};
			}`

			result, err := page.Evaluate(script)
			Expect(err).NotTo(HaveOccurred())

			resultMap := result.(map[string]interface{})
			Expect(resultMap["hasWrapper"]).To(Equal(true))
			Expect(resultMap["hasTooltip"]).To(Equal(true))
			Expect(resultMap["hasIcon"]).To(Equal(true))
		})

		It("should create relative timestamp elements", func() {
			script := `() => {
				var pastTime = new Date(Date.now() - 5 * 60 * 1000).toISOString(); // 5 minutes ago
				var element = window.TimestampComponent.createRelativeElement(pastTime);
				document.body.appendChild(element);
				return {
					hasWrapper: element.classList.contains('timestamp-wrapper'),
					textContent: element.querySelector('.timestamp-local').textContent,
					hasTooltip: !!element.querySelector('.timestamp-tooltip')
				};
			}`

			result, err := page.Evaluate(script)
			Expect(err).NotTo(HaveOccurred())

			resultMap := result.(map[string]interface{})
			Expect(resultMap["hasWrapper"]).To(Equal(true))
			Expect(resultMap["textContent"]).To(ContainSubstring("minute"))
			Expect(resultMap["hasTooltip"]).To(Equal(true))
		})
	})

	Context("Hover Functionality", func() {
		It("should show tooltip on hover", func() {
			// Create a timestamp element
			_, err := page.Evaluate(`() => {
				var element = window.TimestampComponent.createElement('2025-01-25T14:30:00Z');
				element.id = 'test-timestamp';
				document.body.appendChild(element);
			}`)
			Expect(err).NotTo(HaveOccurred())

			// Find the element and hover over it
			element, err := page.QuerySelector("#test-timestamp")
			Expect(err).NotTo(HaveOccurred())

			// Hover over the element to trigger tooltip
			err = element.Hover()
			Expect(err).NotTo(HaveOccurred())

			// Wait for tooltip to become visible using Playwright's wait functionality
			tooltip := page.Locator("#test-timestamp .timestamp-tooltip")
			err = tooltip.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(2000), // 2 second timeout
			})
			Expect(err).NotTo(HaveOccurred(), "Tooltip should become visible on hover")
		})
	})

	Context("Timezone Information", func() {
		It("should provide user timezone info", func() {
			script := `() => {
				var info = window.TimestampComponent.getTimezoneInfo();
				return {
					hasTimeZone: !!info.timeZone,
					hasTimeZoneName: !!info.timeZoneName,
					hasOffset: typeof info.offset === 'number'
				};
			}`

			result, err := page.Evaluate(script)
			Expect(err).NotTo(HaveOccurred())

			resultMap := result.(map[string]interface{})
			Expect(resultMap["hasTimeZone"]).To(Equal(true))
			Expect(resultMap["hasTimeZoneName"]).To(Equal(true))
			Expect(resultMap["hasOffset"]).To(Equal(true))
		})
	})
})
