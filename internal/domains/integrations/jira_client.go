package integrations

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
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
	// Test by getting current user info
	endpoint := fmt.Sprintf("%s/rest/api/2/myself", url)
	
	log.Printf("[DefaultJiraClient] Testing connection to: %s with auth type: %v, username: %s", endpoint, authType, username)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		log.Printf("[DefaultJiraClient] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header
	c.setAuthHeader(req, username, credential, authType)

	log.Printf("[DefaultJiraClient] Sending GET request to JIRA: %s", endpoint)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[DefaultJiraClient] Failed to connect to JIRA at %s: %v", url, err)
		return fmt.Errorf("failed to connect to JIRA: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[DefaultJiraClient] Response status from %s: %d", url, resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		// Read error response body for more details
		var errorBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorBody); err == nil {
			log.Printf("[DefaultJiraClient] Error response from %s: %v", url, errorBody)
			return fmt.Errorf("JIRA authentication failed: status %d, message: %v", resp.StatusCode, errorBody)
		}
		return fmt.Errorf("JIRA authentication failed: status %d", resp.StatusCode)
	}

	log.Printf("[DefaultJiraClient] Connection test successful for %s", url)
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