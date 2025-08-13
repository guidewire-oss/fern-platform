package graphql

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
)

// convertTestRunToGraphQL converts a domain test run to GraphQL model
func (r *Resolver) convertTestRunToGraphQL(testRun *testingDomain.TestRun) *model.TestRun {
	// Convert suite runs
	suiteRuns := make([]*model.SuiteRun, len(testRun.SuiteRuns))
	for i, suite := range testRun.SuiteRuns {
		suiteRuns[i] = r.convertSuiteRunToGraphQL(&suite)
	}

	// Log for debugging with more detail
	suiteDetails := make([]map[string]interface{}, len(testRun.SuiteRuns))
	for i, s := range testRun.SuiteRuns {
		suiteDetails[i] = map[string]interface{}{
			"name":         s.Name,
			"id":           s.ID,
			"total_specs":  s.TotalTests,
			"passed_specs": s.PassedTests,
			"failed_specs": s.FailedTests,
			"spec_count":   len(s.SpecRuns),
		}
	}

	r.logger.WithFields(map[string]interface{}{
		"test_run_id":   testRun.ID,
		"run_id":        testRun.RunID,
		"suite_count":   len(testRun.SuiteRuns),
		"suite_details": suiteDetails,
	}).Info("Converting test run to GraphQL")

	return &model.TestRun{
		ID:           strconv.FormatUint(uint64(testRun.ID), 10),
		RunID:        testRun.RunID,
		ProjectID:    testRun.ProjectID,
		Branch:       convertStringPtr(testRun.Branch),
		CommitSha:    convertStringPtr(testRun.GitCommit),
		Status:       testRun.Status,
		StartTime:    testRun.StartTime,
		EndTime:      testRun.EndTime,
		TotalTests:   testRun.TotalTests,
		PassedTests:  testRun.PassedTests,
		FailedTests:  testRun.FailedTests,
		SkippedTests: testRun.SkippedTests,
		Duration:     int(testRun.Duration.Milliseconds()),
		Environment:  convertStringPtr(testRun.Environment),
		SuiteRuns:    suiteRuns,
		CreatedAt:    testRun.StartTime, // Use StartTime as CreatedAt
		UpdatedAt:    testRun.StartTime, // Use StartTime as UpdatedAt
	}
}

// convertSuiteRunToGraphQL converts a domain suite run to GraphQL model
func (r *Resolver) convertSuiteRunToGraphQL(suite *testingDomain.SuiteRun) *model.SuiteRun {
	// Convert spec runs
	specRuns := make([]*model.SpecRun, len(suite.SpecRuns))
	for i, spec := range suite.SpecRuns {
		specRuns[i] = r.convertSpecRunToGraphQL(spec)
	}

	return &model.SuiteRun{
		ID:           strconv.FormatUint(uint64(suite.ID), 10),
		TestRunID:    strconv.FormatUint(uint64(suite.TestRunID), 10),
		SuiteName:    suite.Name,
		Status:       suite.Status,
		StartTime:    suite.StartTime,
		EndTime:      suite.EndTime,
		TotalSpecs:   suite.TotalTests,
		PassedSpecs:  suite.PassedTests,
		FailedSpecs:  suite.FailedTests,
		SkippedSpecs: suite.SkippedTests,
		Duration:     int(suite.Duration.Milliseconds()),
		SpecRuns:     specRuns,
		CreatedAt:    suite.StartTime,
		UpdatedAt:    suite.StartTime,
	}
}

// convertSpecRunToGraphQL converts a domain spec run to GraphQL model
func (r *Resolver) convertSpecRunToGraphQL(spec *testingDomain.SpecRun) *model.SpecRun {
	var errorMessage *string
	if spec.ErrorMessage != "" {
		errorMessage = &spec.ErrorMessage
	}

	var stackTrace *string
	if spec.StackTrace != "" {
		stackTrace = &spec.StackTrace
	}

	return &model.SpecRun{
		ID:           strconv.FormatUint(uint64(spec.ID), 10),
		SuiteRunID:   strconv.FormatUint(uint64(spec.SuiteRunID), 10),
		SpecName:     spec.Name,
		Status:       spec.Status,
		StartTime:    spec.StartTime,
		EndTime:      spec.EndTime,
		Duration:     int(spec.Duration.Milliseconds()),
		ErrorMessage: errorMessage,
		StackTrace:   stackTrace,
		RetryCount:   spec.RetryCount,
		IsFlaky:      spec.IsFlaky,
		CreatedAt:    spec.StartTime,
		UpdatedAt:    spec.StartTime,
	}
}

