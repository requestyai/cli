package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/onboarding"
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

// environment is the loaded config and the gateway client the commands share.
type environment struct {
	config      config.Config
	apiv2Client *client.Client
}

func newEnvironment() (environment, error) {
	cfg, err := config.Load()
	if err != nil {
		return environment{}, fmt.Errorf("failed to load config: %w", err)
	}

	return environment{
		config:      cfg,
		apiv2Client: client.New(cfg),
	}, nil
}

// ensureAPIKey is the config to launch with, onboarding first when no key is
// saved yet. harness is the display name of the harness about to start, or
// empty for the dashboard.
func (env environment) ensureAPIKey(cmd *cobra.Command, harness string) (config.Config, error) {
	if env.config.APIKey != "" {
		return env.config, nil
	}

	return onboarding.Run(cmd.Context(), onboarding.Options{
		Config:  env.config,
		Harness: harness,
	})
}

func newRootCommand(env environment) *cobra.Command {
	root := &cobra.Command{
		Use:   "requesty",
		Short: "Point your AI coding harnesses at Requesty",
		Long: "Requesty routes every AI coding harness on your machine through one gateway.\n\n" +
			"Run with no arguments for the terminal app that configures harnesses and shows\n" +
			"what you are spending. The subcommands manage your organization instead.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := env.ensureAPIKey(cmd, "")
			if err != nil {
				return err
			}

			if _, err := tea.NewProgram(tui.NewRoot(cfg)).Run(); err != nil {
				return fmt.Errorf("failed to run program: %w", err)
			}

			return nil
		},
	}

	root.AddCommand(
		newLoginCommand(env),
		newAuthCommand(env),
		newAPIKeysCommand(env),
		newGroupsCommand(env),
		newAccessListsCommand(env),
	)
	root.AddCommand(newHarnessCommands(env)...)

	return root
}
