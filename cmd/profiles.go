package cmd

import (
	"fmt"
	"os"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/onboarding"
	"github.com/spf13/cobra"
)

func newProfilesCommand(env *environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profiles",
		Aliases: []string{"profile"},
		Short:   "Manage the profiles saved on this machine",
		Long: "Manage the profiles saved in " + config.DisplayPath() + ".\n\n" +
			"A profile is one sign-in: an API key and the router it goes to. `requesty login`\n" +
			"creates them; pick one for a run with --" + profileFlag + " or " + profileEnv + ", or make it\n" +
			"current with `requesty profiles use`.",
	}

	cmd.PersistentFlags().Bool(jsonFlag, false, "print JSON instead of a table")
	cmd.AddCommand(
		newProfilesListCommand(env),
		newProfilesUseCommand(env),
		newProfilesRemoveCommand(env),
	)

	return cmd
}

// profileListing is one profile as `profiles list --json` prints it; the key
// itself stays out.
type profileListing struct {
	Name          string `json:"name"`
	Current       bool   `json:"current"`
	RouterBaseURL string `json:"router_base_url"`
}

func newProfilesListCommand(env *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the saved profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			listings := make([]profileListing, 0, len(env.store.Profiles))
			for _, name := range env.store.Names() {
				listings = append(listings, profileListing{
					Name:          name,
					Current:       name == env.store.Current,
					RouterBaseURL: env.store.Profiles[name].RouterBaseURL,
				})
			}

			if jsonOutput(cmd) {
				return printJSON(out, listings)
			}
			if len(listings) == 0 {
				_, err := fmt.Fprintln(out, "No profiles yet. Run `requesty login` to sign in.")
				return err
			}

			rows := make([][]string, 0, len(listings))
			for _, listing := range listings {
				marker := ""
				if listing.Current {
					marker = "*"
				}
				rows = append(rows, []string{marker, listing.Name, listing.RouterBaseURL})
			}

			return writeTable(out, []string{"", "NAME", "ROUTER"}, rows)
		},
	}
}

func newProfilesUseCommand(env *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Make a profile the current one",
		Long:  "Make a profile the one used when none is named with --" + profileFlag + " or " + profileEnv + ".",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := env.store.Use(name); err != nil {
				return err
			}
			if err := config.Save(env.store); err != nil {
				return err
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Profile %q is now current.\n", name)
			return err
		},
	}
}

func newProfilesRemoveCommand(env *environment) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Forget a profile on this machine",
		Long: "Forget a profile on this machine. The API key itself is not revoked; do that on\n" +
			onboarding.APIKeysURL + " if it should stop working.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := env.store.Remove(name); err != nil {
				return err
			}
			if err := config.Save(env.store); err != nil {
				return err
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q. Its API key still exists; revoke it at %s if needed.\n",
				name, onboarding.APIKeysURL)
			return err
		},
	}
}

// requestedProfile is the profile the user named for this run: the flag,
// else the environment variable, else empty for whatever the config says.
func requestedProfile(cmd *cobra.Command) string {
	if flag := cmd.Flags().Lookup(profileFlag); flag != nil && flag.Changed {
		return flag.Value.String()
	}

	return os.Getenv(profileEnv)
}
