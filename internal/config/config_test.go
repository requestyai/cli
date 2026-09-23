package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIBaseURL(t *testing.T) {
	tests := []struct {
		name   string
		router string
		want   string
	}{
		{name: "production", router: DefaultRouterBaseURL, want: DefaultAPIBaseURL},
		{name: "staging", router: "https://router.staging.requesty.ai", want: "https://api-v2.staging.requesty.ai"},
		{name: "local stack moves port", router: "http://localhost:40000", want: "http://localhost:40003"},
		{name: "local stack by ip", router: "http://127.0.0.1:40000", want: "http://127.0.0.1:40003"},
		{name: "hostname and port both rewritten", router: "http://router.local:40000", want: "http://api-v2.local:40003"},
		{name: "other port is left alone", router: "http://127.0.0.1:55123", want: "http://127.0.0.1:55123"},
		{name: "unrelated host is left alone", router: "https://gateway.example.test", want: "https://gateway.example.test"},
		{name: "path and trailing slash survive", router: "https://router.requesty.ai/prefix/", want: "https://api-v2.requesty.ai/prefix/"},
		{name: "router in path is not touched", router: "https://gateway.example.test/router", want: "https://gateway.example.test/router"},
		{name: "unparseable address comes back as is", router: "http://[::1", want: "http://[::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Config{RouterBaseURL: tc.router}.APIBaseURL())
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		file string // empty means no file on disk
		want Store
	}{
		{
			name: "missing file means no profiles",
			want: Store{},
		},
		{
			name: "blank router is filled in, key kept",
			file: `{"current": "work", "profiles": {"work": {"api_key": "rqsty-old"}}}`,
			want: Store{Current: "work", Profiles: map[string]Config{"work": {APIKey: "rqsty-old", RouterBaseURL: DefaultRouterBaseURL}}},
		},
		{
			name: "explicit router is returned verbatim",
			file: `{"current": "local", "profiles": {"local": {"api_key": "rqsty-old", "router_base_url": "http://localhost:40000"}}}`,
			want: Store{Current: "local", Profiles: map[string]Config{"local": {APIKey: "rqsty-old", RouterBaseURL: "http://localhost:40000"}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.file != "" {
				dir := filepath.Join(home, dirName)
				require.NoError(t, os.MkdirAll(dir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(tc.file), 0o600))
			}

			got, err := Load()

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLoadRejectsMalformedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, dirName)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte("{not json"), 0o600))

	_, err := Load()

	require.ErrorContains(t, err, "failed to parse config")
}

func TestLoadRejectsInvalidStore(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{
			name: "profiles without current",
			file: `{"profiles":{"work":{"api_key":"k"}}}`,
			want: "current profile is required",
		},
		{
			name: "unknown current",
			file: `{"current":"gone","profiles":{"work":{"api_key":"k"}}}`,
			want: `current profile "gone" is not saved`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			dir := filepath.Join(home, dirName)
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(tc.file), 0o600))

			_, err := Load()

			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestSaveRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var store Store
	work := Config{APIKey: "rqsty-work", RouterBaseURL: "https://router.eu.example"}
	work.SetHarnessModel("claude", "claude-sonnet-4-6")
	store.Set("work", work)

	require.NoError(t, Save(store))
	got, err := Load()

	require.NoError(t, err)
	assert.Equal(t, store, got)
	assert.Equal(t, "claude-sonnet-4-6", got.Profiles["work"].HarnessModels["claude"])
}

func TestHarnessModelsAreOmittedUntilSet(t *testing.T) {
	data, err := json.Marshal(Config{APIKey: "k", RouterBaseURL: DefaultRouterBaseURL})

	require.NoError(t, err)
	assert.NotContains(t, string(data), "harness_models")
}

func TestResolve(t *testing.T) {
	work := Config{APIKey: "rqsty-work", RouterBaseURL: DefaultRouterBaseURL}
	personal := Config{APIKey: "rqsty-me", RouterBaseURL: DefaultRouterBaseURL}
	store := Store{Current: "work", Profiles: map[string]Config{"work": work, "personal": personal}}

	tests := []struct {
		name     string
		store    Store
		ask      string
		wantName string
		wantErr  string
	}{
		{name: "explicit name", store: store, ask: "personal", wantName: "personal"},
		{name: "current when unnamed", store: store, wantName: "work"},
		{name: "nothing saved", store: Store{}, wantErr: ErrNoProfiles.Error()},
		{name: "unknown name", store: store, ask: "sales", wantErr: `profile "sales" not found; saved profiles: personal, work`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := tc.store.Resolve(tc.ask)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, cfg.Name)
			assert.Equal(t, tc.store.Profiles[tc.wantName].APIKey, cfg.APIKey)
			assert.Equal(t, tc.store.Profiles[tc.wantName].RouterBaseURL, cfg.RouterBaseURL)
		})
	}
}

func TestNamesAreSorted(t *testing.T) {
	store := Store{Profiles: map[string]Config{"z": {}, "a": {}}}
	assert.Equal(t, []string{"a", "z"}, store.Names())
}

func TestRemoveCurrentSelectsNextProfile(t *testing.T) {
	store := Store{
		Current:  "work",
		Profiles: map[string]Config{"work": {}, "personal": {}, "admin": {}},
	}

	require.NoError(t, store.Remove("work"))

	assert.Equal(t, "admin", store.Current)
	assert.NotContains(t, store.Profiles, "work")
}
