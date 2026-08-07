package client

import (
	"context"
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