// RecentTestRuns_domain retrieves recent test runs using domain service
func (r *queryResolver) RecentTestRuns_domain(ctx context.Context, projectID *string, limit *int) ([]*model.TestRun, error) {
	limitVal := 10
	if limit != nil {
		limitVal = *limit
	}

	var testRuns []*testingDomain.TestRun
	var err error

	if projectID != nil {
		// Get test runs for specific project
		testRuns, err = r.testingService.GetProjectTestRuns(ctx, *projectID, limitVal)
	} else {
		// Get recent test runs across all projects
		testRuns, err = r.testingService.GetRecentTestRuns(ctx, limitVal)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get recent test runs: %w", err)
	}

	// Convert to GraphQL models
	result := make([]*model.TestRun, len(testRuns))
	for i, tr := range testRuns {
		result[i] = r.convertTestRunToGraphQL(tr)
	}

	return result, nil
}

// convertProjectToGraphQL converts a domain project to GraphQL model
func (r *Resolver) convertProjectToGraphQL(project *projectsDomain.Project) *model.Project {
	snapshot := project.ToSnapshot()
	return &model.Project{
		ID:            strconv.FormatUint(uint64(snapshot.ID), 10),
		ProjectID:     string(snapshot.ProjectID),
		Name:          snapshot.Name,
		Description:   convertStringPtr(snapshot.Description),
		Repository:    convertStringPtr(snapshot.Repository),
		DefaultBranch: snapshot.DefaultBranch,
		IsActive:      snapshot.IsActive,
		Team:          convertStringPtr(string(snapshot.Team)),
		CreatedAt:     snapshot.CreatedAt,
		UpdatedAt:     snapshot.UpdatedAt,
	}
}

// convertTagToGraphQL converts a domain tag to GraphQL model
func (r *Resolver) convertTagToGraphQL(tag *tagsDomain.Tag) *model.Tag {
	// Convert TagID string to numeric ID for GraphQL
	// For now, use a simple hash
	id := "0"
	if tagID := tag.ID(); tagID != "" {
		// Simple conversion - in production you'd want a proper mapping
		id = fmt.Sprintf("%d", len(string(tagID)))
	}

	return &model.Tag{
		ID:          id,
		Name:        tag.Name(),
		Description: nil,        // Not available in domain model
		Color:       nil,        // Not available in domain model
		CreatedAt:   time.Now(), // TODO: Add timestamps to domain model
		UpdatedAt:   time.Now(), // TODO: Add timestamps to domain model
	}
}

// Helper function to convert duration to int pointer (milliseconds)
func convertDurationPtr(d time.Duration) *int {
	ms := int(d.Milliseconds())
	return &ms
}

// GetTestRun implementation using domain service
func (r *queryResolver) GetTestRun_domain(ctx context.Context, id string) (*model.TestRun, error) {
	// Try to parse as uint ID
	if numID, err := strconv.ParseUint(id, 10, 32); err == nil {
		testRun, err := r.testingService.GetTestRun(ctx, uint(numID))
		if err != nil {
			return nil, err
		}
		return r.convertTestRunToGraphQL(testRun), nil
	}

	// Otherwise return error - GetTestRunByRunID not implemented
	return nil, fmt.Errorf("test run not found")
}

// GetProject implementation using domain service
func (r *queryResolver) GetProject_domain(ctx context.Context, projectID string) (*model.Project, error) {
	project, err := r.projectService.GetProject(ctx, projectsDomain.ProjectID(projectID))
	if err != nil {
		return nil, err
	}
	return r.convertProjectToGraphQL(project), nil
}

// ListProjects implementation using domain service
func (r *queryResolver) ListProjects_domain(ctx context.Context, limit *int, offset *int, team *string) ([]*model.Project, error) {
	// Default pagination
	pageLimit := 50
	pageOffset := 0
	if limit != nil {
		pageLimit = *limit
	}
	if offset != nil {
		pageOffset = *offset
	}

	// Get projects based on team filter
	var projects []*projectsDomain.Project
	var err error

	// For now just list all projects - team filtering not implemented
	projects, _, err = r.projectService.ListProjects(ctx, pageLimit, pageOffset)

	if err != nil {
		return nil, err
	}

	// Convert to GraphQL models
	result := make([]*model.Project, len(projects))
	for i, project := range projects {
		result[i] = r.convertProjectToGraphQL(project)
	}

	return result, nil
}

