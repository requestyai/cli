package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Model is one entry of a model list. Managed policies are listed in the
// same shape, so they are Models too.
type Model struct {
	ID string `json:"id"`

	ContextWindow   int `json:"context_window"`
	MaxOutputTokens int `json:"max_output_tokens"`

	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`

	CacheWritePrice float64 `json:"caching_price"`
	CacheReadPrice  float64 `json:"cached_price"`

	DataRetention bool `json:"data_retention"`
}

func (c *Client) Models(ctx context.Context) ([]Model, error) {
	return c.listModels(ctx, "/v1/models")
}

func (c *Client) ManagedPolicies(ctx context.Context) ([]Model, error) {
	return c.listModels(ctx, "/v1/models/managed")
}

func (c *Client) listModels(ctx context.Context, path string) ([]Model, error) {
	endpoint := c.config.RouterBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	type Response struct {
		Data []Model `json:"data"`
	}
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Data, nil
}
