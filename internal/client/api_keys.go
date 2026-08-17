package client

import (
	"context"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// SelfAPIKeyID stands in for the key the request is made with, so a key with
// no manage permission can still read its own record.
const SelfAPIKeyID = "self"

// APIKeyPermission is the access a key has to one part of the API.
type APIKeyPermission string

const (
	APIKeyPermissionNone  APIKeyPermission = "none"
	APIKeyPermissionRead  APIKeyPermission = "read"
	APIKeyPermissionWrite APIKeyPermission = "write"
)

// APIKeyPermissions is what a key is allowed to do.
type APIKeyPermissions struct {
	Manage      APIKeyPermission `json:"manage"`
	Completions APIKeyPermission `json:"completions"`
}

// APIKeyUser identifies the person a key belongs to.
type APIKeyUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// APIKeyGroup identifies the group a key belongs to.
type APIKeyGroup struct {
	ID string `json:"id"`
}

// APIKey is a key as it appears in the organization listing. A MonthlyLimit of
// zero means the key is not capped.
type APIKey struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	MonthlyLimit decimal.Decimal   `json:"monthly_limit"`
	MonthlySpend decimal.Decimal   `json:"monthly_spend"`
	Permissions  APIKeyPermissions `json:"permissions"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedBy    *APIKeyUser       `json:"created_by,omitempty"`
	Group        *APIKeyGroup      `json:"group,omitempty"`
}

// APIKeyDetails is a single key. The listing does not report logging, so this
// is a wider record than APIKey rather than the same one.
type APIKeyDetails struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Logging      bool              `json:"logging"`
	MonthlyLimit decimal.Decimal   `json:"monthly_limit"`
	MonthlySpend decimal.Decimal   `json:"monthly_spend"`
	Permissions  APIKeyPermissions `json:"permissions"`
	Group        *APIKeyGroup      `json:"group,omitempty"`
}

// CreateAPIKeyInput describes a key to create. A nil MonthlyLimit or
// Permissions leaves the organization default in place.
type CreateAPIKeyInput struct {
	Name         string
	MonthlyLimit *decimal.Decimal
	Permissions  *APIKeyPermissions
}

// CreatedAPIKey carries the secret the API returns once and never again.
type CreatedAPIKey struct {
	ID     string `json:"api_key_id"`
	Secret string `json:"api_key"`
}

// APIKeys lists the organization's keys.
func (c *Client) APIKeys(ctx context.Context) ([]APIKey, error) {
	endpoint, err := c.manageURL("apikey")
	if err != nil {
		return nil, err
	}

	var response struct {
		Keys []APIKey `json:"keys"`
	}
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response.Keys, nil
}

// APIKey returns one key. Pass SelfAPIKeyID for the calling key.
func (c *Client) APIKey(ctx context.Context, id string) (APIKeyDetails, error) {
	endpoint, err := c.manageURL("apikey", id)
	if err != nil {
		return APIKeyDetails{}, err
	}

	var details APIKeyDetails
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &details); err != nil {
		return APIKeyDetails{}, err
	}

	return details, nil
}

// CreateAPIKey issues a new key for the organization.
func (c *Client) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (CreatedAPIKey, error) {
	endpoint, err := c.manageURL("apikey")
	if err != nil {
		return CreatedAPIKey{}, err
	}

	body := struct {
		Name         string             `json:"name"`
		MonthlyLimit *decimal.Decimal   `json:"monthly_limit,omitempty"`
		Permissions  *APIKeyPermissions `json:"permissions,omitempty"`
	}{
		Name:         input.Name,
		MonthlyLimit: input.MonthlyLimit,
		Permissions:  input.Permissions,
	}

	var created CreatedAPIKey
	if err := c.do(ctx, http.MethodPost, endpoint, body, &created); err != nil {
		return CreatedAPIKey{}, err
	}

	return created, nil
}

// UpdateAPIKeyLimit sets the monthly spending cap. Zero removes the cap.
func (c *Client) UpdateAPIKeyLimit(ctx context.Context, id string, monthlyLimit decimal.Decimal) error {
	endpoint, err := c.manageURL("apikey", id, "limit")
	if err != nil {
		return err
	}

	body := struct {
		MonthlyLimit decimal.Decimal `json:"monthly_limit"`
	}{MonthlyLimit: monthlyLimit}

	return c.do(ctx, http.MethodPost, endpoint, body, nil)
}

// UpdateAPIKeyLabels replaces every label on the key. An empty map clears them.
func (c *Client) UpdateAPIKeyLabels(ctx context.Context, id string, labels map[string]string) error {
	endpoint, err := c.manageURL("apikey", id, "label")
	if err != nil {
		return err
	}

	if labels == nil {
		labels = map[string]string{}
	}

	body := struct {
		Labels map[string]string `json:"labels"`
	}{Labels: labels}

	return c.do(ctx, http.MethodPost, endpoint, body, nil)
}

// UpdateAPIKeyExpiry sets when the key stops working. A nil expiresAt makes the
// key non-expiring. A key that has already expired cannot be revived.
func (c *Client) UpdateAPIKeyExpiry(ctx context.Context, id string, expiresAt *time.Time) error {
	endpoint, err := c.manageURL("apikey", id, "expiry")
	if err != nil {
		return err
	}

	body := struct {
		ExpiresAt *time.Time `json:"expires_at"`
	}{ExpiresAt: expiresAt}

	return c.do(ctx, http.MethodPost, endpoint, body, nil)
}

// DeleteAPIKey removes a key for good. Requests using it fail immediately.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	endpoint, err := c.manageURL("apikey", id)
	if err != nil {
		return err
	}

	return c.do(ctx, http.MethodDelete, endpoint, nil, nil)
}
