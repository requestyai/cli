package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runProfiles runs `requesty profiles <args>` against a copy of store.
func runProfiles(t *testing.T, store config.Store, args ...string) (string, error) {
	t.Helper()

	var output bytes.Buffer
	command := newRootCommand(&environment{store: clone(store)})
	command.SetOut(&output)
	command.SetArgs(append([]string{"profiles"}, args...))
	err := command.Execute()

	return output.String(), err
}

func TestProfilesListTable(t *testing.T) {
	out, err := runProfiles(t, twoProfiles, "list")

	require.NoError(t, err)
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "*  engineering")
	assert.Contains(t, out, "personal")
	assert.Contains(t, out, config.DefaultRouterBaseURL)
	assert.NotContains(t, out, "rqsty-", "keys stay out of listings")
}

func TestProfilesListJSON(t *testing.T) {
	out, err := runProfiles(t, twoProfiles, "list", "--json")

	require.NoError(t, err)
	var listings []profileListing
	require.NoError(t, json.Unmarshal([]byte(out), &listings))
	assert.Equal(t, []profileListing{
		{Name: "engineering", Current: true, RouterBaseURL: config.DefaultRouterBaseURL},
		{Name: "personal", RouterBaseURL: config.DefaultRouterBaseURL},
	}, listings)
}

func TestProfilesListEmpty(t *testing.T) {
	out, err := runProfiles(t, config.Store{}, "list")

	require.NoError(t, err)
	assert.Contains(t, out, "No profiles yet")
}

func TestProfilesUseSetsCurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	out, err := runProfiles(t, twoProfiles, "use", "personal")

	require.NoError(t, err)
	assert.Contains(t, out, `"personal" is now current`)
	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "personal", saved.Current)
}

func TestProfilesUseUnknown(t *testing.T) {
	_, err := runProfiles(t, twoProfiles, "use", "sales")

	require.ErrorContains(t, err, `profile "sales" not found; saved profiles: engineering, personal`)
}

func TestProfilesRemoveClearsCurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	out, err := runProfiles(t, twoProfiles, "remove", "engineering")

	require.NoError(t, err)
	assert.Contains(t, out, `Removed profile "engineering"`)
	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"personal"}, saved.Names())
	assert.Equal(t, "personal", saved.Current)
}

func TestProfilesRemoveUnknown(t *testing.T) {
	_, err := runProfiles(t, twoProfiles, "remove", "sales")

	require.ErrorContains(t, err, `profile "sales" not found`)
}
