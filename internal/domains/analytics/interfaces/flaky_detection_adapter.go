package interfaces

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// FlakyDetectionAdapter provides HTTP handlers for flaky test detection
type FlakyDetectionAdapter struct {
	service *application.FlakyDetectionService
	logger  *logging.Logger
}

// errInvalidRowID rejects a path parameter that is not a usable row ID.
var errInvalidRowID = errors.New("invalid flaky test row ID")

// parseFlakyTestRowID parses a path parameter into a row ID. 63 bits, not 64:
// the column is BIGSERIAL
func parseFlakyTestRowID(raw string) (uint, error) {
	id, err := strconv.ParseUint(raw, 10, 63)
	if err != nil || id == 0 || uint64(uint(id)) != id {
		return 0, errInvalidRowID
	}
	return uint(id), nil
}

// NewFlakyDetectionAdapter creates a new flaky detection adapter
func NewFlakyDetectionAdapter(service *application.FlakyDetectionService, logger *logging.Logger) *FlakyDetectionAdapter {
	return &FlakyDetectionAdapter{
		service: service,
		logger:  logger,
	}
}

// AnalyzeTestRun handles POST /api/v1/projects/:projectId/test-runs/:testRunId/analyze
func (a *FlakyDetectionAdapter) AnalyzeTestRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		testRunID := c.Param("testRunId")

		if projectID == "" || testRunID == "" {
			c.JSON(400, gin.H{"error": "project ID and test run ID are required"})
			return
		}

		analysis, err := a.service.AnalyzeTestRun(c.Request.Context(), projectID, testRunID)
		if err != nil {
			a.logger.WithError(err).Error("Failed to analyze test run")
			c.JSON(500, gin.H{"error": "Failed to analyze test run"})
			return
		}

		c.JSON(200, gin.H{
			"analysis": analysis,
		})
	}
}

// GetFlakyTests handles GET /api/v1/projects/:projectId/flaky-tests
func (a *FlakyDetectionAdapter) GetFlakyTests() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(400, gin.H{"error": "project ID is required"})
			return
		}

		flakyTests, err := a.service.GetFlakyTests(c.Request.Context(), projectID)
		if err != nil {
			a.logger.WithError(err).Error("Failed to get flaky tests")
			c.JSON(500, gin.H{"error": "Failed to get flaky tests"})
			return
		}

		c.JSON(200, gin.H{
			"flaky_tests": flakyTests,
			"total":       len(flakyTests),
		})
	}
}

// MarkTestResolved handles PUT /api/v1/flaky-tests/:testId/resolve
func (a *FlakyDetectionAdapter) MarkTestResolved() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseFlakyTestRowID(c.Param("testId"))
		if err != nil {
			c.JSON(400, gin.H{"error": "test ID must be a positive integer"})
			return
		}

		if err := a.service.MarkTestResolved(c.Request.Context(), id); err != nil {
			if errors.Is(err, domain.ErrFlakyTestNotFound) {
				c.JSON(404, gin.H{"error": "Flaky test not found"})
				return
			}
			a.logger.WithError(err).Error("Failed to mark test as resolved")
			c.JSON(500, gin.H{"error": "Failed to mark test as resolved"})
			return
		}

		c.JSON(200, gin.H{
			"message": "Test marked as resolved",
		})
	}
}

// IgnoreTest handles PUT /api/v1/flaky-tests/:testId/ignore
func (a *FlakyDetectionAdapter) IgnoreTest() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseFlakyTestRowID(c.Param("testId"))
		if err != nil {
			c.JSON(400, gin.H{"error": "test ID must be a positive integer"})
			return
		}

		if err := a.service.IgnoreTest(c.Request.Context(), id); err != nil {
			if errors.Is(err, domain.ErrFlakyTestNotFound) {
				c.JSON(404, gin.H{"error": "Flaky test not found"})
				return
			}
			a.logger.WithError(err).Error("Failed to ignore test")
			c.JSON(500, gin.H{"error": "Failed to ignore test"})
			return
		}

		c.JSON(200, gin.H{
			"message": "Test marked as ignored",
		})
	}
}

// GetFlakyTestTrends handles GET /api/v1/projects/:projectId/flaky-tests/trends
func (a *FlakyDetectionAdapter) GetFlakyTestTrends() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(400, gin.H{"error": "project ID is required"})
			return
		}

		// Get period from query param, default to 30 days
		periodStr := c.DefaultQuery("period", "30d")
		period, err := parsePeriod(periodStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid period format"})
			return
		}

		trends, err := a.service.GetFlakyTestTrends(c.Request.Context(), projectID, period)
		if err != nil {
			a.logger.WithError(err).Error("Failed to get flaky test trends")
			c.JSON(500, gin.H{"error": "Failed to get trends"})
			return
		}

		c.JSON(200, gin.H{
			"trends": trends,
			"period": periodStr,
		})
	}
}

// RegisterRoutes registers all flaky detection routes
func (a *FlakyDetectionAdapter) RegisterRoutes(router *gin.RouterGroup) {
	// Analysis endpoints
	router.POST("/projects/:projectId/test-runs/:testRunId/analyze", a.AnalyzeTestRun())

	// Flaky test endpoints
	router.GET("/projects/:projectId/flaky-tests", a.GetFlakyTests())
	router.GET("/projects/:projectId/flaky-tests/trends", a.GetFlakyTestTrends())
	router.PUT("/flaky-tests/:testId/resolve", a.MarkTestResolved())
	router.PUT("/flaky-tests/:testId/ignore", a.IgnoreTest())
}

// Helper function to parse period strings like "7d", "30d", "1h"
func parsePeriod(periodStr string) (time.Duration, error) {
	if len(periodStr) < 2 {
		return 0, fmt.Errorf("invalid period format")
	}

	unit := periodStr[len(periodStr)-1]
	valueStr := periodStr[:len(periodStr)-1]

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, err
	}

	switch unit {
	case 'h':
		return time.Duration(value) * time.Hour, nil
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported time unit: %c", unit)
	}
}

// GraphQL resolvers for flaky detection

// ResolveFlakyTests resolves flaky tests for GraphQL
func (a *FlakyDetectionAdapter) ResolveFlakyTests(ctx context.Context, projectID string) ([]*domain.FlakyTest, error) {
	return a.service.GetFlakyTests(ctx, projectID)
}

// ResolveTestFlakeScore calculates the flake score for a specific test
func (a *FlakyDetectionAdapter) ResolveTestFlakeScore(ctx context.Context, projectID string, testName string) (float64, error) {
	// This would look up the specific test's flake score
	// For now, return 0
	return 0.0, nil
}
