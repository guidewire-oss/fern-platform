package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// searchPaginated runs a cursor-paginated GET against /rest/api/3/search/jql, invoking
// onPage once per successful page. It owns request construction, auth, 429 retry with
// exponential backoff (and ctx cancellation), non-200 error handling, response body
// closing on every path, and nextPageToken-driven pagination.
//
// onPage decodes the page body and returns that page's nextPageToken and issue count;
// pagination stops when the token is empty or the page yields zero issues. opName is
// woven into error messages (e.g. "search", "epic releases"). The body is closed by the
// caller (this helper) immediately after onPage returns, so onPage must finish decoding
// before it returns.
func (c *DefaultJiraClient) searchPaginated(
	ctx context.Context,
	baseURL, username, credential string,
	authType AuthenticationType,
	jql, fields, opName string,
	onPage func(body io.Reader, pageNum, statusCode int, pageStart time.Time) (nextPageToken string, issueCount int, err error),
) error {
	const pageSize = 100

	nextPageToken := ""
	pageNum := 0
	for {
		pageNum++
		pageStart := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/rest/api/3/search/jql", baseURL), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		q := req.URL.Query()
		q.Set("jql", jql)
		q.Set("fields", fields)
		q.Set("maxResults", strconv.Itoa(pageSize))
		if nextPageToken != "" {
			q.Set("nextPageToken", nextPageToken)
		}
		req.URL.RawQuery = q.Encode()
		c.setAuthHeader(req, username, credential, authType)

		const maxAttempts = 3
		var resp *http.Response
		for attempt := 0; attempt < maxAttempts; attempt++ {
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to execute %s request: %w", opName, err)
			}
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
			resp.Body.Close()
			if attempt+1 < maxAttempts {
				delay := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return fmt.Errorf("JIRA %s request failed: status %d url=%s body=%s", opName, resp.StatusCode, req.URL.String(), string(body))
		}

		token, count, cbErr := onPage(resp.Body, pageNum, resp.StatusCode, pageStart)
		resp.Body.Close()
		if cbErr != nil {
			return cbErr
		}

		if token == "" || count == 0 {
			break
		}
		nextPageToken = token
	}
	return nil
}

// GetEpicReleases returns distinct non-empty values of a custom release field across all Epics
// in the project, sorted alphabetically. jiraFieldID is the full custom field ID
// (e.g. "customfield_10077"); the numeric part is extracted internally for JQL.
func (c *DefaultJiraClient) GetEpicReleases(ctx context.Context, baseURL, projectKey, jiraFieldID, username, credential string, authType AuthenticationType) ([]string, error) {
	numericID := extractNumericFieldID(jiraFieldID)
	// Scope to epics touched within the past year. The release field is a free-text
	// string with no date semantics, so we filter on the epic's `updated` timestamp:
	// this both shrinks the dropdown to current/active releases and cuts the number of
	// epics paginated, which is the dominant cost of building the picker.
	jql := fmt.Sprintf(`project = %q AND issuetype = Epic AND cf[%s] is not EMPTY AND updated >= -52w ORDER BY cf[%s] ASC`, projectKey, numericID, numericID)
	fieldParam := jiraFieldID // e.g. "customfield_10077" for the fields parameter

	seen := make(map[string]bool)
	start := time.Now()
	log.Printf("[CoverageJiraClient] GetEpicReleases: url=%s project=%s field=%s", baseURL, projectKey, jiraFieldID)

	err := c.searchPaginated(ctx, baseURL, username, credential, authType, jql, fieldParam, "epic releases",
		func(body io.Reader, _ int, _ int, _ time.Time) (string, int, error) {
			var page struct {
				NextPageToken string `json:"nextPageToken"`
				Issues        []struct {
					Fields map[string]json.RawMessage `json:"fields"`
				} `json:"issues"`
			}
			if decodeErr := json.NewDecoder(body).Decode(&page); decodeErr != nil {
				return "", 0, fmt.Errorf("failed to decode epic releases response: %w", decodeErr)
			}
			for _, issue := range page.Issues {
				raw, ok := issue.Fields[fieldParam]
				if !ok || string(raw) == "null" {
					continue
				}
				var val string
				if err := json.Unmarshal(raw, &val); err != nil || val == "" {
					continue
				}
				seen[val] = true
			}
			return page.NextPageToken, len(page.Issues), nil
		})
	if err != nil {
		return nil, err
	}

	releases := make([]string, 0, len(seen))
	for v := range seen {
		releases = append(releases, v)
	}
	sort.Strings(releases)
	log.Printf("[CoverageJiraClient] GetEpicReleases: found %d distinct releases project=%s duration=%dms", len(releases), projectKey, time.Since(start).Milliseconds())
	return releases, nil
}

// SearchIssues executes a JQL query against JIRA and returns all matching issues, paginating as needed.
// Uses GET /rest/api/3/search/jql (Atlassian Cloud cursor-based search endpoint).
func (c *DefaultJiraClient) SearchIssues(ctx context.Context, baseURL, username, credential string, authType AuthenticationType, jql string, fields []string) ([]JiraIssue, error) {
	var all []JiraIssue
	totalStart := time.Now()
	jqlSummary := jql
	if len(jqlSummary) > 80 {
		jqlSummary = jqlSummary[:80] + "..."
	}
	log.Printf("[CoverageJiraClient] SearchIssues: url=%s jql=%q", baseURL, jqlSummary)

	pages := 0
	err := c.searchPaginated(ctx, baseURL, username, credential, authType, jql, strings.Join(fields, ","), "search",
		func(body io.Reader, pageNum, statusCode int, pageStart time.Time) (string, int, error) {
			pages = pageNum

			var page struct {
				NextPageToken string `json:"nextPageToken"`
				Issues        []struct {
					Key    string `json:"key"`
					Fields struct {
						Summary string `json:"summary"`
						Status  struct {
							Name string `json:"name"`
						} `json:"status"`
						IssueType struct {
							Name    string `json:"name"`
							Subtask bool   `json:"subtask"`
						} `json:"issuetype"`
						Parent *struct {
							Key    string `json:"key"`
							Fields struct {
								IssueType struct {
									Name string `json:"name"`
								} `json:"issuetype"`
							} `json:"fields"`
						} `json:"parent"`
					} `json:"fields"`
				} `json:"issues"`
			}
			if decodeErr := json.NewDecoder(body).Decode(&page); decodeErr != nil {
				return "", 0, fmt.Errorf("failed to decode search response: %w", decodeErr)
			}

			log.Printf("[CoverageJiraClient] SearchIssues: page=%d count=%d status=%d duration=%dms", pageNum, len(page.Issues), statusCode, time.Since(pageStart).Milliseconds())

			for _, raw := range page.Issues {
				issue := JiraIssue{
					Key:        raw.Key,
					Summary:    raw.Fields.Summary,
					StatusName: raw.Fields.Status.Name,
					IssueType:  raw.Fields.IssueType.Name,
					Subtask:    raw.Fields.IssueType.Subtask,
				}
				if raw.Fields.Parent != nil {
					issue.Parent = &JiraParent{
						Key:       raw.Fields.Parent.Key,
						IssueType: raw.Fields.Parent.Fields.IssueType.Name,
					}
				}
				all = append(all, issue)
			}

			return page.NextPageToken, len(page.Issues), nil
		})
	if err != nil {
		return nil, err
	}

	log.Printf("[CoverageJiraClient] SearchIssues: done jql=%q pages=%d total=%d duration=%dms", jqlSummary, pages, len(all), time.Since(totalStart).Milliseconds())
	return all, nil
}
