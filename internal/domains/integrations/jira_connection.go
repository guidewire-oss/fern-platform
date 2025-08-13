package integrations

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JiraConnection represents a JIRA integration connection
type JiraConnection struct {
	id                 string
	projectID          string
	name               string
	jiraURL            string
	authenticationType AuthenticationType
	projectKey         string
	username           string
	encryptedCredential string
	status             ConnectionStatus
	isActive           bool
	lastTestedAt       *time.Time
	createdAt          time.Time
	updatedAt          time.Time
}

// JiraClient interface for testing connections
type JiraClient interface {
	TestConnection(ctx context.Context, url, username, credential string, authType AuthenticationType) error
	GetProject(ctx context.Context, url, projectKey, username, credential string, authType AuthenticationType) (*JiraProject, error)
}

// NewJiraConnection creates a new JIRA connection
func NewJiraConnection(projectID, name, jiraURL string, authType AuthenticationType, projectKey, username, credential string) (*JiraConnection, error) {
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}
	if name == "" {
		return nil, errors.New("connection name is required")
	}
	if !isValidJiraURL(jiraURL) {
		return nil, errors.New("JIRA URL must start with http:// or https://")
	}
	if projectKey == "" {
		return nil, errors.New("project key is required")
	}
	if username == "" {
		return nil, errors.New("username is required")
	}
	if credential == "" {
		return nil, errors.New("credential is required")
	}

	now := time.Now()
	return &JiraConnection{
		id:                 uuid.New().String(),
		projectID:          projectID,
		name:               name,
		jiraURL:            strings.TrimRight(jiraURL, "/"),
		authenticationType: authType,
		projectKey:         projectKey,
		username:           username,
		encryptedCredential: credential, // Will be encrypted when saved
		status:             ConnectionStatusPending,
		isActive:           false,
		createdAt:          now,
		updatedAt:          now,
	}, nil
}

// ID returns the connection ID
func (j *JiraConnection) ID() string {
	return j.id
}

// ProjectID returns the project ID
func (j *JiraConnection) ProjectID() string {
	return j.projectID
}

// Name returns the connection name
func (j *JiraConnection) Name() string {
	return j.name
}

// JiraURL returns the JIRA instance URL
func (j *JiraConnection) JiraURL() string {
	return j.jiraURL
}

// AuthenticationType returns the authentication type
func (j *JiraConnection) AuthenticationType() AuthenticationType {
	return j.authenticationType
}

// ProjectKey returns the JIRA project key
func (j *JiraConnection) ProjectKey() string {
	return j.projectKey
}

// Username returns the username
func (j *JiraConnection) Username() string {
	return j.username
}

// Status returns the connection status
func (j *JiraConnection) Status() ConnectionStatus {
	return j.status
}

// IsActive returns whether the connection is active
func (j *JiraConnection) IsActive() bool {
	return j.isActive
}

// GetEncryptedCredentialDirect returns the encrypted credential directly (for repository use only)
func (j *JiraConnection) GetEncryptedCredentialDirect() string {
	return j.encryptedCredential
}

// LastTestedAt returns when the connection was last tested
func (j *JiraConnection) LastTestedAt() *time.Time {
	return j.lastTestedAt
}

// CreatedAt returns when the connection was created
func (j *JiraConnection) CreatedAt() time.Time {
	return j.createdAt
}

// UpdatedAt returns when the connection was last updated
func (j *JiraConnection) UpdatedAt() time.Time {
	return j.updatedAt
}

// UpdateConnectionInfo updates the connection information
func (j *JiraConnection) UpdateConnectionInfo(name, jiraURL, projectKey string) error {
	if name == "" {
		return errors.New("connection name is required")
	}
	if !isValidJiraURL(jiraURL) {
		return errors.New("JIRA URL must start with http:// or https://")
	}
	if projectKey == "" {
		return errors.New("project key is required")
	}

	j.name = name
	j.jiraURL = strings.TrimRight(jiraURL, "/")
	j.projectKey = projectKey
	j.updatedAt = time.Now()
	return nil
}

