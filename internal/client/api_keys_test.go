package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/requestyai/cli/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder captures what the client sent, so a test can assert on the request
// and choose the reply in one place.
type recorder struct {
	method string
	path   string
	auth   string
	query  map[string][]string
	body   string
}

func newTestClient(t *testing.T, status int, reply string) (*Client, *recorder) {
	t.Helper()

	seen := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		seen.method = r.Method
		seen.path = r.URL.EscapedPath()
		seen.auth = r.Header.Get("Authorization")
		seen.query = r.URL.Query()
		seen.body = string(body)

		w.WriteHeader(status)
		if reply != "" {
			_, err := io.WriteString(w, reply)
			require.NoError(t, err)
		}
	}))
	t.Cleanup(server.Close)

	return New(config.Config{APIBaseURL: server.URL, APIKey: "test-key"}), seen
}

func TestClientAPIKeys(t *testing.T) {
	reply := `{"keys":[{"id":"key-1","name":"production","monthly_limit":"500","monthly_spend":"12.5",` +
		`"permissions":{"manage":"read","completions":"write"},"labels":{"env":"prod"},` +
		`"created_by":{"id":"user-1","email":"you@example.com"},"group":{"id":"group-1"}}]}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	keys, err := client.APIKeys(context.Background())

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/apikey", seen.path)
	assert.Equal(t, "Bearer test-key", seen.auth)
	assert.Equal(t, []APIKey{{
		ID:           "key-1",
		Name:         "production",
		MonthlyLimit: decimal.RequireFromString("500"),
		MonthlySpend: decimal.RequireFromString("12.5"),
		Permissions:  APIKeyPermissions{Manage: APIKeyPermissionRead, Completions: APIKeyPermissionWrite},
		Labels:       map[string]string{"env": "prod"},
		CreatedBy:    &APIKeyUser{ID: "user-1", Email: "you@example.com"},
		Group:        &APIKeyGroup{ID: "group-1"},
	}}, keys)
}

func TestClientAPIKey(t *testing.T) {
	reply := `{"id":"key-1","name":"production","logging":true,"monthly_limit":"0","monthly_spend":"3",` +
		`"permissions":{"manage":"none","completions":"write"},"group":{"id":"group-1"}}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	key, err := client.APIKey(context.Background(), SelfAPIKeyID)

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/apikey/self", seen.path)
	assert.Equal(t, APIKeyDetails{
		ID:           "key-1",
		Name:         "production",
		Logging:      true,
		MonthlyLimit: decimal.RequireFromString("0"),
		MonthlySpend: decimal.RequireFromString("3"),
		Permissions:  APIKeyPermissions{Manage: APIKeyPermissionNone, Completions: APIKeyPermissionWrite},
		Group:        &APIKeyGroup{ID: "group-1"},
	}, key)
}

func TestClientAPIKeyEscapesID(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"id":"key-1"}`)

	_, err := client.APIKey(context.Background(), "../org")

	require.NoError(t, err)
	assert.Equal(t, "/v1/manage/apikey/..%2Forg", seen.path)
}

func TestClientCreateAPIKey(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"api_key_id":"key-1","api_key":"rqsty-secret"}`)

	limit := decimal.RequireFromString("100")
	created, err := client.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		Name:         "production",
		MonthlyLimit: &limit,
		Permissions:  &APIKeyPermissions{Manage: APIKeyPermissionRead, Completions: APIKeyPermissionWrite},
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.method)
	assert.Equal(t, "/v1/manage/apikey", seen.path)
	assert.JSONEq(t, `{"name":"production","monthly_limit":"100","permissions":{"manage":"read","completions":"write"}}`, seen.body)
	assert.Equal(t, CreatedAPIKey{ID: "key-1", Secret: "rqsty-secret"}, created)
}

func TestClientCreateAPIKeyOmitsUnsetFields(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"api_key_id":"key-1","api_key":"rqsty-secret"}`)

	_, err := client.CreateAPIKey(context.Background(), CreateAPIKeyInput{Name: "production"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"production"}`, seen.body)
}

func TestClientUpdateAPIKeyLimit(t *testing.T) {
	client, seen := newTestClient(t, http.StatusNoContent, "")

	err := client.UpdateAPIKeyLimit(context.Background(), "key-1", decimal.RequireFromString("49.99"))

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.method)
	assert.Equal(t, "/v1/manage/apikey/key-1/limit", seen.path)
	assert.JSONEq(t, `{"monthly_limit":"49.99"}`, seen.body)
}

func TestClientUpdateAPIKeyLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		wantBody string
	}{
		{name: "set", labels: map[string]string{"env": "prod"}, wantBody: `{"labels":{"env":"prod"}}`},
		{name: "clear with empty map", labels: map[string]string{}, wantBody: `{"labels":{}}`},
		{name: "clear with nil map", labels: nil, wantBody: `{"labels":{}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, seen := newTestClient(t, http.StatusNoContent, "")

			err := client.UpdateAPIKeyLabels(context.Background(), "key-1", tc.labels)

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, seen.method)
			assert.Equal(t, "/v1/manage/apikey/key-1/label", seen.path)
			assert.JSONEq(t, tc.wantBody, seen.body)
		})
	}
}

func TestClientUpdateAPIKeyExpiry(t *testing.T) {
	expiresAt := time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt *time.Time
		wantBody  string
	}{
		{name: "set", expiresAt: &expiresAt, wantBody: `{"expires_at":"2026-12-31T23:59:59Z"}`},
		{name: "clear", expiresAt: nil, wantBody: `{"expires_at":null}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, seen := newTestClient(t, http.StatusNoContent, "")

			err := client.UpdateAPIKeyExpiry(context.Background(), "key-1", tc.expiresAt)

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, seen.method)
			assert.Equal(t, "/v1/manage/apikey/key-1/expiry", seen.path)
			assert.JSONEq(t, tc.wantBody, seen.body)
		})
	}
}

func TestClientDeleteAPIKey(t *testing.T) {
	client, seen := newTestClient(t, http.StatusNoContent, "")

	err := client.DeleteAPIKey(context.Background(), "key-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, seen.method)
	assert.Equal(t, "/v1/manage/apikey/key-1", seen.path)
	assert.Empty(t, seen.body)
}

func TestClientAPIKeyReportsAPIError(t *testing.T) {
	client, _ := newTestClient(t, http.StatusForbidden, `{"error":{"origin":"router","message":"manage read permission required"}}`)

	_, err := client.APIKey(context.Background(), "key-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "manage read permission required")
	assert.Contains(t, err.Error(), "403")
}

func TestClientAPIKeyFallsBackToStatus(t *testing.T) {
	client, _ := newTestClient(t, http.StatusBadGateway, "upstream is down")

	_, err := client.APIKey(context.Background(), "key-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestClientAPIKeysUnauthenticated(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `{"keys":[]}`)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := New(config.Config{APIBaseURL: server.URL})
	keys, err := client.APIKeys(context.Background())

	require.NoError(t, err)
	assert.Empty(t, keys)
	assert.Empty(t, auth)
}

func TestClientCreateAPIKeySendsJSONContentType(t *testing.T) {
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		require.NoError(t, json.NewEncoder(w).Encode(CreatedAPIKey{ID: "key-1", Secret: "rqsty-secret"}))
	}))
	defer server.Close()

	client := New(config.Config{APIBaseURL: server.URL, APIKey: "test-key"})
	_, err := client.CreateAPIKey(context.Background(), CreateAPIKeyInput{Name: "production"})

	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
}
