# PM Connectors Management Use Cases

## Overview

PM (Project Management) Connectors enable seamless integration between Fern Platform and external project management tools like JIRA, GitHub Issues, Azure DevOps, and Aha!. These connectors allow teams to link test results with requirements, user stories, and bug reports, providing end-to-end traceability from requirements to test execution.

## Actors

- **Admin**: A user with administrative privileges who can create and manage PM connectors
- **Manager**: A user who can configure PM links for their team's projects
- **Team Member**: A user who can view PM links and use labels to associate tests with PM items
- **System**: The Fern Platform backend and PM connector services

## Prerequisites

- User must be authenticated (see [00-authentication.md](./00-authentication.md))
- For connector management: User must have admin role
- For project PM links: User must have manager role for the project
- External PM tool must be accessible from Fern Platform
- Valid credentials for the external PM tool

## Use Cases

### UC-10: PM Connectors Management

#### UC-10-01: View PM Connectors List

**As an** Admin or Manager  
**I want to** view all configured PM connectors  
**So that** I can manage integrations with external PM tools

##### Acceptance Criteria

```gherkin
Feature: PM Connectors List View
  As an Admin or Manager
  I want to view PM connectors in a organized way
  So that I can efficiently manage integrations

  Background:
    Given I am authenticated with admin or manager role
    And I am on the main dashboard

  Scenario: Navigate to PM Connectors
    When I click on "PM Connectors" in the navigation menu
    Then I should be taken to the PM Connectors page at "/pm-connectors"
    And I should see a table view with the following columns:
      | Column        | Description                        |
      | Name          | Connector display name             |
      | Type          | JIRA, GitHub, Azure DevOps, etc    |
      | Status        | Active, Inactive, Error            |
      | Last Sync     | Relative time (e.g., "2 hours ago")|
      | Projects      | Count of linked projects           |
      | Actions       | Quick action buttons               |

  Scenario: View connector health status
    Given I am on the PM Connectors page
    Then each connector should display a health indicator:
      | Status    | Icon | Color  | Description                    |
      | Active    | ●    | Green  | Connected (Live sync every 6h) |
      | Syncing   | ◐    | Blue   | Syncing... (2 of 150 items)   |
      | Inactive  | ○    | Gray   | Disconnected (Last seen 2d ago)|
      | Error     | ⚠️   | Red    | Error: Invalid credentials     |

  Scenario: Empty state
    Given no PM connectors exist
    When I view the PM Connectors page
    Then I should see an engaging empty state with:
      | Element       | Content                                |
      | Icon          | 🔌                                     |
      | Title         | Connect Your First PM Tool             |
      | Description   | Sync JIRA issues, GitHub PRs, or Azure DevOps work items |
      | Actions       | [Video Tutorial] [Documentation]       |
      | Primary CTA   | Create Connection                      |
```

#### UC-10-02: Create PM Connector

**As an** Admin  
**I want to** create a new PM connector  
**So that** teams can link their projects to external PM tools

##### Acceptance Criteria

```gherkin
Feature: PM Connector Creation
  As an Admin
  I want to create PM connectors using a streamlined interface
  So that setup is quick and error-free

  Background:
    Given I am authenticated with admin role
    And I am on the PM Connectors page

  Scenario: Initiate connector creation
    When I click on "Create Connection" button
    Then a slide-out panel should appear from the right
    And the main connector list should remain visible but dimmed
    And the panel should have a close button (X) in the top right

  Scenario: Configure basic connector details
    Given the create connector panel is open
    When I view the connector form
    Then I should see the following sections:
      | Section       | Fields                                 |
      | Basic Info    | Name, Type, Description                |
      | Connection    | Base URL, Authentication Method        |
      | Credentials   | (Dynamic based on auth method)         |
      | Test          | Test Connection button with preview    |

  Scenario: Select connector type with visual cards
    Given I am in the Basic Info section
    When I click on the Type field
    Then I should see connector type cards:
      | Type          | Icon | Description                    |
      | JIRA          | 🎯   | Atlassian JIRA Server/Cloud    |
      | GitHub        | 🐙   | GitHub Issues & Projects       |
      | Azure DevOps  | 📘   | Azure Boards & Work Items      |
      | Aha!          | 💡   | Aha! Roadmaps & Ideas          |

  Scenario: Configure JIRA connector with inline validation
    Given I have selected JIRA as the connector type
    When I fill in the connection details:
      | Field         | Value                                  | Validation           |
      | Name          | Production JIRA                        | Required, Unique     |
      | Base URL      | https://company.atlassian.net          | Valid URL format     |
      | Auth Method   | API Token                              | Dropdown selection   |
    Then each field should validate as I type
    And show success indicators (✓) for valid entries
    And show inline errors for invalid entries

  Scenario: Test connection with live feedback
    Given I have filled all required fields
    When I click "Test Connection"
    Then I should see a live test panel showing:
      | Stage         | Status                                 |
      | Connecting    | Establishing connection...             |
      | Authenticating| Verifying credentials...               |
      | Permissions   | Checking API permissions...            |
      | Sample Data   | Fetching sample project: "DEMO"        |
    And each stage should show ✓ or ✗ as it completes
    And if successful, show "Connection successful! Found 5 projects"
```

