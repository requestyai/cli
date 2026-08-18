package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/requestyai/cli/internal/config"
)

type Client struct {
	config     config.Config
	httpClient *http.Client
}

// New builds a client that authenticates with config. An empty API key
// means requests go out unauthenticated.
func New(cfg config.Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) apiBaseURL() (string, error) {
	if c.config.APIBaseURL != "" {
		return c.config.APIBaseURL, nil
	}

	return strings.Replace(c.config.RouterBaseURL, "router", "api-v2", 1), nil
}

func (c *Client) authorize(req *http.Request) {
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
}

// manageURL builds a management API address. Each element is escaped, so an
// identifier that came from a user cannot reach into another path.
func (c *Client) manageURL(elements ...string) (string, error) {
	apiBaseURL, err := c.apiBaseURL()
	if err != nil {
		return "", fmt.Errorf("failed to get api base url: %w", err)
	}

	escaped := make([]string, 0, len(elements))
	for _, element := range elements {
		escaped = append(escaped, url.PathEscape(element))
	}

	endpoint := fmt.Sprintf("%s/v1/manage/%s", apiBaseURL, strings.Join(escaped, "/"))
	return endpoint, nil
}

// do sends an authenticated request, encoding body as JSON when it is not nil
// and decoding the reply into out when out is not nil.
func (c *Client) do(ctx context.Context, method string, endpoint string, body, out any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return statusError(resp)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// statusError describes a rejected request, preferring the explanation the API
// sent over the bare status code.
func statusError(resp *http.Response) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope); err == nil && envelope.Error.Message != "" {
		return fmt.Errorf("%s (status %d)", envelope.Error.Message, resp.StatusCode)
	}

	return fmt.Errorf("status code not ok: %d", resp.StatusCode)
}