// ListTags implementation using domain service
func (r *queryResolver) ListTags_domain(ctx context.Context) ([]*model.Tag, error) {
	// GetAllTags not implemented - use ListTags
	tags, err := r.tagService.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to GraphQL models
	result := make([]*model.Tag, len(tags))
	for i, tag := range tags {
		result[i] = r.convertTagToGraphQL(tag)
	}

	return result, nil
}

// CreateProject implementation using domain service
func (r *mutationResolver) CreateProject_domain(ctx context.Context, input model.CreateProjectInput) (*model.Project, error) {
	// Get current user for team assignment
	user, err := getCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	// Check if user has permission to create projects
	// Only admins and managers can create projects
	roleGroups := getRoleGroupNamesFromContext(ctx)
	canCreate := user.Role == authDomain.RoleAdmin || hasManagerRole(user, roleGroups)

	if !canCreate {
		return nil, fmt.Errorf("insufficient permissions to create project")
	}

	// Generate project ID if not provided
	projectID := input.ProjectID
	if projectID == "" {
		projectID = uuid.New().String()
	}

	// Use team from input if provided, otherwise determine from user groups
	team := projectsDomain.Team("default") // Default team
	if input.Team != nil && *input.Team != "" {
		team = projectsDomain.Team(*input.Team)
	} else if len(user.Groups) > 0 {
		team = projectsDomain.Team(user.Groups[0].GroupName)
	}

	// Create project with creator user ID
	project, err := r.projectService.CreateProject(ctx, projectsDomain.ProjectID(projectID), input.Name, team, user.UserID)
	if err != nil {
		return nil, err
	}

	// Update optional fields
	if input.Description != nil {
		project.UpdateDescription(*input.Description)
	}
	if input.Repository != nil {
		project.UpdateRepository(*input.Repository)
	}
	if input.DefaultBranch != nil {
		project.UpdateDefaultBranch(*input.DefaultBranch)
	}

	// Prepare update request
	updateReq := projectsApp.UpdateProjectRequest{}
	if input.Description != nil {
		updateReq.Description = input.Description
	}
	if input.Repository != nil {
		updateReq.Repository = input.Repository
	}
	if input.DefaultBranch != nil {
		updateReq.DefaultBranch = input.DefaultBranch
	}

	// Save updates if any fields were set
	if updateReq.Description != nil || updateReq.Repository != nil || updateReq.DefaultBranch != nil {
		if err := r.projectService.UpdateProject(ctx, projectsDomain.ProjectID(projectID), updateReq); err != nil {
			return nil, err
		}

		// Fetch updated project
		project, err = r.projectService.GetProject(ctx, projectsDomain.ProjectID(projectID))
		if err != nil {
			return nil, err
		}
	}

	return r.convertProjectToGraphQL(project), nil
}

// CreateTag implementation using domain service
func (r *mutationResolver) CreateTag_domain(ctx context.Context, input model.CreateTagInput) (*model.Tag, error) {
	// CreateTag only takes a name parameter
	createdTag, err := r.tagService.CreateTag(ctx, input.Name)
	if err != nil {
		return nil, err
	}

	return r.convertTagToGraphQL(createdTag), nil
}

// Helper to get string value from pointer
func getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

