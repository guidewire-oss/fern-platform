package integrations

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

// DefaultJiraClient implements the JiraClient interface
type DefaultJiraClient struct {
	httpClient *http.Client
}

// NewDefaultJiraClient creates a new default JIRA client
func NewDefaultJiraClient() *DefaultJiraClient {
	return &DefaultJiraClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection tests the JIRA connection
func (c *DefaultJiraClient) TestConnection(ctx context.Context, url, username, credential string, authType AuthenticationType) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/myself", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req, username, credential, authType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to JIRA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorBody); err == nil {
			logrus.WithFields(logrus.Fields{"url": url, "status": resp.StatusCode, "error_body": errorBody}).
				Warn("JIRA authentication failed")
			return fmt.Errorf("JIRA authentication failed: status %d, message: %v", resp.StatusCode, errorBody)
		}
		return fmt.Errorf("JIRA authentication failed: status %d", resp.StatusCode)
	}

	return nil
}

// GetProject retrieves a JIRA project by key
func (c *DefaultJiraClient) GetProject(ctx context.Context, url, projectKey, username, credential string, authType AuthenticationType) (*JiraProject, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/project/%s", url, projectKey)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header
	c.setAuthHeader(req, username, credential, authType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to JIRA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("project '%s' not found", projectKey)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get project: status %d", resp.StatusCode)
	}

	var project JiraProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to parse project response: %w", err)
	}

	return &project, nil
}

// ListFields retrieves the list of standard (non-custom) JIRA fields
func (c *DefaultJiraClient) ListFields(ctx context.Context, baseURL, username, credential string, authType AuthenticationType) ([]JiraField, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/field", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req, username, credential, authType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to JIRA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorBody); err == nil {
			return nil, fmt.Errorf("failed to list fields: status %d, message: %v", resp.StatusCode, errorBody)
		}
		return nil, fmt.Errorf("failed to list fields: status %d", resp.StatusCode)
	}

	var raw []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
		Schema struct {
			Type  string `json:"type"`
			Items string `json:"items,omitempty"`
		} `json:"schema"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse fields response: %w", err)
	}

	var fields []JiraField
	for _, f := range raw {
		if f.Custom {
			continue
		}
		fields = append(fields, JiraField{
			ID:         f.ID,
			Name:       f.Name,
			Custom:     f.Custom,
			MultiValue: f.Schema.Type == "array",
			SchemaType: f.Schema.Type,
		})
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields, nil
}

// setAuthHeader sets the appropriate authentication header
func (c *DefaultJiraClient) setAuthHeader(req *http.Request, username, credential string, authType AuthenticationType) {
	switch authType {
	case AuthTypeAPIToken:
		// For API token, use Basic auth with email:token
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, credential)))
		req.Header.Set("Authorization", "Basic "+auth)
	case AuthTypeOAuth:
		// For OAuth, use Bearer token
		req.Header.Set("Authorization", "Bearer "+credential)
	case AuthTypePersonalAccessToken:
		// For PAT, use Bearer token
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Fern-Platform/1.0")
}