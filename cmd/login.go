package cmd

import (
	"errors"
	"fmt"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/onboarding"
	"github.com/spf13/cobra"
)

const (
	loginGroupFlag  = "group"
	loginForceFlag  = "force"
	loginAPIKeyFlag = "api-key"
)

var errUnrecognisedAPIKey = errors.New("that API key was not recognised; copy it again from " + onboarding.APIKeysURL)

func newLoginCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [--group <name>] [--api-key <key>] [--force]",
		Short: "Sign in to Requesty and set this machine up",
		Long: "Sign in to Requesty in your browser and set this machine up.\n\n" +
			"Once you approve in the browser, the CLI creates an API key in your Requesty\n" +
			"account and saves it to " + config.DisplayPath() + ", where `requesty claude`,\n" +
			"`requesty codex` and the terminal app pick it up. The key is named after this\n" +
			"machine so you can recognise it on " + onboarding.APIKeysURL + ".\n\n" +
			"The key goes in your group. With several groups you are asked which, or you can\n" +
			"name one with --group. Without any group the key is a personal one.\n\n" +
			"The browser hands the sign-in back to this machine on 127.0.0.1, which does not\n" +
			"work over SSH. There, pass --api-key with a key from " + onboarding.APIKeysURL + "\n" +
			"instead; it is checked against the gateway and saved.\n\n" +
			"When a working key is already saved nothing is created; pass --force to replace\n" +
			"it with a new one.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			groupID, _ := cmd.Flags().GetString(loginGroupFlag)
			apiKey, _ := cmd.Flags().GetString(loginAPIKeyFlag)
			force, _ := cmd.Flags().GetBool(loginForceFlag)

			if env.config.APIKey != "" && !force {
				err := env.apiv2Client.CheckAPIKey(cmd.Context())
				switch {
				case err == nil:
					_, err := fmt.Fprintf(cmd.OutOrStdout(),
						"Already signed in: an API key is saved in %s.\nRun `requesty login --force` to replace it with a new one.\n",
						config.DisplayPath())
					return err
				case errors.Is(err, client.ErrInvalidAPIKey):
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "The saved API key is no longer valid; signing in again.")
				default:
					return fmt.Errorf("failed to check the saved API key: %w", err)
				}
			}

			if apiKey != "" {
				return env.saveAPIKey(cmd, apiKey)
			}

			_, err := onboarding.RunHeadless(cmd.Context(), onboarding.Options{
				Config:  env.config,
				GroupID: groupID,
			})

			return err
		},
	}

	cmd.Flags().String(loginGroupFlag, "", "group to create the API key in, by name or id")
	cmd.Flags().String(loginAPIKeyFlag, "", "save an existing API key instead of signing in")
	cmd.Flags().Bool(loginForceFlag, false, "create a new API key even when one is already saved")

	return cmd
}

// saveAPIKey stores a key the user already has, once the gateway confirms it
// is live. This is the route for machines where the browser cannot hand the
// sign-in back, such as over SSH.
func (env environment) saveAPIKey(cmd *cobra.Command, apiKey string) error {
	cfg := env.config
	cfg.APIKey = apiKey

	err := client.New(cfg).CheckAPIKey(cmd.Context())
	if errors.Is(err, client.ErrInvalidAPIKey) {
		return errUnrecognisedAPIKey
	}
	if err != nil {
		return fmt.Errorf("failed to check the api key: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved your API key to %s.\n", config.DisplayPath())

	return err
}
