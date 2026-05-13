package integrations

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// ListJiraFields retrieves all standard (non-custom) JIRA fields for the given
// connection. The credential is decrypted before being forwarded to the JIRA
// client; if decryption fails an error is logged and returned.
func (s *JiraConnectionService) ListJiraFields(ctx context.Context, connectionID string) ([]JiraField, error) {
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find connection: %w", err)
	}
	if conn == nil {
		return nil, fmt.Errorf("JIRA connection %s not found", connectionID)
	}

	decrypted, decErr := DecryptCredential(conn.encryptedCredential, s.encryptionKey)
	if decErr != nil {
		logrus.WithError(decErr).WithField("connection_id", connectionID).
			Error("failed to decrypt JIRA credential for ListJiraFields")
		return nil, fmt.Errorf("failed to decrypt JIRA credential: %w", decErr)
	}

	fields, err := s.jiraClient.ListFields(ctx, conn.jiraURL, conn.username, decrypted, conn.authenticationType)
	if err != nil {
		return nil, fmt.Errorf("failed to list JIRA fields: %w", err)
	}

	return fields, nil
}


