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
	loginGroupFlag     = "group"
	loginForceFlag     = "force"
	loginAPIKeyFlag    = "api-key"
	loginRouterURLFlag = "router-url"
)

var errUnrecognisedAPIKey = errors.New("that API key was not recognised; copy it again from " + onboarding.APIKeysURL)

func newLoginCommand(env *environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [--group <name>] [--profile <name>] [--api-key <key>] [--router-url <url>] [--force]",
		Short: "Sign in to Requesty and save a profile on this machine",
		Long: "Sign in to Requesty in your browser and save a profile on this machine.\n\n" +
			"The CLI creates an API key in your account, named after this machine, and saves\n" +
			"it as the `" + defaultProfileName + "` profile in " + config.DisplayPath() + ". Harness commands and\n" +
			"the terminal app run with it.\n\n" +
			"The browser hands the sign-in back on 127.0.0.1, which does not work over SSH;\n" +
			"there, pass --api-key with a key from " + onboarding.APIKeysURL + " instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			groupID, _ := cmd.Flags().GetString(loginGroupFlag)
			apiKey, _ := cmd.Flags().GetString(loginAPIKeyFlag)
			routerURL, _ := cmd.Flags().GetString(loginRouterURLFlag)
			force, _ := cmd.Flags().GetBool(loginForceFlag)

			name, _ := cmd.Flags().GetString(profileFlag)
			if name == "" {
				name = defaultProfileName
			}
			if existing, ok := env.store.Profiles[name]; ok {
				if !force {
					return fmt.Errorf("profile %q already exists; pass --force to replace it", name)
				}
				if routerURL == "" {
					routerURL = existing.RouterBaseURL
				}
			}
			if routerURL == "" {
				routerURL = config.DefaultRouterBaseURL
			}
			cfg := config.Config{APIKey: apiKey, RouterBaseURL: routerURL}

			if apiKey == "" {
				var err error
				cfg, err = onboarding.RunHeadless(cmd.Context(), onboarding.Options{Config: cfg, GroupID: groupID})
				if err != nil {
					return err
				}
			} else if err := checkAPIKey(cmd, cfg); err != nil {
				return err
			}

			if err := env.save(name, cfg); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Saved profile %q to %s.\n", name, config.DisplayPath())

			return err
		},
	}

	cmd.Flags().String(loginGroupFlag, "", "group to create the API key in, by name or id")
	cmd.Flags().String(loginAPIKeyFlag, "", "save an existing API key instead of signing in")
	cmd.Flags().String(loginRouterURLFlag, "", "router for the profile (default: the one it has, else "+config.DefaultRouterBaseURL+")")
	cmd.Flags().Bool(loginForceFlag, false, "replace the profile's key when it already exists")

	return cmd
}

// checkAPIKey asks the gateway whether a key the user supplied is live before
// it is saved. This is the route for machines where the browser cannot hand
// the sign-in back, such as over SSH.
func checkAPIKey(cmd *cobra.Command, cfg config.Config) error {
	err := client.New(cfg).CheckAPIKey(cmd.Context())
	if errors.Is(err, client.ErrInvalidAPIKey) {
		return errUnrecognisedAPIKey
	}
	if err != nil {
		return fmt.Errorf("failed to check the api key: %w", err)
	}

	return nil
}
