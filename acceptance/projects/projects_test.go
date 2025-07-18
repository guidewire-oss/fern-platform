package projects_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/playwright-community/playwright-go"
)

// Helper matchers for Playwright
func BeVisible() OmegaMatcher {
	return WithTransform(func(locator playwright.Locator) bool {
		visible, _ := locator.IsVisible()
		return visible
	}, BeTrue())
}

func BeDisabled() OmegaMatcher {
	return WithTransform(func(locator playwright.Locator) bool {
		disabled, _ := locator.IsDisabled()
		return disabled
	}, BeTrue())
}

func BeEnabled() OmegaMatcher {
	return WithTransform(func(locator playwright.Locator) bool {
		enabled, _ := locator.IsEnabled()
		return enabled
	}, BeTrue())
}

var _ = Describe("UC-04: Project Management", func() {
	var (
		browser playwright.Browser
		ctx     playwright.BrowserContext
		page    playwright.Page
		err     error
	)

	BeforeEach(func() {
		browser = CreateBrowser()
		ctx, err = browser.NewContext(contextOptions)
		Expect(err).NotTo(HaveOccurred())
		page, err = ctx.NewPage()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		SaveVideoOnFailure(ctx, CurrentSpecReport())
		ctx.Close()
		browser.Close()
	})

	Describe("UC-04-03: Delete Project with Cascade Delete", func() {
		Context("As a project administrator", func() {
			BeforeEach(func() {
				// Login as admin
				page.Goto(baseURL + "/")
				
				// Click sign in
				signInBtn := page.Locator("button:has-text('Sign in'), a:has-text('Sign in')")
				Expect(signInBtn.Click()).To(Succeed())
				
				// OAuth login flow
				page.WaitForURL("**/auth/realms/**")
				page.Fill("input[name='username']", "admin@fern.com")
				page.Fill("input[name='password']", "test123")
				page.Click("input[type='submit']")
				
				// Wait for redirect back
				Eventually(func() bool {
					url := page.URL()
					return strings.HasPrefix(url, baseURL)
				}, 10*time.Second).Should(BeTrue())
				
				// Navigate to projects page
				page.Click("text=Projects")
				page.WaitForLoadState()
			})

			It("should show confirmation dialog when deleting a project", func() {
				// Find a project with delete button
				projectCard := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
					HasText: "Fern Platform",
				}).First()
				
				// Click delete button
				deleteBtn := projectCard.Locator("button:has-text('🗑️')")
				Expect(deleteBtn.Click()).To(Succeed())
				
				// Verify confirmation dialog appears
				modal := page.Locator("div").Filter(playwright.LocatorFilterOptions{
					HasText: "Delete Project",
				}).First()
				Expect(modal).To(BeVisible())
				
				// Verify warning message
				Expect(page.Locator("text=This action will permanently delete:")).To(BeVisible())
				Expect(page.Locator("text=All test runs associated with this project")).To(BeVisible())
				Expect(page.Locator("text=This action cannot be undone")).To(BeVisible())
				
				// Verify project name confirmation input
				confirmInput := page.Locator("input[placeholder='Type project name to confirm']")
				Expect(confirmInput).To(BeVisible())
				
				// Verify delete button is disabled
				deleteConfirmBtn := modal.Locator("button:has-text('Delete Project')")
				Expect(deleteConfirmBtn).To(BeDisabled())
			})

			It("should enable delete button only when project name matches", func() {
				// Find a project with delete button
				projectCard := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
					HasText: "Fern Platform",
				}).First()
				
				// Click delete button
				deleteBtn := projectCard.Locator("button:has-text('🗑️')")
				Expect(deleteBtn.Click()).To(Succeed())
				
				// Get the modal
				modal := page.Locator("div").Filter(playwright.LocatorFilterOptions{
					HasText: "Delete Project",
				}).First()
				
				// Type wrong project name
				confirmInput := page.Locator("input[placeholder='Type project name to confirm']")
				confirmInput.Fill("Wrong Name")
				
				// Verify delete button is still disabled
				deleteConfirmBtn := modal.Locator("button:has-text('Delete Project')")
				Expect(deleteConfirmBtn).To(BeDisabled())
				
				// Type correct project name
				confirmInput.Fill("Fern Platform")
				
				// Verify delete button is now enabled
				Expect(deleteConfirmBtn).To(BeEnabled())
			})

			It("should cancel deletion when cancel button is clicked", func() {
				// Count initial projects
				initialCount, _ := page.Locator("tr").Count()
				
				// Find a project with delete button
				projectCard := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
					HasText: "Fern Platform",
				}).First()
				
				// Click delete button
				deleteBtn := projectCard.Locator("button:has-text('🗑️')")
				Expect(deleteBtn.Click()).To(Succeed())
				
				// Click cancel
				cancelBtn := page.Locator("button:has-text('Cancel')")
				Expect(cancelBtn.Click()).To(Succeed())
				
				// Verify modal is closed
				modal := page.Locator("div").Filter(playwright.LocatorFilterOptions{
					HasText: "Delete Project",
				})
				Expect(modal).NotTo(BeVisible())
				
				// Verify project still exists
				finalCount, _ := page.Locator("tr").Count()
				Expect(finalCount).To(Equal(initialCount))
			})

			It("should delete project and all associated data when confirmed", func() {
				// Create a test project first
				page.Click("button:has-text('New Project')")
				page.WaitForSelector("text=Create New Project")
				
				projectName := fmt.Sprintf("Test Project %d", time.Now().Unix())
				page.Fill("input[placeholder='My Project']", projectName)
				page.Fill("textarea", "Test project for deletion")
				page.SelectOption("select", playwright.SelectOptionValues{Values: &[]string{"fern"}}) // Select team
				page.Click("button:has-text('Create Project')")
				
				// Wait for project to be created
				page.WaitForSelector(fmt.Sprintf("text=%s", projectName))
				
				// Now delete it
				projectCard := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
					HasText: projectName,
				}).First()
				
				// Click delete button
				deleteBtn := projectCard.Locator("button:has-text('🗑️')")
				Expect(deleteBtn.Click()).To(Succeed())
				
				// Type project name to confirm
				confirmInput := page.Locator("input[placeholder='Type project name to confirm']")
				confirmInput.Fill(projectName)
				
				// Click delete
				deleteConfirmBtn := page.Locator("button:has-text('Delete Project')").Last()
				Expect(deleteConfirmBtn.Click()).To(Succeed())
				
				// Wait for deletion to complete
				page.WaitForLoadState()
				
				// Verify project is gone
				Eventually(func() bool {
					count, _ := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
						HasText: projectName,
					}).Count()
					projectExists := count > 0
					return !projectExists
				}, 10*time.Second).Should(BeTrue())
			})
		})

		Context("As a regular user", func() {
			BeforeEach(func() {
				// Login as regular user
				page.Goto(baseURL + "/")
				
				// Click sign in
				signInBtn := page.Locator("button:has-text('Sign in'), a:has-text('Sign in')")
				Expect(signInBtn.Click()).To(Succeed())
				
				// OAuth login flow
				page.WaitForURL("**/auth/realms/**")
				page.Fill("input[name='username']", "fern-user@fern.com")
				page.Fill("input[name='password']", "test123")
				page.Click("input[type='submit']")
				
				// Wait for redirect back
				Eventually(func() bool {
					url := page.URL()
					return strings.HasPrefix(url, baseURL)
				}, 10*time.Second).Should(BeTrue())
				
				// Navigate to projects page
				page.Click("text=Projects")
				page.WaitForLoadState()
			})

			It("should not show delete button for view-only users", func() {
				// Find a project
				projectCard := page.Locator("tr").Filter(playwright.LocatorFilterOptions{
					HasText: "Fern Platform",
				}).First()
				
				// Verify no delete button is visible
				deleteBtn := projectCard.Locator("button:has-text('🗑️')")
				Expect(deleteBtn).NotTo(BeVisible())
				
				// Verify "View only" text is shown
				Expect(projectCard.Locator("text=View only")).To(BeVisible())
			})
		})
	})
})