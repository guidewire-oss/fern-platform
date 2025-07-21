# Project Registration and Management Use Cases

## Overview

Project registration is the foundational step in using Fern Platform. A project serves as a container for all test data, analytics, and insights. Each project is identified by a unique project ID that client applications use to submit test results.

## Actors

- **Manager**: A user who belongs to both a manager group and a team group
- **Team Member**: A regular user who belongs to a team group
- **System**: The Fern Platform backend

## Prerequisites

- User must be authenticated (see [00-authentication.md](./00-authentication.md))
- User must have completed the login flow successfully
- User must belong to at least one team group
- For management operations: User must also belong to the manager group

## Use Cases

### UC-01: Create New Project

**As a** Manager  
**I want to** create a new project for my team  
**So that** our team can start collecting test data and analytics

#### Acceptance Criteria

```gherkin
Feature: Project Creation
  As a Manager
  I want to create projects for my team
  So that we can track test results

  Background:
    Given I have completed authentication as per UC-00-01
    And I am logged in as a user with manager role
    And I belong to a team "<team_name>"
    And I am on the Projects page

  Scenario: Successfully create a new project
    When I click on "Create New Project" button
    Then I should see the project creation form
    When I fill in the following details:
      | Field           | Value                          |
      | Project Name    | E-Commerce Frontend            |
      | Description     | Frontend tests for e-commerce  |
      | Repository URL  | https://github.com/org/repo    |
      | Default Branch  | main                           |
    And I click "Create Project"
    Then I should see a success message "Project created successfully"
    And I should see the generated project ID
    And the project should appear in my projects list
    And the project should be assigned to "<team_name>"

  Scenario: Project name validation
    When I click on "Create New Project" button
    And I fill in the project name with ""
    And I click "Create Project"
    Then I should see an error message "Project name is required"

  Scenario: Duplicate project name
    Given a project named "E-Commerce Frontend" already exists
    When I click on "Create New Project" button
    And I fill in the project name with "E-Commerce Frontend"
    And I click "Create Project"
    Then I should see an error message "A project with this name already exists"

  Scenario: Non-manager cannot create project
    Given I am logged in as a regular team member
    When I navigate to the Projects page
    Then I should not see the "Create New Project" button
```

### UC-02: View Project Details

**As a** Team Member or Manager  
**I want to** view project details including the project ID  
**So that** I can configure my test clients to send data to this project

#### Acceptance Criteria

```gherkin
Feature: View Project Details
  As a Team Member or Manager
  I want to view project details
  So that I can get the configuration needed for test clients

  Background:
    Given a project "E-Commerce Frontend" exists with ID "<project_id>"
    And the project belongs to "<team_name>"

  Scenario: Manager views project details
    Given I am logged in as a manager of "<team_name>"
    When I navigate to the Projects page
    And I click on "E-Commerce Frontend" project
    Then I should see the project details page
    And I should see the following information:
      | Field           | Value                                      |
      | Project Name    | E-Commerce Frontend                        |
      | Project ID      | <project_id>                               |
      | Description     | Frontend tests for e-commerce              |
      | Repository URL  | https://github.com/org/repo                |
      | Default Branch  | main                                       |
      | Team            | <team_name>                                |
      | Status          | Active                                     |
    And I should see "Edit" and "Delete" buttons
    And I should see a "Copy" button next to the Project ID

  Scenario: Team member views project details
    Given I am logged in as a regular member of "<team_name>"
    When I navigate to the Projects page
    And I click on "E-Commerce Frontend" project
    Then I should see the project details page
    And I should see the Project ID "<project_id>"
    But I should not see "Edit" or "Delete" buttons
    And I should see a "Copy" button next to the Project ID

  Scenario: Copy project ID to clipboard
    Given I am viewing the project details page
    When I click the "Copy" button next to the Project ID
    Then the Project ID should be copied to my clipboard
    And I should see a tooltip "Copied to clipboard"

  Scenario: User from different team cannot view project
    Given I am logged in as a member of a different team "<other_team_name>"
    When I navigate to the Projects page
    Then I should not see "E-Commerce Frontend" in the projects list
```

### UC-03: Update Project Details

**As a** Manager  
**I want to** update project details  
**So that** I can keep project information current

#### Acceptance Criteria

```gherkin
Feature: Update Project
  As a Manager
  I want to update project details
  So that project information stays current

  Background:
    Given I am logged in as a manager of "<team_name>"
    And a project "E-Commerce Frontend" exists for "<team_name>"
    And I am viewing the project details page

  Scenario: Successfully update project details
    When I click the "Edit" button
    Then I should see the project edit form with current values
    When I update the following fields:
      | Field          | New Value                              |
      | Description    | Updated frontend tests for e-commerce  |
      | Default Branch | develop                                |
    And I click "Save Changes"
    Then I should see a success message "Project updated successfully"
    And the project details should show the updated values

  Scenario: Cancel project update
    When I click the "Edit" button
    And I change the description to "New description"
    And I click "Cancel"
    Then I should see the project details page
    And the description should remain unchanged

  Scenario: Project ID cannot be changed
    When I click the "Edit" button
    Then the Project ID field should be read-only
    And I should not be able to modify the Project ID
```

### UC-04: Delete Project