#### UC-10-03: Configure Field Mappings

**As an** Admin  
**I want to** visually map fields between PM tools and Fern  
**So that** data synchronization works correctly

##### Acceptance Criteria

```gherkin
Feature: Visual Field Mapping Interface
  As an Admin
  I want to map fields using a visual interface
  So that I can easily understand the data flow

  Scenario: Access field mapping interface
    Given I have created a PM connector
    When I click on "Configure Field Mappings"
    Then I should see a visual mapping interface with:
      | Left Panel    | PM Tool Fields (e.g., JIRA)           |
      | Center        | Connection lines and transformations   |
      | Right Panel   | Fern Platform Fields                   |

  Scenario: Auto-suggest field mappings
    Given I am in the field mapping interface
    When the interface loads
    Then the system should auto-suggest mappings based on:
      | JIRA Field    | Suggested Fern Field | Confidence |
      | Issue Key     | Requirement ID      | 100%       |
      | Summary       | Title               | 95%        |
      | Description   | Description         | 95%        |
      | Epic Link     | Parent Requirement  | 90%        |
      | Issue Type    | Requirement Type    | 85%        |
      | Fix Version/s | Release Version     | 85%        |
      | Status        | Status              | 80% (needs mapping) |
      | Labels        | Tags                | 90%        |
    And suggested mappings should be shown with dashed lines
    And required fields (Issue Key, Summary) should be marked with asterisks

  Scenario: Create field mapping with drag and drop
    Given I see the field lists
    When I drag "Summary" from JIRA fields
    And drop it on "Title" in Fern fields
    Then a solid line should connect the fields
    And a preview should show: "PROJ-123: Fix login bug" → "Fix login bug"

  Scenario: Configure status value mapping
    Given I have mapped JIRA Status to Fern Status
    When I click on the connection line
    Then a transformation dialog should appear with:
      | JIRA Status   | Maps To     |
      | To Do         | Pending     |
      | In Progress   | Active      |
      | Done          | Completed   |
      | Blocked       | On Hold     |
    And I should be able to add custom mappings

  Scenario: Validate epic hierarchy mapping
    Given I have mapped "Epic Link" to "Parent Requirement"
    When I hover over the mapping line
    Then I should see a tooltip explaining: "Creates parent-child relationships for hierarchical coverage reporting"
    And when I save the mapping
    Then the system should validate that epic issues will be synchronized
    And show a preview of the hierarchy structure that will be created
```

### UC-11: Project PM Linking

#### UC-11-01: Link Project to PM Connector

**As a** Manager  
**I want to** link my project to a PM connector  
**So that** my team can associate tests with PM items

##### Acceptance Criteria

```gherkin
Feature: Project PM Linking
  As a Manager
  I want to link projects to PM tools
  So that we have traceability

  Background:
    Given I am authenticated with manager role
    And I have a project "E-Commerce Frontend"
    And at least one PM connector exists

  Scenario: Access PM integration settings
    Given I am on the project details page
    When I click on the "Integrations" tab
    Then I should see a "PM Tools" section
    And it should show available PM connectors

  Scenario: Add PM link with inline form
    Given I am in the PM Tools section
    When I click "Link PM Tool"
    Then an inline form should expand showing:
      | Field         | Description                            |
      | Connector     | Dropdown of available connectors       |
      | Project Key   | External project identifier            |
      | Sync Settings | How often to sync (if applicable)      |
    And the form should not be a modal

  Scenario: Validate external project
    Given I have selected "Production JIRA" connector
    When I enter "ECOM" as the project key
    And I click "Validate"
    Then the system should check if the project exists
    And show project details if found:
      | Field         | Value                                  |
      | Name          | E-Commerce Project                     |
      | Lead          | john.doe@company.com                   |
      | Issue Count   | 234 open issues                        |
```

