# Requirements: JIRA Field Mapping

## Introduction

This feature lets a project manager map standard JIRA issue fields onto Fern's
requirement fields so that JIRA issues can be synchronized into Fern with the
correct metadata. Mappings are configured per Fern project against an existing
JIRA connection (see `internal/domains/integrations/`) and consumed by the sync
feature (issue #27) when translating JIRA issues into Fern requirements.

UI reference: see `jira-field-mapping-ui-mock.png` for the target layout.

## Requirements

### Requirement 1: Field Mapping Configuration

**User Story:** As a project manager, I want to map JIRA fields to Fern
requirement fields for my project, so that JIRA issues sync into Fern with the
correct requirement metadata.

#### Acceptance Criteria

1. THE SYSTEM SHALL expose the following Fern requirement fields as mapping
   targets: Requirement ID (required, single value), Requirement Title
   (required, text), Requirement Description (optional, rich text), Parent
   Requirement (optional, reference), Requirement Type (optional, select),
   Release Version (optional, text), Requirement Status (optional, select),
   Tags/Labels (optional, multi-value).
2. WHEN a project manager opens the Field Mapping screen for a project that has
   a connected JIRA integration THE SYSTEM SHALL display a two-column
   drag-and-drop interface: JIRA fields on the left, Fern requirement fields on
   the right, with the available JIRA fields fetched from the connected instance
   via the backend. (The drag-and-drop UI and SVG connection-line rendering were
   delivered in issue #25; this feature wires it to real data and persistence.)
3. WHEN a project has no previously saved mapping THE SYSTEM SHALL pre-populate
   sensible defaults (Requirement ID → Issue Key, Requirement Title → Summary,
   Requirement Description → Description, Parent Requirement → Epic Link,
   Requirement Type → Issue Type, Requirement Status → Status, Tags/Labels →
   Labels) and SHALL leave Release Version unmapped.
4. WHEN a project manager selects a JIRA field for a Fern field THE SYSTEM SHALL
   update the in-memory mapping for that row without persisting until the user
   saves.
5. WHEN a project manager selects "— unmapped —" for a Fern field THE SYSTEM
   SHALL treat that field as unmapped on save.
6. WHEN a project manager clicks Reset THE SYSTEM SHALL restore the mapping to
   the defaults defined in criterion 3 without persisting.

### Requirement 2: Required Field Enforcement

**User Story:** As a project manager, I want the system to prevent me from
saving an incomplete mapping, so that synchronized requirements always have the
identifiers and titles they need.

#### Acceptance Criteria

1. THE SYSTEM SHALL mark Requirement ID and Requirement Title as required.
2. IF either Requirement ID or Requirement Title is unmapped THEN THE SYSTEM
   SHALL disable the Save Mapping action and display a warning identifying which
   required fields are unmapped.
3. WHEN a save request is submitted with any required Fern field unmapped THE
   SYSTEM SHALL reject the request with a validation error and SHALL NOT modify
   the persisted mapping.

### Requirement 3: One-to-One Mapping Conflicts

**User Story:** As a project manager, I want each JIRA field to map to at most
one Fern field, so that requirement data is not duplicated or ambiguous.

#### Acceptance Criteria

1. WHEN a project manager selects a JIRA field that is already mapped to a
   different Fern field THE SYSTEM SHALL flag both rows as conflicting and
   display an inline error indicating the conflict.
2. WHILE any conflict is present THE SYSTEM SHALL disable the Save Mapping
   action.
3. WHEN a save request is submitted with a duplicate JIRA-field assignment THE
   SYSTEM SHALL reject the request with a validation error identifying the
   conflicting fields.

### Requirement 4: Multi-Value JIRA Field Handling

**User Story:** As a project manager, I want to control how multi-value JIRA
fields (e.g. Fix Version/s) collapse into single-value Fern fields, so that the
synchronized data matches what my team expects.

#### Acceptance Criteria

1. WHEN a Fern field whose Fern type is single-value is mapped to a JIRA field
   that supports multiple values THE SYSTEM SHALL prompt the project manager to
   choose a reduction strategy from: "Use first value only", "Concatenate all
   values", or "Create separate entries per value".
2. THE SYSTEM SHALL persist the selected reduction strategy together with the
   mapping.
3. WHEN a Fern field whose type is multi-value (Tags/Labels) is mapped to a
   multi-value JIRA field THE SYSTEM SHALL NOT prompt for a reduction strategy
   and SHALL preserve all values on sync.

### Requirement 5: Persistence and Scope

**User Story:** As a project manager, I want my field mappings to be saved per
project and survive page reloads, so that I do not need to reconfigure them
each session.

#### Acceptance Criteria

1. WHEN a project manager clicks Save Mapping with a valid configuration THE
   SYSTEM SHALL persist the mapping scoped to the current Fern project.
2. WHEN the Field Mapping screen is reopened for a project that has a saved
   mapping THE SYSTEM SHALL load and display the saved mapping.
3. THE SYSTEM SHALL store each Fern project's mapping independently; updating
   one project's mapping SHALL NOT affect another project's mapping.
4. WHEN a mapping is saved THE SYSTEM SHALL record the saving user and the
   timestamp of the change for audit purposes.

### Requirement 6: Mapping Status Display

**User Story:** As a project manager, I want at-a-glance feedback on the
completeness of my mapping, so that I can see whether I am ready to sync.

#### Acceptance Criteria

1. THE SYSTEM SHALL display the current JIRA connection status (connected /
   disconnected) and the connected instance and project on the Field Mapping
   screen.
2. THE SYSTEM SHALL display counts of mapped Fern fields, unmapped Fern
   fields, and whether all required Fern fields are mapped.
3. WHEN the mapping is successfully saved THE SYSTEM SHALL display a transient
   confirmation message.
4. WHILE the project has no connected JIRA integration THE SYSTEM SHALL hide
   the mapping selects and SHALL display a prompt directing the user to
   configure a JIRA connection first.

### Requirement 7: Access Control

**User Story:** As a project owner, I want only authorized users to change a
project's field mapping, so that other teams cannot misconfigure my project.

#### Acceptance Criteria

1. THE SYSTEM SHALL permit only project managers (users with manage rights on
   the project) and administrators to view and modify a project's field
   mapping.
2. WHEN a user without manage rights requests to load or modify a project's
   field mapping THE SYSTEM SHALL deny the request with an authorization error.
3. THE SYSTEM SHALL log every create, update, and delete of a field mapping
   with the acting user, project, and timestamp.

### Requirement 8: Effect on Synchronization

**User Story:** As a project manager, I want my saved mapping to be used by the
next JIRA sync, so that issues flow into Fern with the configuration I chose.

#### Acceptance Criteria

1. WHEN a JIRA sync runs for a project THE SYSTEM SHALL use the project's saved
   field mapping (and reduction strategies) to translate each JIRA issue into
   a Fern requirement.
2. WHEN a project has no saved mapping THE SYSTEM SHALL refuse to run a JIRA
   sync for that project and SHALL surface a clear error indicating that a
   mapping must be configured first.
3. WHEN a field mapping is changed THE SYSTEM SHALL apply the new mapping on
   subsequent syncs and SHALL NOT retroactively rewrite previously synced
   requirements.

### Requirement 9: Non-Functional Requirements

**User Story:** As a platform operator, I want the field mapping feature to follow
Fern Platform's existing conventions for testing, persistence, and observability,
so that it does not introduce operational surprises.

#### Acceptance Criteria

1. THE SYSTEM SHALL provide unit-test coverage for the field-mapping service
   (validation, persistence, conflict detection) and integration-test coverage
   for the GraphQL endpoints that read and write mappings.
2. THE SYSTEM SHALL implement persistence using the project's GORM + golang-
   migrate conventions (new migration under `migrations/`, repository pattern
   in `internal/domains/integrations/`).
3. THE SYSTEM SHALL expose mapping operations through the existing GraphQL
   schema (`internal/reporter/graphql/`) and follow the existing
   schema-first + generated resolver pattern.
4. WHILE a save is in flight THE SYSTEM SHALL prevent concurrent edits to the
   same project's mapping from producing a lost update (last-writer-wins is
   acceptable only if the second writer sees the resulting state).
5. THE SYSTEM SHALL emit structured logs (logrus) for save and validation
   failure events.

## Constraints

- Mappings are scoped to a Fern project and a JIRA connection that already
  exists; this feature does not create or modify JIRA connections.
- Fern requirement fields are fixed for this feature; adding new Fern-side
  requirement fields is out of scope.
- Custom JIRA fields are out of scope for this iteration; only standard JIRA
  fields enumerated by the JIRA REST API field listing are supported.
- The actual synchronization of issues (issue #27) and the linking of tests to
  requirements (issue #28) are separate specs; this spec covers only
  configuring the mapping.
- The UI is built with the existing static HTML/CSS/JS frontend in `web/`; no
  new SPA framework is introduced.