// UpdateCredentials updates the authentication credentials
func (j *JiraConnection) UpdateCredentials(authType AuthenticationType, username, credential string) error {
	if username == "" {
		return errors.New("username is required")
	}
	if credential == "" {
		return errors.New("credential is required")
	}

	j.authenticationType = authType
	j.username = username
	j.encryptedCredential = credential // Will be encrypted when saved
	j.status = ConnectionStatusPending // Reset status when credentials change
	j.updatedAt = time.Now()
	return nil
}

// TestConnection tests the JIRA connection
func (j *JiraConnection) TestConnection(ctx context.Context, client JiraClient) error {
	log.Printf("[JiraConnection] Testing connection for ID: %s, URL: %s", j.id, j.jiraURL)
	
	err := client.TestConnection(ctx, j.jiraURL, j.username, j.encryptedCredential, j.authenticationType)
	now := time.Now()
	j.lastTestedAt = &now
	j.updatedAt = now

	if err != nil {
		j.status = ConnectionStatusFailed
		log.Printf("[JiraConnection] Test failed for %s: %v", j.jiraURL, err)
		return fmt.Errorf("connection test failed: %w", err)
	}

	j.status = ConnectionStatusConnected
	log.Printf("[JiraConnection] Test successful for %s, status updated to Connected", j.jiraURL)
	return nil
}

// Activate activates the connection
func (j *JiraConnection) Activate() {
	j.isActive = true
	j.updatedAt = time.Now()
}

// Deactivate deactivates the connection
func (j *JiraConnection) Deactivate() {
	j.isActive = false
	j.updatedAt = time.Now()
}

// GetEncryptedCredential returns the credential encrypted with the provided key
func (j *JiraConnection) GetEncryptedCredential(key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	plaintext := []byte(j.encryptedCredential)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredential decrypts a credential with the provided key
func DecryptCredential(encrypted string, key []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode credential: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// Snapshot returns a read-only snapshot of the connection
func (j *JiraConnection) Snapshot() JiraConnectionSnapshot {
	return JiraConnectionSnapshot{
		ID:                 j.id,
		ProjectID:          j.projectID,
		Name:               j.name,
		JiraURL:            j.jiraURL,
		AuthenticationType: j.authenticationType,
		ProjectKey:         j.projectKey,
		Username:           j.username,
		Status:             j.status,
		IsActive:           j.isActive,
		LastTestedAt:       j.lastTestedAt,
		CreatedAt:          j.createdAt,
		UpdatedAt:          j.updatedAt,
	}
}

// JiraConnectionSnapshot is a read-only view of a JIRA connection
type JiraConnectionSnapshot struct {
	ID                 string
	ProjectID          string
	Name               string
	JiraURL            string
	AuthenticationType AuthenticationType
	ProjectKey         string
	Username           string
	Status             ConnectionStatus
	IsActive           bool
	LastTestedAt       *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ReconstructJiraConnection reconstructs a JiraConnection from persisted data
func ReconstructJiraConnection(
	id, projectID, name, jiraURL string,
	authType AuthenticationType,
	projectKey, username, encryptedCredential string,
	status ConnectionStatus,
	isActive bool,
	lastTestedAt *time.Time,
	createdAt, updatedAt time.Time,
) *JiraConnection {
	return &JiraConnection{
		id:                  id,
		projectID:           projectID,
		name:                name,
		jiraURL:             jiraURL,
		authenticationType:  authType,
		projectKey:          projectKey,
		username:            username,
		encryptedCredential: encryptedCredential,
		status:              status,
		isActive:            isActive,
		lastTestedAt:        lastTestedAt,
		createdAt:           createdAt,
		updatedAt:           updatedAt,
	}
}

// Helper function to validate JIRA URL
func isValidJiraURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}