**As a** Manager  
**I want to** delete projects that are no longer needed  
**So that** we can maintain a clean project list

#### Acceptance Criteria

```gherkin
Feature: Delete Project
  As a Manager
  I want to delete unused projects
  So that we maintain a clean system

  Background:
    Given I am logged in as a manager of "<team_name>"
    And a project "Old Project" exists for "<team_name>"
    And the project has test data from the last 30 days

  Scenario: Delete project with confirmation
    Given I am viewing the "Old Project" details page
    When I click the "Delete" button
    Then I should see a confirmation dialog with the message:
      """
      Are you sure you want to delete "Old Project"?
      This will permanently delete all associated test data.
      This action cannot be undone.
      """
    And I should see "Cancel" and "Delete" buttons

  Scenario: Confirm project deletion
    Given I see the delete confirmation dialog
    When I click "Delete"
    Then I should see a success message "Project deleted successfully"
    And I should be redirected to the Projects list
    And "Old Project" should not appear in the list

  Scenario: Cancel project deletion
    Given I see the delete confirmation dialog
    When I click "Cancel"
    Then the dialog should close
    And I should remain on the project details page
    And the project should not be deleted

  Scenario: Cannot delete project with recent activity
    Given the project has test runs from the last 24 hours
    When I click the "Delete" button
    Then I should see a warning message:
      """
      This project has recent test activity. 
      Are you sure you want to delete it?
      """
```

### UC-05: Deactivate/Activate Project

**As a** Manager  
**I want to** temporarily deactivate projects  
**So that** they stop accepting test data without losing historical data

#### Acceptance Criteria

```gherkin
Feature: Project Status Management
  As a Manager
  I want to activate/deactivate projects
  So that I can control which projects accept test data

  Background:
    Given I am logged in as a manager of "<team_name>"
    And an active project "E-Commerce Frontend" exists

  Scenario: Deactivate an active project
    Given I am viewing the project details page
    And the project status shows "Active"
    When I click the "Deactivate Project" button
    Then I should see a confirmation dialog
    When I confirm the action
    Then the project status should change to "Inactive"
    And I should see a warning banner "This project is inactive and will not accept new test data"

  Scenario: Activate an inactive project
    Given the project is currently "Inactive"
    When I click the "Activate Project" button
    Then the project status should change to "Active"
    And the warning banner should disappear

  Scenario: Test client attempts to send data to inactive project
    Given the project is "Inactive"
    When a test client sends results with this project ID
    Then the API should return a 403 error
    And the error message should be "Project is inactive"
```

## Test Data Variables

The following variables are used throughout the test scenarios and should be provided during test execution:

| Variable | Description | Example |
|----------|-------------|---------|
| `<team_name>` | The name of the team group in Keycloak | `frontend-team`, `backend-team`, `mobile-team` |
| `<other_team_name>` | A different team for negative test cases | `qa-team`, `devops-team` |
| `<project_id>` | The generated UUID for a project | `550e8400-e29b-41d4-a716-446655440001` |

### Example Test Execution

```bash
# Running with Ginkgo
ginkgo -v \
  -team-name="frontend-team" \
  -other-team-name="backend-team" \
  -base-url="http://fern-platform.local:8080"
```

## Integration Points

### Client Configuration Example

Once a project is created, development teams need to configure their test clients:

```javascript
// Jest Configuration
module.exports = {
  reporters: [
    'default',
    ['@guidewire/fern-jest-client', {
      url: process.env.FERN_URL || 'https://fern-platform.company.com',
      projectId: '<project_id>' // Get this from your manager
    }]
  ]
}
```

```java
// JUnit Configuration
@RunWith(FernJUnitRunner.class)
@FernConfig(
    url = "${FERN_URL}",
    projectId = "<project_id>" // Get this from your manager
)
public class MyTestSuite {
    // Your tests
}
```

```go
// Ginkgo Configuration
import "github.com/guidewire-oss/fern-ginkgo-client/reporter"

func TestSuite(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecsWithDefaultAndCustomReporters(t, "My Suite",
        []Reporter{reporter.NewFernReporter()})
}
```

```yaml
# Environment Configuration (all frameworks)
export FERN_PROJECT_ID='<project_id>'  # Get this from your manager
export FERN_URL='https://fern-platform.company.com'
```

## Security Considerations

1. **Project ID Generation**: Must be cryptographically secure and globally unique
2. **Team Isolation**: Users can only see/manage projects for teams they belong to
3. **Role Enforcement**: Only managers can perform write operations
4. **Audit Trail**: All project operations should be logged with user, timestamp, and action
5. **API Security**: Project ID alone should not be sufficient for data submission in production

## Error Scenarios

1. **Network Failures**: Show appropriate error messages and retry options
2. **Permission Denied**: Clear messaging when users lack required permissions
3. **Validation Errors**: Inline field validation with helpful error messages
4. **Concurrent Updates**: Handle race conditions when multiple managers edit simultaneously

## Future Enhancements

1. **Project Templates**: Pre-configured project settings for common scenarios
2. **Bulk Operations**: Select and perform actions on multiple projects
3. **Project Archiving**: Soft delete with ability to restore
4. **Project Cloning**: Duplicate project settings for similar projects
5. **Team Transfer**: Move project ownership between teams
6. **API Key Management**: Per-project API keys for enhanced security