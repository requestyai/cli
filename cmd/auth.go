package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCommand(env *environment) *cobra.Command {
	auth := &cobra.Command{
		Use:               "auth",
		Short:             "Credential helpers for integrations",
		Args:              cobra.NoArgs,
		Hidden:            true,
		PersistentPreRunE: env.requireProfile,
	}

	auth.AddCommand(&cobra.Command{
		Use:   "token",
		Short: "Print the Requesty API key of the profile in use",
		Long: "Print the API key of the profile named with --" + profileFlag + " or " + profileEnv + ", else the\n" +
			"current one. Harnesses that fetch their key on demand, such as Codex, call this.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), env.session.config.APIKey); err != nil {
				return fmt.Errorf("failed to print API key: %w", err)
			}
			return nil
		},
	})

	return auth
}
