package api_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
)

var _ = Describe("DomainHandler Validation", func() {
	Describe("Input Validation", func() {
		Context("Test Run Creation", func() {
			testCases := []testhelpers.ValidationTestCase{
				{
					Name: "valid test run creation request",
					Input: map[string]interface{}{
						"test_run_id":  "run-123",
						"project_id":   "project-123",
						"branch":       "main",
						"sha":          "abc123def456",
						"triggered_by": "ci",
					},
					ShouldPass: true,
				},
				{
					Name: "missing required test_run_id",
					Input: map[string]interface{}{
						"project_id":   "project-123",
						"branch":       "main",
						"sha":          "abc123def456",
						"triggered_by": "ci",
					},
					ShouldPass:  false,
					ErrorFields: []string{"test_run_id"},
				},
				{
					Name: "missing required project_id",
					Input: map[string]interface{}{
						"test_run_id":  "run-123",
						"branch":       "main",
						"sha":          "abc123def456",
						"triggered_by": "ci",
					},
					ShouldPass:  false,
					ErrorFields: []string{"project_id"},
				},
				{
					Name: "invalid triggered_by value",
					Input: map[string]interface{}{
						"test_run_id":  "run-123",
						"project_id":   "project-123",
						"branch":       "main",
						"sha":          "abc123def456",
						"triggered_by": "invalid_trigger",
					},
					ShouldPass:  false,
					ErrorFields: []string{"triggered_by"},
				},
			}

			testhelpers.RunValidationTests(validateTestRunCreation, testCases)
		})

		Context("Project Creation", func() {
			testCases := []testhelpers.ValidationTestCase{
				{
					Name: "valid project creation request",
					Input: map[string]interface{}{
						"project_id": "new-project",
						"name":       "New Project",
						"team":       "fern",
					},
					ShouldPass: true,
				},
				{
					Name: "valid project with description",
					Input: map[string]interface{}{
						"project_id":  "new-project",
						"name":        "New Project",
						"team":        "fern",
						"description": "A test project",
					},
					ShouldPass: true,
				},
				{
					Name: "missing project_id",
					Input: map[string]interface{}{
						"name": "New Project",
						"team": "fern",
					},
					ShouldPass:  false,
					ErrorFields: []string{"project_id"},
				},
				{
					Name: "invalid team value",
					Input: map[string]interface{}{
						"project_id": "new-project",
						"name":       "New Project",
						"team":       "invalid_team",
					},
					ShouldPass:  false,
					ErrorFields: []string{"team"},
				},
				{
					Name: "project_id with invalid characters",
					Input: map[string]interface{}{
						"project_id": "project with spaces",
						"name":       "New Project",
						"team":       "fern",
					},
					ShouldPass:  false,
					ErrorFields: []string{"project_id"},
				},
			}

			testhelpers.RunValidationTests(validateProjectCreation, testCases)
		})

		Context("Pagination Parameters", func() {
			testCases := []testhelpers.ValidationTestCase{
				{
					Name: "valid pagination",
					Input: map[string]interface{}{
						"limit":  10,
						"offset": 0,
					},
					ShouldPass: true,
				},
				{
					Name: "negative limit",
					Input: map[string]interface{}{
						"limit":  -1,
						"offset": 0,
					},
					ShouldPass:  false,
					ErrorFields: []string{"limit"},
				},
				{
					Name: "limit exceeds maximum",
					Input: map[string]interface{}{
						"limit":  1001,
						"offset": 0,
					},
					ShouldPass:  false,
					ErrorFields: []string{"limit"},
				},
				{
					Name: "negative offset",
					Input: map[string]interface{}{
						"limit":  10,
						"offset": -5,
					},
					ShouldPass:  false,
					ErrorFields: []string{"offset"},
				},
				{
					Name: "float64 limit (as from JSON unmarshalling)",
					Input: map[string]interface{}{
						"limit":  float64(10),
						"offset": float64(0),
					},
					ShouldPass: true,
				},
				{
					Name: "float64 negative limit",
					Input: map[string]interface{}{
						"limit":  float64(-1),
						"offset": float64(0),
					},
					ShouldPass:  false,
					ErrorFields: []string{"limit"},
				},
				{
					Name: "float64 non-whole number limit",
					Input: map[string]interface{}{
						"limit":  float64(10.5),
						"offset": float64(0),
					},
					ShouldPass:  false,
					ErrorFields: []string{"limit"},
				},
				{
					Name: "float64 non-whole number offset",
					Input: map[string]interface{}{
						"limit":  float64(10),
						"offset": float64(5.7),
					},
					ShouldPass:  false,
					ErrorFields: []string{"offset"},
				},
			}

			testhelpers.RunValidationTests(validatePagination, testCases)
		})
	})

	Describe("URL Parameter Validation", func() {
		It("should validate numeric IDs", func() {
			validIDs := []string{"1", "123", "999999"}
			for _, id := range validIDs {
				err := validateNumericID(id)
				Expect(err).NotTo(HaveOccurred())
			}

			invalidIDs := []string{"", "abc", "1.5", "-1", "1a", " 1 "}
			for _, id := range invalidIDs {
				err := validateNumericID(id)
				Expect(err).To(HaveOccurred())
			}
		})

		It("should validate project IDs", func() {
			validProjectIDs := []string{
				"project-123",
				"my_project",
				"test.project",
				"PROJECT-456",
			}
			for _, id := range validProjectIDs {
				err := validateProjectID(id)
				Expect(err).NotTo(HaveOccurred())
			}

			invalidProjectIDs := []string{
				"",
				"project with spaces",
				"project/with/slashes",
				"project@special",
			}
			for _, id := range invalidProjectIDs {
				err := validateProjectID(id)
				Expect(err).To(HaveOccurred())
			}
		})
	})
})

