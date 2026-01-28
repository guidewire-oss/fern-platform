# Summary Domain

The Summary domain provides aggregated test run statistics and reporting capabilities.

## Structure

```
summary/
├── domain/          # Core business entities and repository interface
│   ├── summary.go   # Domain entities (SummaryRequest, SummaryResponse, etc.)
│   └── repository.go # Repository interface
├── application/     # Business logic and use cases
│   └── summary_service.go # Summary aggregation service
├── infrastructure/  # External integrations (database)
│   └── gorm_summary_repository.go # GORM-based repository implementation
└── interfaces/      # HTTP handlers and adapters
    ├── summary_handler.go # Gin HTTP handler
    └── summary_handler_test.go # Handler tests
```

## Features

- **Flexible Grouping**: Aggregate test results by any combination of tags (e.g., testtype, component, owner, category)
- **Status Aggregation**: Automatically calculate overall test run status (passed/failed) based on individual test results
- **Multi-Test Run Support**: Handle multiple test runs with the same project and seed
- **Dynamic Summaries**: Generate summaries with only non-zero counts for cleaner output

## API

### Get Summary

**Endpoint**: `GET /api/v1/summary/:projectId/:seed` (requires authentication)

**Query Parameters**:
- `group_by` (optional, repeatable): Tag categories to group results by

**Example Request**:
```
GET /api/v1/summary/project-123/seed-456?group_by=testtype&group_by=component&group_by=owner
```

**Example Response**:
```json
{
  "project_id": "project-123",
  "seed": "seed-456",
  "branch": "main",
  "sha": "abc123def456",
  "status": "passed",
  "tests": 10,
  "start_time": "2025-10-20T10:00:00Z",
  "end_time": "2025-10-20T10:15:00Z",
  "summary": [
    {
      "testtype": "acceptance",
      "component": "jspolicy",
      "owner": "capitola",
      "total": 5,
      "passed": 4,
      "failed": 1
    },
    {
      "testtype": "acceptance",
      "component": "keda",
      "owner": "danville",
      "total": 5,
      "passed": 5
    }
  ]
}
```

## Usage

### Initialize the Service

```go
import (
    "github.com/guidewire-oss/fern-platform/internal/domains/summary/application"
    "github.com/guidewire-oss/fern-platform/internal/domains/summary/infrastructure"
    "github.com/guidewire-oss/fern-platform/internal/domains/summary/interfaces"
)

// Create repository
repo := infrastructure.NewGormSummaryRepository(db)

// Create service
service := application.NewSummaryService(repo)

// Create handler
handler := interfaces.NewSummaryHandler(service)

// Register route
router.GET("/api/v1/summary/:projectId/:seed", handler.GetSummary)
```

### Query Summary

```go
import "github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"

req := domain.SummaryRequest{
    ProjectUUID: "project-123",
    Seed:        "seed-456",
    GroupBy:     []string{"testtype", "component"},
}

summary, err := service.GetSummary(req)
if err != nil {
    // Handle error
}

// Use summary...
```

## Design Decisions

1. **Seed as Run ID**: The `seed` parameter maps to the `run_id` field in the database, which stores test run identifiers as strings
2. **Dynamic Grouping**: Support any combination of tag categories for flexible reporting
3. **Unspecified Tags**: When a spec doesn't have a requested tag category, it's grouped under "unspecified"
4. **Sparse Output**: Only include non-zero status counts in the summary to reduce response size
5. **Deterministic Sorting**: Sort summary results by grouping keys for consistent output

## Testing

Run tests with:
```bash
go test ./internal/domains/summary/interfaces/... -v
```

The test suite covers:
- Single component grouping
- Multiple component grouping
- Failed test handling
- Non-existent tag handling
- Empty results
- No grouping (overall summary)
