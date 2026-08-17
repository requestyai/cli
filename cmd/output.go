package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

// jsonOutput reports whether the caller asked for machine-readable output. The
// flag is declared once on the parent command and inherited from there.
func jsonOutput(cmd *cobra.Command) bool {
	value, err := cmd.Flags().GetBool(jsonFlag)
	return err == nil && value
}

// printJSON writes v as indented JSON, which is the contract scripts rely on.
func printJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(v)
}

// writeTable prints rows under headers, each column as wide as its widest cell.
func writeTable(w io.Writer, headers []string, rows [][]string) error {
	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for _, row := range rows {
		if _, err := fmt.Fprintln(writer, strings.Join(row, "\t")); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// writeFields prints label and value pairs, one per line, for a single record.
func writeFields(w io.Writer, fields [][2]string) error {
	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	for _, field := range fields {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", field[0], field[1]); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// formatMoney renders an amount in dollars and cents.
func formatMoney(amount decimal.Decimal) string {
	return "$" + amount.StringFixed(2)
}

// formatLimit renders a spending cap, where zero means there is none.
func formatLimit(limit decimal.Decimal) string {
	if limit.IsZero() {
		return "unlimited"
	}

	return formatMoney(limit)
}

// formatLabels renders labels as sorted key=value pairs.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}

	pairs := make([]string, 0, len(labels))
	for key, value := range labels {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)

	return strings.Join(pairs, " ")
}

// formatPermissions renders what a key is allowed to do.
func formatPermissions(permissions client.APIKeyPermissions) string {
	return fmt.Sprintf("manage=%s completions=%s", permissions.Manage, permissions.Completions)
}

// formatGroup renders the group a key belongs to, if it belongs to one.
func formatGroup(group *client.APIKeyGroup) string {
	if group == nil || group.ID == "" {
		return "-"
	}

	return group.ID
}
