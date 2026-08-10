package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCheckAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{name: "recognised key", statusCode: http.StatusOK},
		{name: "unsigned key", statusCode: http.StatusUnauthorized, wantErr: ErrInvalidAPIKey},
		{name: "forbidden key", statusCode: http.StatusForbidden, wantErr: ErrInvalidAPIKey},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/auth/check", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client := New(config.Config{APIBaseURL: server.URL, APIKey: "test-key"})
			err := client.CheckAPIKey(context.Background())

			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestClientCheckAPIKeyUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(config.Config{APIBaseURL: server.URL, APIKey: "test-key"})
	err := client.CheckAPIKey(context.Background())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidAPIKey)
}
