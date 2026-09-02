package util

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
)

const (
	managePermissionFlag      = "manage-permission"
	completionsPermissionFlag = "completions-permission"
	roleFlag                  = "role"

	// NeverExpires is the expiry argument for a key that should keep working.
	NeverExpires = "never"
)

// ParseID trims an API key ID and rejects an empty value.
func ParseID(value string) (string, error) {
	return parseID("api key", value)
}

// ParseGroupID trims a group ID and rejects an empty value.
func ParseGroupID(value string) (string, error) {
	return parseID("group", value)
}

// ParseUserID trims a user ID and rejects an empty value.
func ParseUserID(value string) (string, error) {
	return parseID("user", value)
}

// ParseAccessListID trims an access list ID and rejects an empty value.
func ParseAccessListID(value string) (string, error) {
	return parseID("access list", value)
}

// parseID trims an identifier and names what was missing when it is empty.
func parseID(subject, value string) (string, error) {
	id := strings.TrimSpace(value)
	if id == "" {
		return "", fmt.Errorf("missing %s id", subject)
	}

	return id, nil
}

// ParseRole validates a group member role.
func ParseRole(value string) (client.GroupRole, error) {
	switch role := client.GroupRole(strings.TrimSpace(value)); role {
	case client.GroupRoleAdmin, client.GroupRoleMember:
		return role, nil
	default:
		return "", fmt.Errorf("invalid --%s %q: want admin or member", roleFlag, value)
	}
}

// ParseModality validates the kind of model an access list entry is for.
func ParseModality(value string) (client.Modality, error) {
	modality := client.Modality(strings.TrimSpace(value))
	if slices.Contains(client.Modalities, modality) {
		return modality, nil
	}

	return "", fmt.Errorf("invalid modality %q: want %s", value, modalityChoices())
}

// modalityChoices lists the modalities the way an error message reads them out.
func modalityChoices() string {
	names := make([]string, 0, len(client.Modalities))
	for _, modality := range client.Modalities {
		names = append(names, string(modality))
	}

	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// ParseModels trims model identifiers and rejects any that are blank, which
// would otherwise slip into an access list as an entry nothing can match.
func ParseModels(values []string) ([]string, error) {
	models := make([]string, 0, len(values))
	for _, value := range values {
		model := strings.TrimSpace(value)
		if model == "" {
			return nil, fmt.Errorf("invalid model %q: want a model id such as openai/gpt-4o", value)
		}
		models = append(models, model)
	}

	return models, nil
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
