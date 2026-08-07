package client

import (
	"net/http"
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
