package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/tui"
	"github.com/spf13/cobra"
)

// Run executes the requesty command line.
func Run() error {
	env, err := newEnvironment()
	if err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	return newRootCommand(env).Execute()
}

func newRootCommand(env *environment) *cobra.Command {
	root := &cobra.Command{
		Use:   "requesty",
		Short: "Point your AI coding harnesses at Requesty",
		Long: "Requesty routes every AI coding harness on your machine through one gateway.\n\n" +
			"Run with no arguments for the terminal app that configures harnesses and shows\n" +
			"what you are spending. The subcommands manage your organization instead.\n\n" +
			"Every command runs as one saved profile: an API key and its router. Name one\n" +
			"with --" + profileFlag + "; otherwise the current profile is used.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := env.ensureProfile(cmd, "", "")
			if err != nil {
				return err
			}

			if _, err := tea.NewProgram(tui.NewRoot(cfg)).Run(); err != nil {
				return fmt.Errorf("failed to run program: %w", err)
			}

			return nil
		},
	}
	root.PersistentFlags().String(profileFlag, "", "saved profile to run as")

	root.AddCommand(
		newLoginCommand(env),
		newAuthCommand(env),
		newProfilesCommand(env),
		newAPIKeysCommand(env),
		newGroupsCommand(env),
		newAccessListsCommand(env),
	)
	root.AddCommand(newHarnessCommands(env)...)

	return root
}
