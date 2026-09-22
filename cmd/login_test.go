package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkServer answers /v1/auth/check with status, standing in for the gateway
// when a login has to decide whether a supplied key works.
func checkServer(t *testing.T, status int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/auth/check", r.URL.Path)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	return server
}

func TestLoginSavesValidatedKeyAsCurrentDefaultProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusOK)
	var output bytes.Buffer
	command := newRootCommand(&environment{})
	command.SetOut(&output)
	command.SetArgs([]string{"login", "--api-key", "rqsty-existing", "--router-url", server.URL})

	require.NoError(t, command.Execute())

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, defaultProfileName, saved.Current)
	assert.Equal(t, config.Config{APIKey: "rqsty-existing", RouterBaseURL: server.URL}, saved.Profiles[defaultProfileName])
	assert.Contains(t, output.String(), `Saved profile "default"`)
	assert.NotContains(t, output.String(), "rqsty-existing")
}

func TestLoginSavesNamedProfileWithoutChangingCurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusOK)
	command := newRootCommand(&environment{store: clone(oneProfile)})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"login", "--api-key", "rqsty-manage", "--profile", "manage", "--router-url", server.URL})

	require.NoError(t, command.Execute())

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "work", saved.Current)
	assert.Equal(t, "rqsty-manage", saved.Profiles["manage"].APIKey)
}

func TestLoginRequiresForceToReplaceProfile(t *testing.T) {
	server := checkServer(t, http.StatusOK)
	store := config.Store{Current: "default", Profiles: map[string]config.Config{"default": {APIKey: "rqsty-old", RouterBaseURL: server.URL}}}
	command := newRootCommand(&environment{store: store})
	command.SetArgs([]string{"login", "--api-key", "rqsty-new"})

	err := command.Execute()

	require.ErrorContains(t, err, "pass --force")
}

func TestLoginForceReplacesKeyAndKeepsRouter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusOK)
	store := config.Store{Current: "default", Profiles: map[string]config.Config{"default": {APIKey: "rqsty-old", RouterBaseURL: server.URL}}}
	command := newRootCommand(&environment{store: store})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"login", "--api-key", "rqsty-new", "--force"})

	require.NoError(t, command.Execute())

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, config.Config{APIKey: "rqsty-new", RouterBaseURL: server.URL}, saved.Profiles["default"])
}

func TestLoginRejectsUnrecognisedKeyWithoutSaving(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := checkServer(t, http.StatusUnauthorized)
	command := newRootCommand(&environment{})
	command.SetArgs([]string{"login", "--api-key", "rqsty-typo", "--router-url", server.URL})

	require.ErrorIs(t, command.Execute(), errUnrecognisedAPIKey)

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, saved.Profiles)
}

func TestLoginFlagsAreRegistered(t *testing.T) {
	command := newRootCommand(&environment{})
	login, _, err := command.Find([]string{"login"})
	require.NoError(t, err)
	for _, name := range []string{loginGroupFlag, loginAPIKeyFlag, loginForceFlag, loginRouterURLFlag} {
		assert.NotNil(t, login.Flags().Lookup(name), "missing --%s", name)
	}
	assert.NotNil(t, login.InheritedFlags().Lookup(profileFlag))
}
