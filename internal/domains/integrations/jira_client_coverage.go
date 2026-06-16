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

// GetEpicReleases returns distinct non-empty values of a custom release field across all Epics
// in the project, sorted alphabetically. jiraFieldID is the full custom field ID
// (e.g. "customfield_10077"); the numeric part is extracted internally for JQL.
func (c *DefaultJiraClient) GetEpicReleases(ctx context.Context, baseURL, projectKey, jiraFieldID, username, credential string, authType AuthenticationType) ([]string, error) {
	const pageSize = 100

	numericID := extractNumericFieldID(jiraFieldID)
	jql := fmt.Sprintf(`project = %q AND issuetype = Epic AND cf[%s] is not EMPTY ORDER BY cf[%s] ASC`, projectKey, numericID, numericID)
	fieldParam := jiraFieldID // e.g. "customfield_10077" for the fields parameter

	seen := make(map[string]bool)
	nextPageToken := ""
	pageNum := 0
	start := time.Now()
	log.Printf("[CoverageJiraClient] GetEpicReleases: url=%s project=%s field=%s", baseURL, projectKey, jiraFieldID)

	for {
		pageNum++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/rest/api/3/search/jql", baseURL), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		q := req.URL.Query()
		q.Set("jql", jql)
		q.Set("fields", fieldParam)
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
				return nil, fmt.Errorf("failed to fetch epic releases: %w", err)
			}
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
			resp.Body.Close()
			if attempt+1 < maxAttempts {
				delay := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return nil, fmt.Errorf("JIRA epic releases request failed: status %d body=%s", resp.StatusCode, string(body))
		}

		var page struct {
			NextPageToken string `json:"nextPageToken"`
			Issues        []struct {
				Fields map[string]json.RawMessage `json:"fields"`
			} `json:"issues"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return nil, fmt.Errorf("failed to decode epic releases response: %w", err)
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

		if page.NextPageToken == "" || len(page.Issues) == 0 {
			break
		}
		nextPageToken = page.NextPageToken
	}

	releases := make([]string, 0, len(seen))
	for v := range seen {
		releases = append(releases, v)
	}
	sort.Strings(releases)
	log.Printf("[CoverageJiraClient] GetEpicReleases: found %d distinct releases project=%s duration=%dms", len(releases), projectKey, time.Since(start).Milliseconds())
	return releases, nil
}

// GetVersions fetches all fix versions for a JIRA project.
func (c *DefaultJiraClient) GetVersions(ctx context.Context, baseURL, projectKey, username, credential string, authType AuthenticationType) ([]JiraVersion, error) {
	endpoint := fmt.Sprintf("%s/rest/api/3/project/%s/versions", baseURL, projectKey)
	start := time.Now()
	log.Printf("[CoverageJiraClient] GetVersions: url=%s project=%s", baseURL, projectKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuthHeader(req, username, credential, authType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[CoverageJiraClient] GetVersions: request failed url=%s err=%v", baseURL, err)
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[CoverageJiraClient] GetVersions: non-200 status=%d url=%s", resp.StatusCode, baseURL)
		return nil, fmt.Errorf("JIRA versions request failed: status %d", resp.StatusCode)
	}

	var raw []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Released    bool   `json:"released"`
		ReleaseDate string `json:"releaseDate,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode versions response: %w", err)
	}

	versions := make([]JiraVersion, len(raw))
	for i, v := range raw {
		versions[i] = JiraVersion{ID: v.ID, Name: v.Name, Released: v.Released, ReleaseDate: v.ReleaseDate}
	}
	log.Printf("[CoverageJiraClient] GetVersions: returned %d versions project=%s duration=%dms", len(versions), projectKey, time.Since(start).Milliseconds())
	return versions, nil
}

// SearchIssues executes a JQL query against JIRA and returns all matching issues, paginating as needed.
// Uses GET /rest/api/3/search/jql (Atlassian Cloud cursor-based search endpoint).
func (c *DefaultJiraClient) SearchIssues(ctx context.Context, baseURL, username, credential string, authType AuthenticationType, jql string, fields []string) ([]JiraIssue, error) {
	const pageSize = 100

	var all []JiraIssue
	nextPageToken := ""
	pageNum := 0
	totalStart := time.Now()
	jqlSummary := jql
	if len(jqlSummary) > 80 {
		jqlSummary = jqlSummary[:80] + "..."
	}
	log.Printf("[CoverageJiraClient] SearchIssues: url=%s jql=%q", baseURL, jqlSummary)

	for {
		pageNum++
		pageStart := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/rest/api/3/search/jql", baseURL), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		q := req.URL.Query()
		q.Set("jql", jql)
		q.Set("fields", strings.Join(fields, ","))
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
				return nil, fmt.Errorf("failed to search issues: %w", err)
			}
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
			resp.Body.Close()
			if attempt+1 < maxAttempts {
				delay := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return nil, fmt.Errorf("JIRA search request failed: status %d url=%s body=%s", resp.StatusCode, req.URL.String(), string(errBody))
		}

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
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return nil, fmt.Errorf("failed to decode search response: %w", err)
		}

		log.Printf("[CoverageJiraClient] SearchIssues: page=%d count=%d status=%d duration=%dms", pageNum, len(page.Issues), resp.StatusCode, time.Since(pageStart).Milliseconds())

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

		if page.NextPageToken == "" || len(page.Issues) == 0 {
			break
		}
		nextPageToken = page.NextPageToken
	}

	log.Printf("[CoverageJiraClient] SearchIssues: done jql=%q pages=%d total=%d duration=%dms", jqlSummary, pageNum, len(all), time.Since(totalStart).Milliseconds())
	return all, nil
}
