package cmd

import (
	"bytes"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	oneProfile = config.Store{
		Current: "work",
		Profiles: map[string]config.Config{
			"work": {APIKey: "my-api-key", RouterBaseURL: config.DefaultRouterBaseURL},
		},
	}
	twoProfiles = config.Store{
		Current: "engineering",
		Profiles: map[string]config.Config{
			"engineering": {APIKey: "rqsty-eng", RouterBaseURL: config.DefaultRouterBaseURL},
			"personal":    {APIKey: "rqsty-me", RouterBaseURL: config.DefaultRouterBaseURL},
		},
	}
)

// clone copies a fixture so a command that writes profiles does not change
// it for other tests.
func clone(store config.Store) config.Store {
	store.Profiles = maps.Clone(store.Profiles)
	return store
}

func TestAuthTokenPrintsCurrentProfileKey(t *testing.T) {
	var output bytes.Buffer
	command := newRootCommand(&environment{store: oneProfile})
	command.SetOut(&output)
	command.SetArgs([]string{"auth", "token"})

	require.NoError(t, command.Execute())
	assert.Equal(t, "my-api-key\n", output.String())
}

func TestAuthTokenPicksExplicitProfile(t *testing.T) {
	var output bytes.Buffer
	command := newRootCommand(&environment{store: twoProfiles})
	command.SetOut(&output)
	command.SetArgs([]string{"auth", "token", "--profile", "personal"})

	require.NoError(t, command.Execute())
	assert.Equal(t, "rqsty-me\n", output.String())
}

func TestAuthTokenUsesEnvironmentProfile(t *testing.T) {
	t.Setenv(profileEnv, "personal")
	var output bytes.Buffer
	command := newRootCommand(&environment{store: twoProfiles})
	command.SetOut(&output)
	command.SetArgs([]string{"auth", "token"})

	require.NoError(t, command.Execute())
	assert.Equal(t, "rqsty-me\n", output.String())
}

func TestAuthTokenRejectsMissingProfile(t *testing.T) {
	command := newRootCommand(&environment{})
	command.SetArgs([]string{"auth", "token"})

	err := command.Execute()

	require.ErrorIs(t, err, config.ErrNoProfiles)
}

func TestManagementCommandUsesSelectedProfile(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(server.Close)

	store := config.Store{
		Current: "work",
		Profiles: map[string]config.Config{
			"work":     {APIKey: "rqsty-work", RouterBaseURL: server.URL},
			"personal": {APIKey: "rqsty-me", RouterBaseURL: server.URL},
		},
	}
	command := newRootCommand(&environment{store: store})
	command.SetArgs([]string{"api-keys", "list", "--profile", "personal"})

	require.NoError(t, command.Execute())
	assert.Equal(t, "Bearer rqsty-me", authorization)
}

func TestAuthCommandIsHidden(t *testing.T) {
	command := newRootCommand(&environment{})

	auth, _, err := command.Find([]string{"auth"})

	require.NoError(t, err)
	assert.True(t, auth.Hidden)
}
