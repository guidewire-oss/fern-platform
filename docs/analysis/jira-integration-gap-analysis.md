# JIRA Integration Gap Analysis

## Executive Summary

This document analyzes the gaps between existing JIRA integration documentation/implementation in Fern Platform and the new requirements provided.

## New Requirements Overview

1. **Managers connect to JIRA project when creating/editing projects**
2. **Visual field mapping from JIRA fields to Fern common fields**
3. **Test labeling/tagging with JIRA IDs**
4. **Release reporting showing epic/story test coverage**
5. **Visual, intuitive release readiness reports**
6. **Optional webhook-based JIRA status updates**

## Current State Analysis

### Existing Documentation

#### 1. PM Connectors Management (UC-10)
- **Location**: `/docs/use-cases/10-pm-connectors-management.md`
- **Status**: Well-documented use cases for PM connector management
- **Coverage**:
  - ✅ Creating PM connectors (including JIRA)
  - ✅ Visual field mapping interface (UC-10-03)
  - ✅ Project PM linking (UC-11)
  - ✅ PM labels in tests (UC-11-02)
  - ✅ Connection testing and validation
  - ✅ Sync history tracking

#### 2. Requirements Traceability RFC (RFC-003)
- **Location**: `/docs/rfc/rfc-003-requirements-traceability-and-test-coverage-intelligence.md`
- **Status**: Comprehensive RFC for requirements traceability
- **Coverage**:
  - ✅ Test annotation framework with JIRA support
  - ✅ Requirements coverage dashboard
  - ✅ Sprint/Release coverage analysis
  - ✅ JIRA webhook integration (mentioned)
  - ✅ Coverage reporting and metrics
  - ✅ Multi-system support architecture (JIRA-first MVP)

### Existing Implementation

#### 1. Test Framework Support
- **Mock Server**: `mock_graphql_server.go` includes field mapping structures
- **Field Mapping Types**:
  ```go
  type FieldMapping struct {
      ID              string
      SourcePath      string
      TargetField     string
      TransformType   string
      IsActive        bool
      Order           int
  }
  ```

#### 2. Acceptance Tests
- Multiple test files for PM connector functionality
- Tests cover connector creation, project linking, and label usage

## Gap Analysis

### ✅ Requirements Already Covered

1. **Managers connect to JIRA project** - Fully documented in UC-11-01
2. **Visual field mapping** - Documented in UC-10-03 with drag-and-drop interface
3. **Test labeling with JIRA IDs** - Covered in UC-11-02 and RFC-003

### 🟡 Partially Covered Requirements

4. **Release reporting showing epic/story test coverage**
   - **Current**: RFC-003 describes sprint/release coverage analysis
   - **Gap**: No specific epic/story hierarchy reporting
   - **Missing**: 
     - Epic-level aggregation of story coverage
     - Visual hierarchy display (Epic → Story → Test)
     - Epic completion metrics based on story testing

5. **Visual, intuitive release readiness reports**
   - **Current**: RFC-003 includes release readiness reports
   - **Gap**: Limited visual design specifications
   - **Missing**:
     - Specific UI/UX mockups for dashboards
     - Interactive visualization components
     - Executive-friendly report templates

### ❌ Not Covered Requirements

6. **Optional webhook-based JIRA status updates**
   - **Current**: RFC-003 mentions webhooks for incoming data
   - **Gap**: No outbound webhook documentation
   - **Missing**:
     - JIRA status update API integration
     - Webhook configuration UI
     - Event mapping (test results → JIRA transitions)
     - Error handling and retry logic

## Detailed Gap Analysis

### Gap 1: Epic/Story Hierarchy Coverage Reporting

**Current State**:
- Flat requirement coverage reporting
- Sprint-level aggregation only

**Required State**:
- Hierarchical coverage display
- Epic → Story → Sub-task relationships
- Coverage roll-up from stories to epics

**Implementation Needs**:
```graphql
type Epic {
  id: ID!
  key: String!
  title: String!
  stories: [Story!]!
  coveragePercentage: Float!
  testCount: Int!
  status: String!
}

type Story {
  id: ID!
  key: String!
  title: String!
  epic: Epic
  tests: [Test!]!
  coverage: Coverage!
}
```

### Gap 2: Enhanced Visual Release Reports

**Current State**:
- Text-based coverage reports
- Basic percentage displays

**Required State**:
- Interactive dashboards
- Visual progress indicators
- Risk heat maps
- Trend charts

**Implementation Needs**:
- Chart.js or D3.js integration
- Dashboard component library
- Export to PDF/PowerPoint
- Real-time updates

### Gap 3: Outbound JIRA Webhook Integration

**Current State**:
- Inbound data sync only
- No automated JIRA updates

**Required State**:
- Bidirectional integration
- Automatic JIRA ticket updates
- Configurable status transitions
- Comment posting with test results

**Implementation Needs**:
```go
type JIRAWebhookConfig struct {
    Enabled         bool
    TransitionRules []TransitionRule
    CommentTemplate string
    RetryPolicy     RetryPolicy
}

type TransitionRule struct {
    TestStatus      string
    JIRATransition  string
    Conditions      []Condition
}
```

## Recommendations

### 1. Enhance Epic/Story Coverage Reporting
- Extend data model to support JIRA hierarchy
- Add GraphQL queries for epic-level aggregation
- Create UI components for tree visualization

### 2. Develop Visual Dashboard Components
- Create reusable React components for:
  - Coverage gauge charts
  - Risk heat maps
  - Trend line graphs
  - Release confidence scores
- Implement export functionality

### 3. Implement Outbound JIRA Integration
- Add webhook configuration to PM connector settings
- Create webhook event processor
- Implement JIRA REST API client for updates
- Add audit logging for all JIRA updates

### 4. Update Documentation
- Add epic/story coverage examples to UC-10
- Document webhook configuration process
- Create visual dashboard user guide
- Add troubleshooting section for JIRA sync

## Implementation Priority

1. **High Priority**: Epic/Story hierarchy reporting (addresses core coverage visibility)
2. **Medium Priority**: Visual dashboard enhancements (improves user experience)
3. **Low Priority**: Outbound webhooks (nice-to-have automation)

## Next Steps

1. Review and validate gaps with stakeholders
2. Create detailed technical specifications for each gap
3. Update existing RFCs or create new ones as needed
4. Plan implementation phases
5. Update acceptance criteria in existing use cases