// UpdateProject implementation using domain service
func (r *mutationResolver) UpdateProject_domain(ctx context.Context, id string, input model.UpdateProjectInput) (*model.Project, error) {
	r.logger.WithFields(map[string]interface{}{
		"id":    id,
		"input": input,
	}).Info("UpdateProject mutation called")

	// Check user permissions
	user, err := getCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Try to parse as database ID first
	var projectID projectsDomain.ProjectID
	if idUint, err := strconv.ParseUint(id, 10, 32); err == nil {
		// It's a numeric ID, need to find the project
		projects, _, err := r.projectService.ListProjects(ctx, 1000, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		var found bool
		for _, p := range projects {
			if p.ID() == uint(idUint) {
				projectID = p.ProjectID()
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("project not found")
		}
	} else {
		// Assume it's a project UUID
		projectID = projectsDomain.ProjectID(id)
	}

	// Get the existing project
	existingProject, err := r.projectService.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	snapshot := existingProject.ToSnapshot()

	// Check if user can update this project
	canUpdate := false
	if user.Role == authDomain.RoleAdmin {
		canUpdate = true
	} else {
		// Get role group names from context
		roleGroups := getRoleGroupNamesFromContext(ctx)

		// Check if user has team + manager group combination
		if snapshot.Team != "" {
			hasTeamGroup := false
			hasManagerGroup := false

			for _, group := range user.Groups {
				groupName := strings.TrimPrefix(group.GroupName, "/")
				if groupName == string(snapshot.Team) {
					hasTeamGroup = true
				}
				if groupName == roleGroups.ManagerGroup {
					hasManagerGroup = true
				}
			}

			// If user is in both team and manager groups, they can update
			if hasTeamGroup && hasManagerGroup {
				canUpdate = true
			}
		}

		// If not via groups, check scopes
		if !canUpdate {
			// Check scopes for update permission on this project
			requiredScopes := []string{
				fmt.Sprintf("project:write:%s", snapshot.ProjectID),
				fmt.Sprintf("project:*:%s", snapshot.ProjectID),
			}

			// If project has a team, also check team-based scopes
			if snapshot.Team != "" {
				requiredScopes = append(requiredScopes,
					fmt.Sprintf("project:write:%s:*", snapshot.Team),
					fmt.Sprintf("project:*:%s:*", snapshot.Team),
				)
			}

			scopes := getUserScopesFromContext(ctx)
			for _, scope := range scopes {
				for _, required := range requiredScopes {
					if matchScope(scope, required) {
						canUpdate = true
						break
					}
				}
				if canUpdate {
					break
				}
			}
		}
	}

	if !canUpdate {
		r.logger.WithFields(map[string]interface{}{
			"user_id":      user.UserID,
			"project_id":   projectID,
			"user_role":    user.Role,
			"user_groups":  user.Groups,
			"project_team": snapshot.Team,
		}).Warn("User lacks permission to update project")
		return nil, fmt.Errorf("insufficient permissions to update project")
	}

	// Prepare update request
	updateReq := projectsApp.UpdateProjectRequest{}
	if input.Name != nil {
		updateReq.Name = input.Name
	}
	if input.Description != nil {
		updateReq.Description = input.Description
	}
	if input.Repository != nil {
		updateReq.Repository = input.Repository
	}
	if input.DefaultBranch != nil {
		updateReq.DefaultBranch = input.DefaultBranch
	}
	if input.Team != nil {
		team := projectsDomain.Team(*input.Team)
		updateReq.Team = &team
	}

	// Update the project
	r.logger.WithFields(map[string]interface{}{
		"project_id":     projectID,
		"update_request": updateReq,
	}).Info("Updating project")

	if err := r.projectService.UpdateProject(ctx, projectID, updateReq); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	// Fetch updated project
	updatedProject, err := r.projectService.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	r.logger.WithFields(map[string]interface{}{
		"project_id": projectID,
		"updated_by": user.UserID,
	}).Info("Project updated successfully")

	return r.convertProjectToGraphQL(updatedProject), nil
}

// DeleteProject implementation using domain service
func (r *mutationResolver) DeleteProject_domain(ctx context.Context, id string) (bool, error) {
	r.logger.WithFields(map[string]interface{}{
		"id": id,
	}).Info("DeleteProject mutation called")

	// Check user permissions
	user, err := getCurrentUser(ctx)
	if err != nil {
		return false, err
	}

	// Try to parse as database ID first
	var projectID projectsDomain.ProjectID
	if idUint, err := strconv.ParseUint(id, 10, 32); err == nil {
		// It's a numeric ID, need to find the project
		projects, _, err := r.projectService.ListProjects(ctx, 1000, 0)
		if err != nil {
			return false, fmt.Errorf("failed to list projects: %w", err)
		}

		var found bool
		for _, p := range projects {
			if p.ID() == uint(idUint) {
				projectID = p.ProjectID()
				found = true
				break
			}
		}

		if !found {
			return false, fmt.Errorf("project not found")
		}
	} else {
		// Assume it's a project UUID
		projectID = projectsDomain.ProjectID(id)
	}

	// Get the existing project
	existingProject, err := r.projectService.GetProject(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("project not found: %w", err)
	}

	snapshot := existingProject.ToSnapshot()

	// Check if user can delete this project
	canDelete := false
	if user.Role == authDomain.RoleAdmin {
		canDelete = true
	} else {
		// Get role group names from context
		roleGroups := getRoleGroupNamesFromContext(ctx)

		// Check if user has team + manager group combination
		if snapshot.Team != "" {
			hasTeamGroup := false
			hasManagerGroup := false

			for _, group := range user.Groups {
				groupName := strings.TrimPrefix(group.GroupName, "/")
				if groupName == string(snapshot.Team) {
					hasTeamGroup = true
				}
				if groupName == roleGroups.ManagerGroup {
					hasManagerGroup = true
				}
			}

			// If user is in both team and manager groups, they can delete
			if hasTeamGroup && hasManagerGroup {
				canDelete = true
			}
		}

		// If not via groups, check scopes
		if !canDelete {
			// Check scopes for delete permission on this project
			requiredScopes := []string{
				fmt.Sprintf("project:delete:%s", snapshot.ProjectID),
				fmt.Sprintf("project:*:%s", snapshot.ProjectID),
			}

			// If project has a team, also check team-based scopes
			if snapshot.Team != "" {
				requiredScopes = append(requiredScopes,
					fmt.Sprintf("project:delete:%s:*", snapshot.Team),
					fmt.Sprintf("project:*:%s:*", snapshot.Team),
				)
			}

			scopes := getUserScopesFromContext(ctx)
			for _, scope := range scopes {
				for _, required := range requiredScopes {
					if matchScope(scope, required) {
						canDelete = true
						break
					}
				}
				if canDelete {
					break
				}
			}
		}
	}

	if !canDelete {
		return false, fmt.Errorf("insufficient permissions to delete project")
	}

	// Delete project
	err = r.projectService.DeleteProject(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("failed to delete project: %w", err)
	}

	r.logger.WithFields(map[string]interface{}{
		"project_id": projectID,
		"deleted_by": user.UserID,
	}).Info("Project deleted")

	return true, nil
}

// Project implementation using domain service
func (r *queryResolver) Project_domain(ctx context.Context, id string) (*model.Project, error) {
	// Parse ID and find project
	projects, _, err := r.projectService.ListProjects(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	for _, p := range projects {
		if p.ID() == uint(idUint) {
			return r.convertProjectToGraphQL(p), nil
		}
	}

	return nil, fmt.Errorf("project not found")
}

// ProjectByProjectID implementation using domain service
func (r *queryResolver) ProjectByProjectID_domain(ctx context.Context, projectID string) (*model.Project, error) {
	project, err := r.projectService.GetProject(ctx, projectsDomain.ProjectID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return r.convertProjectToGraphQL(project), nil
}

// Projects implementation using domain service with pagination
func (r *queryResolver) Projects_domain(ctx context.Context, filter *model.ProjectFilter, first *int, after *string) (*model.ProjectConnection, error) {
	// Get current user
	user, err := getCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Apply pagination
	pageSize := 20
	if first != nil && *first > 0 && *first <= 100 {
		pageSize = *first
	}

	offset := 0
	if after != nil && *after != "" {
		// Simple cursor: just the index
		if idx, err := strconv.Atoi(*after); err == nil && idx >= 0 {
			offset = idx + 1
		}
	}

	// Get projects with pagination
	projects, totalCount, err := r.projectService.ListProjects(ctx, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	// Filter projects based on user's team access
	var filteredProjects []*projectsDomain.Project
	if user.Role == authDomain.RoleAdmin {
		// Admins can see all projects
		filteredProjects = projects
	} else {
		// Get user's teams
		userTeams := getUserTeamsFromContext(ctx)
		teamMap := make(map[string]bool)
		for _, team := range userTeams {
			teamMap[team] = true
		}

		// Filter projects by team
		for _, project := range projects {
			snapshot := project.ToSnapshot()
			if snapshot.Team != "" && teamMap[string(snapshot.Team)] {
				filteredProjects = append(filteredProjects, project)
			}
		}

		// Update total count to reflect filtered results
		totalCount = int64(len(filteredProjects))
	}

	// Apply search filter if provided
	if filter != nil && filter.Search != nil && *filter.Search != "" {
		searchLower := strings.ToLower(*filter.Search)
		searchFiltered := make([]*projectsDomain.Project, 0)
		for _, project := range filteredProjects {
			snapshot := project.ToSnapshot()
			if strings.Contains(strings.ToLower(snapshot.Name), searchLower) ||
				strings.Contains(strings.ToLower(string(snapshot.ProjectID)), searchLower) {
				searchFiltered = append(searchFiltered, project)
			}
		}
		filteredProjects = searchFiltered
		totalCount = int64(len(filteredProjects))
	}

	// Apply active filter if provided
	if filter != nil && filter.ActiveOnly != nil && *filter.ActiveOnly {
		activeFiltered := make([]*projectsDomain.Project, 0)
		for _, project := range filteredProjects {
			if project.ToSnapshot().IsActive {
				activeFiltered = append(activeFiltered, project)
			}
		}
		filteredProjects = activeFiltered
		totalCount = int64(len(filteredProjects))
	}

	hasMore := offset+len(filteredProjects) < int(totalCount)

	// Build edges
	edges := make([]*model.ProjectEdge, len(filteredProjects))
	for i, project := range filteredProjects {
		edges[i] = &model.ProjectEdge{
			Node:   r.convertProjectToGraphQL(project),
			Cursor: fmt.Sprintf("%d", offset+i), // Simple cursor
		}
	}

	// Build page info
	pageInfo := &model.PageInfo{
		HasNextPage:     hasMore,
		HasPreviousPage: offset > 0,
	}
	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &model.ProjectConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: int(totalCount),
	}, nil
}

// DashboardSummary implementation using domain service
func (r *queryResolver) DashboardSummary_domain(ctx context.Context) (*model.DashboardSummary, error) {
	// Get all projects to count them
	projects, totalProjects, err := r.projectService.ListProjects(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get project count: %w", err)
	}

	// Count active projects
	activeProjects := int64(0)
	for _, project := range projects {
		if project.ToSnapshot().IsActive {
			activeProjects++
		}
	}

	// Get test run statistics
	recentRuns, err := r.testingService.GetRecentTestRuns(ctx, 100)
	if err != nil {
		// Log error but don't fail the whole query
		r.logger.WithError(err).Error("Failed to get recent test runs for dashboard")
	}

	totalTestRuns := len(recentRuns)
	recentTestRuns := len(recentRuns)
	overallPassRate := float64(0)
	totalTestsExecuted := 0
	avgDuration := 0

	if len(recentRuns) > 0 {
		var totalTests, passedTests int
		var totalDuration int64

		for _, tr := range recentRuns {
			totalTests += tr.TotalTests
			passedTests += tr.PassedTests
			totalTestsExecuted += tr.TotalTests
			totalDuration += tr.Duration.Milliseconds()
		}

		if totalTests > 0 {
			overallPassRate = float64(passedTests) / float64(totalTests) * 100
		}

		if len(recentRuns) > 0 {
			avgDuration = int(totalDuration / int64(len(recentRuns)))
		}
	}

	version := "1.0.0"
	return &model.DashboardSummary{
		Health: &model.HealthStatus{
			Status:    "healthy",
			Service:   "fern-platform",
			Timestamp: time.Now(),
			Version:   &version,
		},
		ProjectCount:        int(totalProjects),
		ActiveProjectCount:  int(activeProjects),
		TotalTestRuns:       totalTestRuns,
		RecentTestRuns:      recentTestRuns,
		OverallPassRate:     overallPassRate,
		TotalTestsExecuted:  totalTestsExecuted,
		AverageTestDuration: avgDuration,
	}, nil
}

// TreemapData implementation using domain service
func (r *queryResolver) TreemapData_domain(ctx context.Context, projectID *string, days *int) (*model.TreemapData, error) {
	// Default to 7 days if not specified
	daysToQuery := 7
	if days != nil && *days > 0 {
		daysToQuery = *days
	}

	// Calculate date range
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -daysToQuery)

	// Get all projects that the user has access to
	projects, _, err := r.projectService.ListProjects(ctx, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %w", err)
	}

	// For each project, get test runs
	var allTestRuns []*testingDomain.TestRun
	for _, project := range projects {
		// Skip if specific project requested and doesn't match
		if projectID != nil && *projectID != "" && string(project.ProjectID()) != *projectID {
			continue
		}

		// Get test runs for this project
		testRuns, _, err := r.testingService.ListTestRuns(ctx, string(project.ProjectID()), 1000, 0)
		if err != nil {
			r.logger.WithError(err).Errorf("Failed to get test runs for project %s", project.ProjectID())
			continue
		}

		// Filter by date range
		for _, run := range testRuns {
			if run.StartTime.After(startTime) && run.StartTime.Before(endTime) {
				allTestRuns = append(allTestRuns, run)
			}
		}
	}

	// Create a map for quick project lookup
	projectMap := make(map[string]*projectsDomain.Project)
	for _, p := range projects {
		projectMap[string(p.ProjectID())] = p
	}

	// Group test runs by project
	projectRuns := make(map[string][]*testingDomain.TestRun)
	for _, run := range allTestRuns {
		// Only include runs for projects user has access to
		if _, ok := projectMap[run.ProjectID]; ok {
			projectRuns[run.ProjectID] = append(projectRuns[run.ProjectID], run)
		}
	}

	// Build treemap data
	var projectNodes []*model.ProjectTreemapNode
	totalDuration := 0
	totalTests := 0
	totalPassed := 0

	for projectID, runs := range projectRuns {
		project, ok := projectMap[projectID]
		if !ok {
			continue
		}

		// Convert to GraphQL project model
		gqlProject := r.convertProjectToGraphQL(project)

		// Aggregate suite data across all runs for this project
		suiteMap := make(map[string]*model.SuiteTreemapNode)
		projectDuration := 0
		projectTests := 0
		projectPassed := 0

		for _, run := range runs {
			// Get test run with details including suite runs
			testRunWithDetails, err := r.testingService.GetTestRunWithDetails(ctx, run.ID)
			if err != nil {
				r.logger.WithError(err).Errorf("Failed to get test run details for run %d", run.ID)
				continue
			}

			for _, suite := range testRunWithDetails.SuiteRuns {
				key := suite.Name

				if node, exists := suiteMap[key]; exists {
					// Update existing suite node
					node.TotalDuration += int(suite.Duration.Milliseconds())
					node.TotalSpecs += suite.TotalTests
					node.PassedSpecs += suite.PassedTests
					node.FailedSpecs += suite.FailedTests
				} else {
					// Create new suite node
					gqlSuite := &model.SuiteRun{
						ID:           strconv.FormatUint(uint64(suite.ID), 10),
						TestRunID:    strconv.FormatUint(uint64(suite.TestRunID), 10),
						SuiteName:    suite.Name,
						Status:       suite.Status,
						StartTime:    suite.StartTime,
						EndTime:      suite.EndTime,
						TotalSpecs:   suite.TotalTests,
						PassedSpecs:  suite.PassedTests,
						FailedSpecs:  suite.FailedTests,
						SkippedSpecs: suite.SkippedTests,
						Duration:     int(suite.Duration.Milliseconds()),
					}

					suiteMap[key] = &model.SuiteTreemapNode{
						Suite:         gqlSuite,
						Specs:         []*model.SpecTreemapNode{}, // Not including spec level for performance
						TotalDuration: int(suite.Duration.Milliseconds()),
						TotalSpecs:    suite.TotalTests,
						PassedSpecs:   suite.PassedTests,
						FailedSpecs:   suite.FailedTests,
						PassRate:      0,
					}
				}
			}

			projectDuration += int(run.Duration.Milliseconds())
			projectTests += run.TotalTests
			projectPassed += run.PassedTests
		}

		// Convert suite map to slice and calculate pass rates
		var suiteNodes []*model.SuiteTreemapNode
		for _, node := range suiteMap {
			if node.TotalSpecs > 0 {
				node.PassRate = float64(node.PassedSpecs) / float64(node.TotalSpecs)
			}
			suiteNodes = append(suiteNodes, node)
		}

		// Calculate project pass rate
		projectPassRate := float64(0)
		if projectTests > 0 {
			projectPassRate = float64(projectPassed) / float64(projectTests)
		}

		projectNode := &model.ProjectTreemapNode{
			Project:       gqlProject,
			Suites:        suiteNodes,
			TotalDuration: projectDuration,
			TotalTests:    projectTests,
			PassedTests:   projectPassed,
			FailedTests:   projectTests - projectPassed,
			PassRate:      projectPassRate,
			TotalRuns:     len(runs), // Add the count of test runs for this project within the time range
		}

		projectNodes = append(projectNodes, projectNode)
		totalDuration += projectDuration
		totalTests += projectTests
		totalPassed += projectPassed
	}

	// Calculate overall pass rate
	overallPassRate := float64(0)
	if totalTests > 0 {
		overallPassRate = float64(totalPassed) / float64(totalTests)
	}

	return &model.TreemapData{
		Projects:        projectNodes,
		TotalDuration:   totalDuration,
		TotalTests:      totalTests,
		OverallPassRate: overallPassRate,
	}, nil
}

// TestRuns implementation using domain service with pagination
func (r *queryResolver) TestRuns_domain(ctx context.Context, filter *model.TestRunFilter, first *int, after *string, orderBy *string, orderDirection *model.OrderDirection) (*model.TestRunConnection, error) {
	// Apply pagination
	pageSize := 20
	if first != nil && *first > 0 && *first <= 100 {
		pageSize = *first
	}

	offset := 0
	if after != nil && *after != "" {
		// Simple cursor: just the index
		if idx, err := strconv.Atoi(*after); err == nil && idx >= 0 {
			offset = idx + 1
		}
	}

	// Get project ID from filter if provided
	projectID := ""
	if filter != nil && filter.ProjectID != nil {
		projectID = *filter.ProjectID
	}

	// Get test runs with pagination
	testRuns, totalCount, err := r.testingService.ListTestRuns(ctx, projectID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list test runs: %w", err)
	}

	hasMore := offset+len(testRuns) < int(totalCount)

	// Build edges
	edges := make([]*model.TestRunEdge, len(testRuns))
	for i, run := range testRuns {
		edges[i] = &model.TestRunEdge{
			Node:   r.convertTestRunToGraphQL(run),
			Cursor: fmt.Sprintf("%d", offset+i), // Simple cursor
		}
	}

	// Build page info
	pageInfo := &model.PageInfo{
		HasNextPage:     hasMore,
		HasPreviousPage: offset > 0,
	}
	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &model.TestRunConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: int(totalCount),
	}, nil
}

// Tags implementation using domain service with pagination
func (r *queryResolver) Tags_domain(ctx context.Context, filter *model.TagFilter, first *int, after *string) (*model.TagConnection, error) {
	// Get all tags - domain doesn't support pagination yet
	tags, err := r.tagService.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	// Apply filter if provided
	filteredTags := tags
	if filter != nil && filter.Search != nil && *filter.Search != "" {
		filtered := make([]*tagsDomain.Tag, 0)
		searchLower := strings.ToLower(*filter.Search)
		for _, tag := range tags {
			if strings.Contains(strings.ToLower(tag.Name()), searchLower) {
				filtered = append(filtered, tag)
			}
		}
		filteredTags = filtered
	}

	// Apply pagination
	pageSize := 20
	if first != nil && *first > 0 && *first <= 100 {
		pageSize = *first
	}

	offset := 0
	if after != nil && *after != "" {
		// Simple cursor: just the index
		if idx, err := strconv.Atoi(*after); err == nil && idx >= 0 {
			offset = idx + 1
		}
	}

	// Slice tags for pagination
	start := offset
	end := offset + pageSize
	if start > len(filteredTags) {
		start = len(filteredTags)
	}
	if end > len(filteredTags) {
		end = len(filteredTags)
	}

	paginatedTags := filteredTags[start:end]
	hasMore := end < len(filteredTags)

	// Build edges
	edges := make([]*model.TagEdge, len(paginatedTags))
	for i, tag := range paginatedTags {
		edges[i] = &model.TagEdge{
			Node:   r.convertTagToGraphQL(tag),
			Cursor: fmt.Sprintf("%d", start+i),
		}
	}

	// Build page info
	pageInfo := &model.PageInfo{
		HasNextPage:     hasMore,
		HasPreviousPage: offset > 0,
	}
	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &model.TagConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: len(filteredTags),
	}, nil
}
