package util

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
)

const (
	managePermissionFlag      = "manage-permission"
	completionsPermissionFlag = "completions-permission"

	// NeverExpires is the expiry argument for a key that should keep working.
	NeverExpires = "never"
)

// ParseID trims an API key ID and rejects an empty value.
func ParseID(value string) (string, error) {
	id := strings.TrimSpace(value)
	if id == "" {
		return "", errors.New("missing api key id")
	}

	return id, nil
}

// ParsePermissions validates and combines the management and completions
// permissions. Both values must be provided together, or both left empty.
func ParsePermissions(manage, completions string) (*client.APIKeyPermissions, error) {
	if manage == "" && completions == "" {
		return nil, nil
	}
	if manage == "" || completions == "" {
		return nil, fmt.Errorf("set both --%s and --%s, or neither", managePermissionFlag, completionsPermissionFlag)
	}

	parsedManage, err := parsePermission(managePermissionFlag, manage)
	if err != nil {
		return nil, err
	}
	parsedCompletions, err := parsePermission(completionsPermissionFlag, completions)
	if err != nil {
		return nil, err
	}

	return &client.APIKeyPermissions{Manage: parsedManage, Completions: parsedCompletions}, nil
}

// parsePermission validates one API key permission and names its source flag
// in any returned error.
func parsePermission(flag, value string) (client.APIKeyPermission, error) {
	switch permission := client.APIKeyPermission(value); permission {
	case client.APIKeyPermissionNone, client.APIKeyPermissionRead, client.APIKeyPermissionWrite:
		return permission, nil
	default:
		return "", fmt.Errorf("invalid --%s %q: want none, read or write", flag, value)
	}
}

// ParseLabels converts key=value arguments into labels. Keys are trimmed, while
// values are preserved as entered.
func ParseLabels(pairs []string) (map[string]string, error) {
	labels := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("invalid label %q: want key=value", pair)
		}
		labels[key] = value
	}

	return labels, nil
}

// ParseTime parses an API key expiry in RFC3339 format.
func ParseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid expiry %q: want %s or an RFC3339 time such as 2026-12-31T23:59:59Z",
			value, NeverExpires)
	}

	return parsed, nil
}

// ParseMoney parses a non-negative decimal amount. It accepts surrounding
// whitespace and an optional dollar sign; name identifies the value in errors.
func ParseMoney(name, value string) (decimal.Decimal, error) {
	amount, err := decimal.NewFromString(strings.TrimPrefix(strings.TrimSpace(value), "$"))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("invalid %s %q: want an amount such as 100 or 49.99", name, value)
	}
	if amount.IsNegative() {
		return decimal.Decimal{}, fmt.Errorf("invalid %s %q: want zero or more", name, value)
	}

	return amount, nil
}
