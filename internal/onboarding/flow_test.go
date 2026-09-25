package onboarding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manageServer stands in for the gateway's key-creation endpoint.
type manageServer struct {
	t             *testing.T
	createStatus  int
	createMessage string
	tokens        []string
	creates       []map[string]any
}

func (s *manageServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/manage/apikey", func(w http.ResponseWriter, r *http.Request) {
		s.tokens = append(s.tokens, r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(s.t, json.NewDecoder(r.Body).Decode(&body))
		s.creates = append(s.creates, body)
		if s.createStatus != 0 {
			w.WriteHeader(s.createStatus)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": s.createMessage}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"api_key_id": "key-1", "api_key": "rqsty-new"})
	})
	return mux
}

// signedIn is a session as signIn would leave it, minus the browser: an
// OAuth token in the client and the caller's groups listed. The router points
// at the fake, which the management API address follows. HOME is a throwaway
// so a test can check that nothing is written there.
func signedIn(t *testing.T, groups ...client.Group) (*session, *manageServer) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	fake := &manageServer{t: t}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	cfg := config.Config{RouterBaseURL: server.URL}
	authConfig := cfg
	authConfig.APIKey = "token"
	return &session{
		opts:   Options{Config: cfg},
		api:    client.New(authConfig),
		groups: groups,
	}, fake
}

func group(id, name string) client.Group {
	return client.Group{ID: id, Name: name}
}

func TestChooseGroup(t *testing.T) {
	engineering, research := group("g-1", "Engineering"), group("g-2", "Research")

	tests := []struct {
		name    string
		groups  []client.Group
		groupID string
		want    *client.Group
		wantErr error
	}{
		{name: "no groups means a personal key", groups: nil, want: nil},
		{name: "a single group is chosen", groups: []client.Group{engineering}, want: &engineering},
		{name: "several groups need a choice", groups: []client.Group{engineering, research}, wantErr: errGroupChoiceRequired},
		{name: "by id", groups: []client.Group{engineering, research}, groupID: "g-2", want: &research},
		{name: "by name, case-insensitively", groups: []client.Group{engineering, research}, groupID: "research", want: &research},
		{name: "unknown group", groups: []client.Group{engineering}, groupID: "Research", wantErr: errGroupNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &session{opts: Options{GroupID: tc.groupID}, groups: tc.groups}

			got, err := s.chooseGroup()

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestChooseGroupNamesTheOptions(t *testing.T) {
	s := &session{groups: []client.Group{group("g-1", "Engineering"), group("g-2", "Research")}}

	_, err := s.chooseGroup()

	require.ErrorContains(t, err, "you belong to 2 groups (Engineering, Research)")
}

func TestCreateKeyPersonal(t *testing.T) {
	s, fake := signedIn(t)

	r, err := s.createKey(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, "rqsty-new", r.Config.APIKey)
	assert.Equal(t, s.opts.Config.RouterBaseURL, r.Config.RouterBaseURL, "the loaded router is kept")
	assert.Equal(t, defaultKeyName(), r.KeyName)
	assert.Nil(t, r.Group)
	assert.Equal(t, []string{"Bearer token"}, fake.tokens, "the OAuth token creates the key")
	assert.Equal(t, map[string]any{"name": defaultKeyName()}, fake.creates[0], "no group_id for a personal key")

	saved, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, saved.Profiles, "the command layer owns persistence")
}

func TestCreateKeyInGroup(t *testing.T) {
	research := group("g-2", "Research")
	s, fake := signedIn(t, research)

	r, err := s.createKey(context.Background(), &research)

	require.NoError(t, err)
	assert.Equal(t, &research, r.Group)
	assert.Equal(t, "g-2", fake.creates[0]["group_id"])
}

func TestCreateKeyReportsGatewayFailure(t *testing.T) {
	s, fake := signedIn(t)
	fake.createStatus = http.StatusBadRequest
	fake.createMessage = "group_id is required"

	_, err := s.createKey(context.Background(), nil)

	require.ErrorContains(t, err, "group_id is required")
}

func TestFindGroup(t *testing.T) {
	groups := []client.Group{group("g-1", "Engineering"), group("g-2", "Research")}

	byID, ok := findGroup(groups, "g-2")
	require.True(t, ok)
	assert.Equal(t, "Research", byID.Name)

	byName, ok := findGroup(groups, "ENGINEERING")
	require.True(t, ok)
	assert.Equal(t, "g-1", byName.ID)

	_, ok = findGroup(groups, "Sales")
	assert.False(t, ok)
}

func TestKeyNameFor(t *testing.T) {
	tests := map[string]string{
		"my-laptop":                 "Requesty CLI (my-laptop)",
		"Fayzans-MacBook-Pro.local": "Requesty CLI (Fayzans-MacBook-Pro)",
		"ci:runner#7":               "Requesty CLI (cirunner7)",
		"...":                       "Requesty CLI",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz": "Requesty CLI (abcdefghijklmnopqrstuvwxyzabcdefghijklmn)",
	}
	for host, want := range tests {
		assert.Equal(t, want, keyNameFor(host))
	}
}
