package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginPrintsScopesAndExpiry(t *testing.T) {
	var gotOptions oauth.Options
	var output bytes.Buffer
	command := newRootCommand(environment{
		config: config.Config{APIBaseURL: "http://localhost:40003"},
		login: func(_ context.Context, opts oauth.Options) (*oauth.Token, error) {
			gotOptions = opts
			return &oauth.Token{
				AccessToken: "secret-token",
				TokenType:   "Bearer",
				Scope:       "manage:group:r manage:apikey:w",
				ExpiresIn:   5 * time.Minute,
			}, nil
		},
	})
	command.SetOut(&output)
	command.SetArgs([]string{"login"})

	require.NoError(t, command.Execute())

	assert.Equal(t, "http://localhost:40003", gotOptions.APIBaseURL)
	assert.NotNil(t, gotOptions.Status)
	assert.Equal(t,
		"Signed in to Requesty.\n"+
			"Scopes      manage:group:r manage:apikey:w\n"+
			"Expires in  5m0s\n",
		output.String())
	assert.NotContains(t, output.String(), "secret-token")
}

func TestLoginDefaultsToProductionAPI(t *testing.T) {
	var gotOptions oauth.Options
	command := newRootCommand(environment{
		login: func(_ context.Context, opts oauth.Options) (*oauth.Token, error) {
			gotOptions = opts
			return &oauth.Token{AccessToken: "t", ExpiresIn: time.Minute}, nil
		},
	})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"login"})

	require.NoError(t, command.Execute())
	assert.Equal(t, config.DefaultAPIBaseURL, gotOptions.APIBaseURL)
}

func TestLoginPrintTokenFlagRevealsToken(t *testing.T) {
	var output bytes.Buffer
	command := newRootCommand(environment{
		login: func(context.Context, oauth.Options) (*oauth.Token, error) {
			return &oauth.Token{AccessToken: "secret-token", Scope: "manage:group:r", ExpiresIn: time.Minute}, nil
		},
	})
	command.SetOut(&output)
	command.SetArgs([]string{"login", "--print-token"})

	require.NoError(t, command.Execute())
	assert.Contains(t, output.String(), "Access token  secret-token\n")
}

func TestLoginPrintTokenFlagIsHidden(t *testing.T) {
	command := newRootCommand(environment{})

	login, _, err := command.Find([]string{"login"})
	require.NoError(t, err)

	flag := login.Flags().Lookup(loginPrintTokenFlag)
	require.NotNil(t, flag)
	assert.True(t, flag.Hidden)
}

func TestLoginReportsFailure(t *testing.T) {
	command := newRootCommand(environment{
		login: func(context.Context, oauth.Options) (*oauth.Token, error) {
			return nil, errors.New("sign-in was not completed: access_denied")
		},
	})
	command.SetArgs([]string{"login"})

	require.EqualError(t, command.Execute(), "sign-in was not completed: access_denied")
}