#### UC-11-02: Use PM Labels in Tests

**As a** Team Member  
**I want to** use labels to link tests to PM items  
**So that** we have requirement traceability

##### Acceptance Criteria

```gherkin
Feature: PM Labels in Tests
  As a Team Member
  I want to use PM labels
  So that tests are linked to requirements

  Background:
    Given my project is linked to a PM connector
    And I am writing or viewing tests

  Scenario: Add PM label to test
    Given I am viewing a test case
    When I add a label "jira:ECOM-123"
    Then the label should be validated against the PM tool
    And if valid, show the issue title as a tooltip
    And the label should be styled with the PM tool's branding

  Scenario: View linked PM items
    Given a test has PM labels
    When I view the test details
    Then I should see a "Linked Requirements" section
    And each linked item should show:
      | Field         | Example                                |
      | Icon          | 🎯 (JIRA icon)                        |
      | ID            | ECOM-123                               |
      | Title         | Add shopping cart functionality        |
      | Status        | In Progress                            |
      | Link          | → Open in JIRA                         |
```

### UC-12: PM Connector Operations

#### UC-12-01: Test Connection

**As an** Admin  
**I want to** test PM connector connections  
**So that** I can verify the integration is working

##### Acceptance Criteria

```gherkin
Feature: Connection Testing
  As an Admin
  I want to test connections
  So that I can diagnose issues

  Scenario: Test existing connector
    Given I have a configured PM connector
    When I click the "Test Connection" quick action
    Then a non-blocking test should run
    And show real-time progress
    And update the health status indicator
    And log results for troubleshooting
```

#### UC-12-02: View Sync History

**As an** Admin or Manager  
**I want to** view synchronization history  
**So that** I can monitor integration health

##### Acceptance Criteria

```gherkin
Feature: Sync History
  As an Admin or Manager
  I want to view sync history
  So that I can monitor operations

  Scenario: View sync timeline
    Given a PM connector with sync history
    When I click on "View History"
    Then I should see a timeline view showing:
      | Time          | Operation   | Status    | Details              |
      | 2 hours ago   | Auto Sync   | Success   | 45 items synced      |
      | 8 hours ago   | Auto Sync   | Partial   | 43/45 items synced   |
      | 1 day ago     | Manual Sync | Failed    | Authentication error |
    And each entry should be expandable for details
```

## Error Handling

### Connection Errors
- Show specific error messages (not generic "Connection failed")
- Provide actionable recovery steps
- Include "Retry" options where appropriate
- Log detailed errors for admin troubleshooting

### Validation Errors
- Inline validation with immediate feedback
- Clear error messages explaining what's wrong
- Suggestions for fixing the error
- Prevent form submission until errors are resolved

### Sync Errors
- Partial sync support (don't fail everything if one item fails)
- Detailed sync reports showing what succeeded/failed
- Automatic retry with exponential backoff
- Email notifications for critical failures

## Security Considerations

1. **Credential Storage**
   - Encrypt all credentials at rest
   - Never display credentials after saving
   - Use secure credential update flow
   - Audit credential access

2. **API Access**
   - Use least-privilege API permissions
   - Implement rate limiting
   - Monitor for suspicious activity
   - Regular security audits

3. **Data Privacy**
   - Only sync necessary fields
   - Respect data retention policies
   - Allow data purging
   - Maintain sync audit logs

## Performance Requirements

1. **Response Times**
   - List view: < 500ms
   - Create connector: < 2s
   - Test connection: < 5s
   - Field mapping: < 1s

2. **Scalability**
   - Support 100+ connectors per instance
   - Handle 10,000+ linked items per connector
   - Concurrent sync operations
   - Efficient pagination

## Success Metrics

1. **Adoption**
   - % of projects with PM links
   - Average connectors per organization
   - Daily active PM label usage

2. **Reliability**
   - Connector uptime %
   - Sync success rate
   - Mean time to recovery

3. **User Satisfaction**
   - Time to create first connector
   - Support tickets related to PM connectors
   - User feedback scores