// Validation functions that would be in the actual handler
func validateTestRunCreation(input interface{}) error {
	data, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid input type")
	}

	// Check required fields
	if _, ok := data["test_run_id"]; !ok {
		return fmt.Errorf("test_run_id is required")
	}
	if _, ok := data["project_id"]; !ok {
		return fmt.Errorf("project_id is required")
	}

	// Validate triggered_by
	if triggeredBy, ok := data["triggered_by"].(string); ok {
		validTriggers := []string{"ci", "manual", "scheduled", "api"}
		valid := false
		for _, t := range validTriggers {
			if triggeredBy == t {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("triggered_by must be one of: ci, manual, scheduled, api")
		}
	}

	return nil
}

func validateProjectCreation(input interface{}) error {
	data, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid input type")
	}

	// Check required fields
	if _, ok := data["project_id"]; !ok {
		return fmt.Errorf("project_id is required")
	}
	if _, ok := data["name"]; !ok {
		return fmt.Errorf("name is required")
	}
	if _, ok := data["team"]; !ok {
		return fmt.Errorf("team is required")
	}

	// Validate project_id format
	if projectID, ok := data["project_id"].(string); ok {
		if err := validateProjectID(projectID); err != nil {
			return fmt.Errorf("invalid project_id: %w", err)
		}
	}

	// Validate team
	if team, ok := data["team"].(string); ok {
		validTeams := []string{"fern", "atmos", "admin"}
		valid := false
		for _, t := range validTeams {
			if team == t {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("team must be one of: fern, atmos, admin")
		}
	}

	return nil
}

func validatePagination(input interface{}) error {
	data, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid input type")
	}

	// Handle both int and float64 since JSON unmarshals numbers as float64
	if limitVal, exists := data["limit"]; exists {
		var limit int
		switch v := limitVal.(type) {
		case int:
			limit = v
		case float64:
			// Check if the float64 is a whole number
			if v != float64(int(v)) {
				return fmt.Errorf("limit must be a whole number")
			}
			limit = int(v)
		default:
			return fmt.Errorf("limit must be a number")
		}
		
		if limit < 0 {
			return fmt.Errorf("limit must be non-negative")
		}
		if limit > 1000 {
			return fmt.Errorf("limit cannot exceed 1000")
		}
	}

	if offsetVal, exists := data["offset"]; exists {
		var offset int
		switch v := offsetVal.(type) {
		case int:
			offset = v
		case float64:
			// Check if the float64 is a whole number
			if v != float64(int(v)) {
				return fmt.Errorf("offset must be a whole number")
			}
			offset = int(v)
		default:
			return fmt.Errorf("offset must be a number")
		}
		
		if offset < 0 {
			return fmt.Errorf("offset must be non-negative")
		}
	}

	return nil
}

func validateNumericID(id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}

	for _, r := range id {
		if r < '0' || r > '9' {
			return fmt.Errorf("ID must be numeric")
		}
	}

	return nil
}

func validateProjectID(id string) error {
	if id == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	// Allow alphanumeric, dash, underscore, and dot
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || 
			(r >= 'A' && r <= 'Z') || 
			(r >= '0' && r <= '9') || 
			r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("project ID contains invalid character: %c", r)
		}
	}

	return nil
}