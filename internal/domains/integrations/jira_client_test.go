package integrations_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
)

type jiraAPIField struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Custom bool              `json:"custom"`
	Schema jiraAPIFieldSchema `json:"schema"`
}

type jiraAPIFieldSchema struct {
	Type  string `json:"type"`
	Items string `json:"items,omitempty"`
}

func TestDefaultJiraClient_ListFields(t *testing.T) {
	ctx := context.Background()
	client := integrations.NewDefaultJiraClient()

	t.Run("includes custom fields and sorts by name ascending", func(t *testing.T) {
		fields := []jiraAPIField{
			{ID: "summary", Name: "Summary", Custom: false, Schema: jiraAPIFieldSchema{Type: "string"}},
			{ID: "labels", Name: "Labels", Custom: false, Schema: jiraAPIFieldSchema{Type: "array", Items: "string"}},
			{ID: "customfield_10016", Name: "Story Points", Custom: true, Schema: jiraAPIFieldSchema{Type: "number"}},
			{ID: "description", Name: "Description", Custom: false, Schema: jiraAPIFieldSchema{Type: "string"}},
			{ID: "customfield_10001", Name: "Epic Link", Custom: true, Schema: jiraAPIFieldSchema{Type: "string"}},
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/2/field" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(fields)
		}))
		defer srv.Close()

		result, err := client.ListFields(ctx, srv.URL, "user@example.com", "token", integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 5 {
			t.Fatalf("expected 5 fields (including custom), got %d", len(result))
		}
		// Sorted: Description, Epic Link, Labels, Story Points, Summary
		names := []string{result[0].Name, result[1].Name, result[2].Name, result[3].Name, result[4].Name}
		expected := []string{"Description", "Epic Link", "Labels", "Story Points", "Summary"}
		for i, n := range expected {
			if names[i] != n {
				t.Errorf("position %d: expected %q got %q", i, n, names[i])
			}
		}
	})

	t.Run("sets MultiValue true for array-type fields only", func(t *testing.T) {
		fields := []jiraAPIField{
			{ID: "summary", Name: "Summary", Custom: false, Schema: jiraAPIFieldSchema{Type: "string"}},
			{ID: "labels", Name: "Labels", Custom: false, Schema: jiraAPIFieldSchema{Type: "array", Items: "string"}},
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(fields)
		}))
		defer srv.Close()

		result, err := client.ListFields(ctx, srv.URL, "user@example.com", "token", integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		byName := map[string]integrations.JiraField{}
		for _, f := range result {
			byName[f.Name] = f
		}
		if !byName["Labels"].MultiValue {
			t.Error("expected Labels.MultiValue = true")
		}
		if summaryField, ok := byName["Summary"]; !ok {
			t.Error("expected Summary to be present in result")
		} else if summaryField.MultiValue {
			t.Error("expected Summary.MultiValue = false")
		}
	})

	t.Run("sends Authorization: Basic header for AuthTypeAPIToken", func(t *testing.T) {
		var capturedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode([]jiraAPIField{})
		}))
		defer srv.Close()

		username, token := "user@example.com", "api-secret-token"
		_, err := client.ListFields(ctx, srv.URL, username, token, integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, token)))
		if capturedAuth != expected {
			t.Errorf("expected auth header %q, got %q", expected, capturedAuth)
		}
	})

	t.Run("sends Authorization: Bearer header for AuthTypePersonalAccessToken", func(t *testing.T) {
		var capturedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode([]jiraAPIField{})
		}))
		defer srv.Close()

		pat := "my-personal-access-token"
		_, err := client.ListFields(ctx, srv.URL, "ignored", pat, integrations.AuthTypePersonalAccessToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedAuth != "Bearer "+pat {
			t.Errorf("expected Bearer header, got %q", capturedAuth)
		}
	})

	t.Run("returns error on HTTP 401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := client.ListFields(ctx, srv.URL, "user@example.com", "bad", integrations.AuthTypeAPIToken)
		if err == nil {
			t.Error("expected error for HTTP 401, got nil")
		}
	})

	t.Run("returns error on HTTP 500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := client.ListFields(ctx, srv.URL, "user@example.com", "token", integrations.AuthTypeAPIToken)
		if err == nil {
			t.Error("expected error for HTTP 500, got nil")
		}
	})
}
