package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkServer answers /v1/auth/check with status, standing in for the gateway
// when a login has to decide whether the saved key still works.
func checkServer(t *testing.T, status int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/auth/check", r.URL.Path)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	return server
}

func loginEnvironment(cfg config.Config) environment {
	return environment{config: cfg, apiv2Client: client.New(cfg)}
}

func TestLoginDoesNothingWhenKeyStillWorks(t *testing.T) {
	server := checkServer(t, http.StatusOK)
	env := loginEnvironment(config.Config{APIKey: "rqsty-old", RouterBaseURL: server.URL})
	var output bytes.Buffer
	command := newRootCommand(env)
	command.SetOut(&output)
	command.SetArgs([]string{"login"})

	require.NoError(t, command.Execute())

	assert.Contains(t, output.String(), "Already signed in")
	assert.Contains(t, output.String(), "requesty login --force")
}

func TestLoginReportsCheckFailure(t *testing.T) {
	server := checkServer(t, http.StatusBadGateway)
	env := loginEnvironment(config.Config{APIKey: "rqsty-old", RouterBaseURL: server.URL})
	command := newRootCommand(env)
	command.SetArgs([]string{"login"})

	err := command.Execute()

	require.ErrorContains(t, err, "failed to check the saved API key")
}

func TestLoginSavesAValidatedAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusOK)
	env := loginEnvironment(config.Config{RouterBaseURL: server.URL})
	var output bytes.Buffer
	command := newRootCommand(env)
	command.SetOut(&output)
	command.SetArgs([]string{"login", "--api-key", "rqsty-existing"})

	require.NoError(t, command.Execute())

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, config.Config{
		APIKey:        "rqsty-existing",
		RouterBaseURL: server.URL,
	}, saved, "the key is added to the loaded config, which is otherwise saved as it was")
	assert.Contains(t, output.String(), "Saved your API key")
	assert.NotContains(t, output.String(), "rqsty-existing")
}

func TestLoginRejectsAnUnrecognisedAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusUnauthorized)
	env := loginEnvironment(config.Config{RouterBaseURL: server.URL})
	command := newRootCommand(env)
	command.SetArgs([]string{"login", "--api-key", "rqsty-typo"})

	require.ErrorIs(t, command.Execute(), errUnrecognisedAPIKey)

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, saved.APIKey, "a rejected key must not be written")
}

func TestLoginFlagsAreRegistered(t *testing.T) {
	command := newRootCommand(environment{})

	login, _, err := command.Find([]string{"login"})
	require.NoError(t, err)

	for _, name := range []string{loginGroupFlag, loginAPIKeyFlag, loginForceFlag} {
		assert.NotNil(t, login.Flags().Lookup(name), "missing --%s", name)
	}
}
