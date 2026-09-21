package config

import (
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
		want Config
	}{
		{
			name: "missing file gives defaults with no key",
			want: Config{RouterBaseURL: DefaultRouterBaseURL},
		},
		{
			name: "blank router is filled in, key kept",
			file: `{"api_key": "rqsty-old"}`,
			want: Config{APIKey: "rqsty-old", RouterBaseURL: DefaultRouterBaseURL},
		},
		{
			name: "explicit router is returned verbatim",
			file: `{"api_key": "rqsty-old", "router_base_url": "http://localhost:40000"}`,
			want: Config{APIKey: "rqsty-old", RouterBaseURL: "http://localhost:40000"},
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

func TestSaveRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := Config{APIKey: "rqsty-new", RouterBaseURL: "http://localhost:40000"}

	require.NoError(t, Save(want))
	got, err := Load()

	require.NoError(t, err)
	assert.Equal(t, want, got)
}
