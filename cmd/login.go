package cmd

import (
	"fmt"
	"strings"

	"github.com/requestyai/cli/internal/oauth"
	"github.com/spf13/cobra"
)

const (
	loginPrintTokenFlag = "print-token"
)

func newLoginCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Requesty in your browser",
		Long: "Sign in to Requesty in your browser.\n\n" +
			"The CLI opens the Requesty consent page and, once you approve, receives a\n" +
			"short-lived token on this machine. Nothing is written to disk.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			login := env.login
			if login == nil {
				login = oauth.Login
			}

			token, err := login(cmd.Context(), oauth.Options{
				APIBaseURL: env.config.ResolveAPIBaseURL(),
				Status:     cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintln(out, "Signed in to Requesty."); err != nil {
				return err
			}

			fields := [][2]string{
				{"Scopes", strings.Join(token.Scopes(), " ")},
				{"Expires in", token.ExpiresIn.String()},
			}
			if printToken, _ := cmd.Flags().GetBool(loginPrintTokenFlag); printToken {
				fields = append(fields, [2]string{"Access token", token.AccessToken})
			}

			return writeFields(out, fields)
		},
	}

	cmd.Flags().Bool(loginPrintTokenFlag, false, "print the access token for manual testing")
	_ = cmd.Flags().MarkHidden(loginPrintTokenFlag)

	return cmd
}
