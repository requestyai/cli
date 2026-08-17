package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

const (
	jsonFlag = "json"
)

const (
	apiKeyNameFlag                  = "name"
	apiKeyMonthlyLimitFlag          = "monthly-limit"
	apiKeyManagePermissionFlag      = "manage-permission"
	apiKeyCompletionsPermissionFlag = "completions-permission"
	apiKeyYesFlag                   = "yes"
)

// neverExpires is the expiry to pass for a key that should keep working.
const neverExpires = "never"

func newAPIKeysCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "api-keys",
		Aliases: []string{"api-key", "keys"},
		Short:   "Manage the API keys in your organization",
		Long: "Manage the API keys in your organization.\n\n" +
			"The show command also takes " + client.SelfAPIKeyID +
			" as the id, meaning the key this CLI is configured with.",
	}

	cmd.PersistentFlags().Bool(jsonFlag, false, "print JSON instead of a table")
	cmd.AddCommand(
		newAPIKeysListCommand(env),
		newAPIKeysShowCommand(env),
		newAPIKeysCreateCommand(env),
		newAPIKeysSetCommand(env),
		newAPIKeysClearCommand(env),
		newAPIKeysDeleteCommand(env),
	)

	return cmd
}

func newAPIKeysListCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the API keys in your organization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			keys, err := env.client.APIKeys(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, keys)
			}
			if len(keys) == 0 {
				_, err := fmt.Fprintln(out, "No API keys yet.")
				return err
			}

			rows := make([][]string, 0, len(keys))
			for _, key := range keys {
				rows = append(rows, []string{
					key.ID,
					key.Name,
					formatMoney(key.MonthlySpend),
					formatLimit(key.MonthlyLimit),
					formatLabels(key.Labels),
				})
			}

			return writeTable(out, []string{"ID", "NAME", "SPEND", "LIMIT", "LABELS"}, rows)
		},
	}
}

func newAPIKeysShowCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one API key",
		Long: "Show one API key.\n\n" +
			"Pass " + client.SelfAPIKeyID + " to read the key this CLI is configured with, which\n" +
			"works even without manage permission.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			key, err := env.client.APIKey(cmd.Context(), id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, key)
			}

			return writeFields(out, [][2]string{
				{"ID", key.ID},
				{"Name", key.Name},
				{"Spend this month", formatMoney(key.MonthlySpend)},
				{"Monthly limit", formatLimit(key.MonthlyLimit)},
				{"Permissions", formatPermissions(key.Permissions)},
				{"Logging", strconv.FormatBool(key.Logging)},
				{"Group", formatGroup(key.Group)},
			})
		},
	}
}

func newAPIKeysCreateCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API key",
		Long: "Create an API key.\n\n" +
			"The key itself is returned once and cannot be read again afterwards.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()

			name, err := flags.GetString(apiKeyNameFlag)
			if err != nil {
				return err
			}

			input := client.CreateAPIKeyInput{Name: name}

			if flags.Changed(apiKeyMonthlyLimitFlag) {
				raw, err := flags.GetString(apiKeyMonthlyLimitFlag)
				if err != nil {
					return err
				}
				limit, err := parseMoney("--"+apiKeyMonthlyLimitFlag, raw)
				if err != nil {
					return err
				}
				input.MonthlyLimit = &limit
			}

			manage, err := flags.GetString(apiKeyManagePermissionFlag)
			if err != nil {
				return err
			}
			completions, err := flags.GetString(apiKeyCompletionsPermissionFlag)
			if err != nil {
				return err
			}
			permissions, err := parsePermissions(manage, completions)
			if err != nil {
				return err
			}
			input.Permissions = permissions

			created, err := env.client.CreateAPIKey(cmd.Context(), input)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, created)
			}

			if err := writeFields(out, [][2]string{
				{"ID", created.ID},
				{"Key", created.Secret},
			}); err != nil {
				return err
			}

			_, err = fmt.Fprintln(out, "\nStore this key now. It is never shown again.")
			return err
		},
	}

	flags := cmd.Flags()
	flags.String(apiKeyNameFlag, "", "name for the new key")
	flags.String(apiKeyMonthlyLimitFlag, "", "monthly spending cap in dollars, for example 100 (default: the organization setting)")
	flags.String(apiKeyManagePermissionFlag, "", "access to the management API: none, read or write")
	flags.String(apiKeyCompletionsPermissionFlag, "", "access to completions: none, read or write")
	if err := cmd.MarkFlagRequired(apiKeyNameFlag); err != nil {
		panic(err)
	}

	return cmd
}

func newAPIKeysSetCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set a property on an API key",
	}
	cmd.AddCommand(
		newAPIKeysSetLimitCommand(env),
		newAPIKeysSetLabelsCommand(env),
		newAPIKeysSetExpiryCommand(env),
	)

	return cmd
}

func newAPIKeysSetLimitCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "limit <id> <amount>",
		Short: "Set the monthly spending cap of an API key",
		Long: "Set the monthly spending cap of an API key.\n\n" +
			"The amount is in dollars, for example 100 or 49.99. Pass 0 to remove the cap.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			limit, err := parseMoney("amount", args[1])
			if err != nil {
				return err
			}

			if err := env.client.UpdateAPIKeyLimit(cmd.Context(), id, limit); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "monthly_limit",
				fmt.Sprintf("Monthly limit for %s is now %s.", id, formatLimit(limit)))
		},
	}
}

func newAPIKeysSetLabelsCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "labels <id> <key=value>...",
		Short: "Set the labels on an API key",
		Long: "Set the labels on an API key.\n\n" +
			"Labels are replaced wholesale: pass every label you want to keep.\n" +
			"Use clear labels to remove them all.",
		// A missing pair would otherwise wipe every label, so say what to run
		// instead rather than reporting an argument count.
		Args: func(_ *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return errors.New("missing api key id")
			case 1:
				return errors.New("no labels given: pass them as key=value, or run clear labels to remove them all")
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			labels, err := parseLabels(args[1:])
			if err != nil {
				return err
			}

			if err := env.client.UpdateAPIKeyLabels(cmd.Context(), id, labels); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "labels",
				fmt.Sprintf("Labels on %s are now %s.", id, formatLabels(labels)))
		},
	}
}

func newAPIKeysClearCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear a property on an API key",
	}
	cmd.AddCommand(newAPIKeysClearLabelsCommand(env))

	return cmd
}

func newAPIKeysClearLabelsCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "labels <id>",
		Short: "Remove every label from an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			if err := env.client.UpdateAPIKeyLabels(cmd.Context(), id, nil); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "labels", fmt.Sprintf("Removed every label from %s.", id))
		},
	}
}

func newAPIKeysSetExpiryCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "expiry <id> <time|" + neverExpires + ">",
		Short: "Set when an API key stops working",
		Long: "Set when an API key stops working.\n\n" +
			"The time is RFC3339, for example 2026-12-31T23:59:59Z. Pass " + neverExpires + " to make\n" +
			"the key non-expiring. A key that has already expired cannot be revived.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			var expiresAt *time.Time
			if !strings.EqualFold(strings.TrimSpace(args[1]), neverExpires) {
				parsed, err := parseTime(args[1])
				if err != nil {
					return err
				}
				expiresAt = &parsed
			}

			if err := env.client.UpdateAPIKeyExpiry(cmd.Context(), id, expiresAt); err != nil {
				return err
			}

			message := fmt.Sprintf("%s no longer expires.", id)
			if expiresAt != nil {
				message = fmt.Sprintf("%s expires at %s.", id, expiresAt.Format(time.RFC3339))
			}

			return reportUpdate(cmd, id, "expiry", message)
		},
	}
}

func newAPIKeysDeleteCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an API key",
		Long: "Delete an API key.\n\n" +
			"Deletion is permanent, and every request made with the key fails from then on.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			skipPrompt, err := cmd.Flags().GetBool(apiKeyYesFlag)
			if err != nil {
				return err
			}

			if !skipPrompt {
				confirmed, err := confirm(cmd, fmt.Sprintf("Delete API key %s? This cannot be undone [y/N]: ", id))
				if err != nil {
					return err
				}
				if !confirmed {
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "Left it alone.")
					return err
				}
			}

			if err := env.client.DeleteAPIKey(cmd.Context(), id); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, map[string]any{"id": id, "deleted": true})
			}

			_, err = fmt.Fprintf(out, "Deleted API key %s.\n", id)
			return err
		},
	}

	cmd.Flags().BoolP(apiKeyYesFlag, "y", false, "delete without asking first")

	return cmd
}

// reportUpdate acknowledges a change the API accepted but does not echo back.
func reportUpdate(cmd *cobra.Command, id, field, message string) error {
	out := cmd.OutOrStdout()
	if jsonOutput(cmd) {
		return printJSON(out, map[string]string{"id": id, "updated": field})
	}

	_, err := fmt.Fprintln(out, message)

	return err
}

// confirm asks the question and treats anything but yes as no.
func confirm(cmd *cobra.Command, question string) (bool, error) {
	if _, err := fmt.Fprint(cmd.OutOrStdout(), question); err != nil {
		return false, err
	}

	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// parseID keeps a blank identifier from being sent as a request for the whole
// collection.
func parseID(value string) (string, error) {
	id := strings.TrimSpace(value)
	if id == "" {
		return "", errors.New("missing api key id")
	}

	return id, nil
}

// parsePermissions builds the permission block, which the API only takes with
// both halves set, so asking for one means saying what the other is too.
func parsePermissions(manage, completions string) (*client.APIKeyPermissions, error) {
	if manage == "" && completions == "" {
		return nil, nil
	}
	if manage == "" || completions == "" {
		return nil, fmt.Errorf("set both --%s and --%s, or neither", apiKeyManagePermissionFlag, apiKeyCompletionsPermissionFlag)
	}

	parsedManage, err := parsePermission(apiKeyManagePermissionFlag, manage)
	if err != nil {
		return nil, err
	}
	parsedCompletions, err := parsePermission(apiKeyCompletionsPermissionFlag, completions)
	if err != nil {
		return nil, err
	}

	return &client.APIKeyPermissions{Manage: parsedManage, Completions: parsedCompletions}, nil
}

func parsePermission(flag, value string) (client.APIKeyPermission, error) {
	switch permission := client.APIKeyPermission(value); permission {
	case client.APIKeyPermissionNone, client.APIKeyPermissionRead, client.APIKeyPermissionWrite:
		return permission, nil
	default:
		return "", fmt.Errorf("invalid --%s %q: want none, read or write", flag, value)
	}
}

func parseLabels(pairs []string) (map[string]string, error) {
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

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid expiry %q: want %s or an RFC3339 time such as 2026-12-31T23:59:59Z",
			value, neverExpires)
	}

	return parsed, nil
}

// parseMoney reads an amount, named for whichever flag or argument it came from.
func parseMoney(name, value string) (decimal.Decimal, error) {
	amount, err := decimal.NewFromString(strings.TrimPrefix(strings.TrimSpace(value), "$"))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("invalid %s %q: want an amount such as 100 or 49.99", name, value)
	}
	if amount.IsNegative() {
		return decimal.Decimal{}, fmt.Errorf("invalid %s %q: want zero or more", name, value)
	}

	return amount, nil
}
