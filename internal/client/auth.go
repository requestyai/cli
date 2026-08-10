package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrInvalidAPIKey reports that the gateway does not recognise the key.
var ErrInvalidAPIKey = errors.New("invalid api key")

// CheckAPIKey asks the gateway whether the configured key is a live one of its
// own, rejecting a key that was never issued, was deleted, or has expired.
func (c *Client) CheckAPIKey(ctx context.Context) error {
	apiBaseURL, err := c.apiBaseURL()
	if err != nil {
		return fmt.Errorf("failed to get api base url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/v1/auth/check", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrInvalidAPIKey
	default:
		return fmt.Errorf("status code not ok: %d", resp.StatusCode)
	}
}
