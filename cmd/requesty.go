package cmd

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/requestyai/cli/internal/oauth"
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

// environment is everything the commands need from the outside world, kept
// behind function fields so tests can stand in for the gateway and the UI.
type environment struct {
	config      config.Config
	apiv2Client *client.Client

	// login runs the browser sign-in. nil means the real OAuth flow.
	login func(context.Context, oauth.Options) (*oauth.Token, error)

	// launchHarness starts a harness through Requesty. nil means the harness's
	// own Launch, which replaces the current process and so cannot run under
	// a test.
	launchHarness func(harnesses.Harness, harnesses.LaunchOptions) error
}

func newEnvironment() (environment, error) {
	cfg, err := config.Load()
	if err != nil {
		return environment{}, fmt.Errorf("failed to load config: %w", err)
	}

	return environment{
		config:        cfg,
		apiv2Client:   client.New(cfg),
		login:         oauth.Login,
		launchHarness: harnesses.Harness.Launch,
	}, nil
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
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := tea.NewProgram(tui.NewRoot(env.config)).Run(); err != nil {
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
