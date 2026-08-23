package util

import (
	"testing"
	"time"

	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseID(t *testing.T) {
	id, err := ParseID("  key-123  ")
	require.NoError(t, err)
	assert.Equal(t, "key-123", id)

	_, err = ParseID(" \t ")
	assert.EqualError(t, err, "missing api key id")
}

func TestParsePermissions(t *testing.T) {
	permissions, err := ParsePermissions("", "")
	require.NoError(t, err)
	assert.Nil(t, permissions)

	permissions, err = ParsePermissions("read", "write")
	require.NoError(t, err)
	assert.Equal(t, &client.APIKeyPermissions{
		Manage:      client.APIKeyPermissionRead,
		Completions: client.APIKeyPermissionWrite,
	}, permissions)

	_, err = ParsePermissions("read", "")
	assert.EqualError(t, err, "set both --manage-permission and --completions-permission, or neither")

	_, err = ParsePermissions("invalid", "read")
	assert.EqualError(t, err, `invalid --manage-permission "invalid": want none, read or write`)
}

func TestParseLabels(t *testing.T) {
	labels, err := ParseLabels([]string{" env =production", "endpoint=https://example.com?a=b"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"env":      "production",
		"endpoint": "https://example.com?a=b",
	}, labels)

	_, err = ParseLabels([]string{"missing-value-separator"})
	assert.EqualError(t, err, `invalid label "missing-value-separator": want key=value`)
}

func TestParseTime(t *testing.T) {
	parsed, err := ParseTime("2026-12-31T23:59:59Z")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), parsed)

	_, err = ParseTime("tomorrow")
	assert.EqualError(t, err,
		`invalid expiry "tomorrow": want never or an RFC3339 time such as 2026-12-31T23:59:59Z`)
}

func TestParseMoney(t *testing.T) {
	for _, value := range []string{"49.99", " $49.99 "} {
		amount, err := ParseMoney("amount", value)
		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("49.99").Equal(amount))
	}

	_, err := ParseMoney("amount", "free")
	assert.EqualError(t, err, `invalid amount "free": want an amount such as 100 or 49.99`)

	_, err = ParseMoney("amount", "-1")
	assert.EqualError(t, err, `invalid amount "-1": want zero or more`)
}
