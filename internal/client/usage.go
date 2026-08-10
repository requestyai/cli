package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"
)

type UsageEntry struct {
	CompletionsRequests   int             `json:"completions_requests"`
	EmbeddingRequests     int             `json:"embedding_requests"`
	ImageRequests         int             `json:"image_requests"`
	SpeechRequests        int             `json:"speech_requests"`
	TranscriptionRequests int             `json:"transcription_requests"`
	TotalRequests         int             `json:"total_requests"`
	Spend                 decimal.Decimal `json:"spend"`
	InputTokens           int             `json:"input_tokens"`
	OutputTokens          int             `json:"output_tokens"`
	TotalTokens           int             `json:"total_tokens"`
	GroupedData           []UsageGrouped  `json:"grouped_data,omitempty"`
}

type UsageGrouped struct {
	GroupByValues         map[string]any  `json:"group_by_values"`
	CompletionsRequests   int             `json:"completions_requests"`
	EmbeddingRequests     int             `json:"embedding_requests"`
	ImageRequests         int             `json:"image_requests"`
	SpeechRequests        int             `json:"speech_requests"`
	TranscriptionRequests int             `json:"transcription_requests"`
	TotalRequests         int             `json:"total_requests"`
	Spend                 decimal.Decimal `json:"spend"`
	InputTokens           int             `json:"input_tokens"`
	OutputTokens          int             `json:"output_tokens"`
	TotalTokens           int             `json:"total_tokens"`
}

type UsageInput struct {
	Start      time.Time
	End        time.Time
	Resolution string
	GroupBy    string
}

// Usage returns the organization's usage for the requested period.
// If End or Resolution is unset, the API's defaults are used.
func (c *Client) Usage(ctx context.Context, input UsageInput) (map[string]UsageEntry, error) {
	apiBaseURL, err := c.apiBaseURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get api base url: %w", err)
	}

	endpoint, err := url.Parse(fmt.Sprintf("%s/v1/manage/apikey/self/usage", apiBaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	query := endpoint.Query()
	query.Set("start", input.Start.Format(time.RFC3339))
	if !input.End.IsZero() {
		query.Set("end", input.End.Format(time.RFC3339))
	}
	if input.Resolution != "" {
		query.Set("resolution", input.Resolution)
	}
	if input.GroupBy != "" {
		query.Set("group_by", input.GroupBy)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code not ok: %d", resp.StatusCode)
	}

	var response struct {
		Usage map[string]UsageEntry `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Usage, nil
}
