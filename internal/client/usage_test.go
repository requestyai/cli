package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/requestyai/cli/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientUsage(t *testing.T) {
	start := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.August, 7, 12, 30, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/manage/apikey/self/usage", r.URL.Path)
		assert.Equal(t, start.Format(time.RFC3339), r.URL.Query().Get("start"))
		assert.Equal(t, end.Format(time.RFC3339), r.URL.Query().Get("end"))
		assert.Equal(t, "day", r.URL.Query().Get("resolution"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_, err := fmt.Fprint(w, `{"usage":{"2026-08-07T00:00:00Z":{"total_requests":7,"total_tokens":42,"spend":"1.25"}}}`)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := New(config.Config{APIBaseURL: server.URL, APIKey: "test-key"})
	usage, err := client.Usage(context.Background(), UsageInput{
		Start:      start,
		End:        end,
		Resolution: "day",
	})

	require.NoError(t, err)
	assert.Equal(t, UsageEntry{
		TotalRequests: 7,
		TotalTokens:   42,
		Spend:         decimal.RequireFromString("1.25"),
	}, usage["2026-08-07T00:00:00Z"])
}
