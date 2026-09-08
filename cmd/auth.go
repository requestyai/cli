package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCommand(env environment) *cobra.Command {
	auth := &cobra.Command{
		Use:    "auth",
		Short:  "Credential helpers for integrations",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	auth.AddCommand(&cobra.Command{
		Use:   "token",
		Short: "Print the configured Requesty API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if env.config.APIKey == "" {
				return fmt.Errorf("no Requesty API key configured")
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), env.config.APIKey); err != nil {
				return fmt.Errorf("failed to print API key: %w", err)
			}
			return nil
		},
	})

	return auth
}
