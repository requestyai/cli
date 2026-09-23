package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Models(t *testing.T) {
	client := New(config.Config{RouterBaseURL: config.DefaultRouterBaseURL})

	models, err := client.Models(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(models), 100)
}

func TestClient_ManagedPolicies(t *testing.T) {
	var seen struct{ path, auth string }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.auth = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `{"object":"list","data":[
			{"id":"claude-sonnet-4-6","context_window":1000000,"max_output_tokens":128000,"input_price":0.000003,"output_price":0.000015},
			{"id":"gpt-5.5","context_window":400000,"input_price":0.0000055,"output_price":0.000033}
		]}`)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := New(config.Config{RouterBaseURL: server.URL, APIKey: "test-key"})

	policies, err := client.ManagedPolicies(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "/v1/models/managed", seen.path)
	assert.Equal(t, "Bearer test-key", seen.auth)
	assert.Equal(t, []Model{
		{ID: "claude-sonnet-4-6", ContextWindow: 1_000_000, MaxOutputTokens: 128_000, InputPrice: 0.000003, OutputPrice: 0.000015},
		{ID: "gpt-5.5", ContextWindow: 400_000, InputPrice: 0.0000055, OutputPrice: 0.000033},
	}, policies)
}

func TestClient_ManagedPoliciesRejectsErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := New(config.Config{RouterBaseURL: server.URL, APIKey: "bad"}).ManagedPolicies(context.Background())

	require.EqualError(t, err, "status code not ok: 401")
}
