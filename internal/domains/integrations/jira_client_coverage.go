package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetVersions fetches all fix versions for a JIRA project.
func (c *DefaultJiraClient) GetVersions(ctx context.Context, baseURL, projectKey, username, credential string, authType AuthenticationType) ([]JiraVersion, error) {
	endpoint := fmt.Sprintf("%s/rest/api/3/project/%s/versions", baseURL, projectKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuthHeader(req, username, credential, authType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
	return versions, nil
}

// SearchIssues executes a JQL query against JIRA and returns all matching issues, paginating as needed.
func (c *DefaultJiraClient) SearchIssues(ctx context.Context, baseURL, username, credential string, authType AuthenticationType, jql string, fields []string) ([]JiraIssue, error) {
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/search", baseURL)
	const pageSize = 100

	var all []JiraIssue
	startAt := 0

	for {
		body, err := json.Marshal(map[string]any{
			"jql":        jql,
			"fields":     fields,
			"maxResults": pageSize,
			"startAt":    startAt,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal search request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		c.setAuthHeader(req, username, credential, authType)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to search issues: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("JIRA search request failed: status %d", resp.StatusCode)
		}

		var page struct {
			StartAt    int `json:"startAt"`
			MaxResults int `json:"maxResults"`
			Total      int `json:"total"`
			Issues     []struct {
				Key    string `json:"key"`
				Fields struct {
					Summary   string `json:"summary"`
					Status    struct{ Name string `json:"name"` } `json:"status"`
					IssueType struct{ Name string `json:"name"` } `json:"issuetype"`
					Parent    *struct {
						Key    string `json:"key"`
						Fields struct {
							IssueType struct{ Name string `json:"name"` } `json:"issuetype"`
						} `json:"fields"`
					} `json:"parent"`
				} `json:"fields"`
			} `json:"issues"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return nil, fmt.Errorf("failed to decode search response: %w", err)
		}

		for _, raw := range page.Issues {
			issue := JiraIssue{
				Key:        raw.Key,
				Summary:    raw.Fields.Summary,
				StatusName: raw.Fields.Status.Name,
				IssueType:  raw.Fields.IssueType.Name,
			}
			if raw.Fields.Parent != nil {
				issue.Parent = &JiraParent{
					Key:       raw.Fields.Parent.Key,
					IssueType: raw.Fields.Parent.Fields.IssueType.Name,
				}
			}
			all = append(all, issue)
		}

		if startAt+len(page.Issues) >= page.Total {
			break
		}
		startAt += len(page.Issues)
	}

	return all, nil
}
