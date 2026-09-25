package modelpicker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lists is what a fake router serves for the two endpoints the picker reads.
// A nil list answers with an error status rather than an empty list.
type lists struct {
	policies []client.Model
	models   []client.Model
}

// newClient returns a client pointed at a router that serves l.
func newClient(t *testing.T, l lists) *client.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var models []client.Model
		switch r.URL.Path {
		case "/v1/models/managed":
			models = l.policies
		case "/v1/models":
			models = l.models
		}
		if models == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models}))
	}))
	t.Cleanup(server.Close)

	return client.New(config.Config{RouterBaseURL: server.URL, APIKey: "test-key"})
}

var (
	policies = []client.Model{
		{ID: "gpt-5.5", ContextWindow: 400_000, InputPrice: 0.0000055, OutputPrice: 0.000033},
		{ID: "claude-sonnet-4-6", ContextWindow: 1_000_000, InputPrice: 0.000003, OutputPrice: 0.000015},
		{ID: "claude-fable-5", ContextWindow: 1_000_000, InputPrice: 0.00001, OutputPrice: 0.00005},
	}
	models = []client.Model{
		{ID: "anthropic/claude-sonnet-4-6", ContextWindow: 1_000_000},
		{ID: "openai/gpt-5.5", ContextWindow: 400_000},
	}
)

func key(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

func update(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

func view(m model) string {
	return ansi.Strip(m.View().Content)
}

// loaded runs the picker's own load commands so both tabs are filled in.
func loaded(t *testing.T, l lists, preferred ...string) model {
	t.Helper()

	m := newModel(context.Background(), Options{Client: newClient(t, l), Harness: "Claude Code", Preferred: preferred})
	for _, tab := range []tab{tabPolicies, tabModels} {
		msg := m.load(tab)()
		require.IsType(t, loadedMsg{}, msg)
		m, _ = update(m, msg)
	}

	return m
}

func TestRunReturnsTheFirstRoutablePreferredWithoutAsking(t *testing.T) {
	opts := Options{Client: newClient(t, lists{policies: policies, models: models}), Harness: "Claude Code"}

	tests := []struct {
		name      string
		preferred []string
		want      string
	}{
		{name: "a managed policy", preferred: []string{"claude-sonnet-4-6"}, want: "claude-sonnet-4-6"},
		{name: "a model", preferred: []string{"openai/gpt-5.5"}, want: "openai/gpt-5.5"},
		{name: "the first that is listed", preferred: []string{"claude-haiku-4-5", "claude-fable-5", "gpt-5.5"}, want: "claude-fable-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts.Preferred = tt.preferred

			got, asked, err := Run(context.Background(), opts)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.False(t, asked)
		})
	}
}

func TestRoutablePreferredIsEmptyWhenNothingMatches(t *testing.T) {
	tests := []struct {
		name string
		l    lists
	}{
		{name: "not listed", l: lists{policies: policies, models: models}},
		{name: "lists cannot be fetched", l: lists{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{Client: newClient(t, tt.l), Preferred: []string{"claude-haiku-4-5"}}

			assert.Empty(t, routablePreferred(context.Background(), opts))
		})
	}
}

func TestShowsPoliciesFirstSortedWithPreferredUnderCursor(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models}, "claude-haiku-4-5", "claude-sonnet-4-6")

	assert.Equal(t, tabPolicies, m.tab)
	assert.Equal(t, 1, m.cursor, "sorted: claude-fable-5, claude-sonnet-4-6, gpt-5.5")
	out := view(m)
	assert.Contains(t, out, "Choose a model")
	assert.Contains(t, out, "Claude Code")
	assert.Contains(t, out, "Policies (3)")
	assert.Contains(t, out, "Models (2)")
	assert.Contains(t, out, "❯ claude-sonnet-4-6")
	assert.Contains(t, out, "1000K")
	assert.Contains(t, out, "$3.00")
	assert.Contains(t, out, "$15.00")
}

func TestEnterChoosesTheModelUnderTheCursor(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models})

	m, _ = update(m, key("down"))
	m, cmd := update(m, key("enter"))

	assert.Equal(t, "claude-sonnet-4-6", m.chosen)
	assert.NotNil(t, cmd, "enter quits the program")
}

func TestTabSwitchesToModelsAndBack(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models}, "openai/gpt-5.5")

	m, _ = update(m, key("tab"))

	assert.Equal(t, tabModels, m.tab)
	assert.Equal(t, 1, m.cursor, "preselect applies on whichever tab lists it")
	assert.Contains(t, view(m), "❯ openai/gpt-5.5")

	m, _ = update(m, key("tab"))

	assert.Equal(t, tabPolicies, m.tab)
	assert.Equal(t, 0, m.cursor)
}

func TestSearchFiltersTheCurrentTabAndResetsCursor(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models}, "gpt-5.5")
	require.Equal(t, 2, m.cursor)

	m, _ = update(m, key("s"))
	m, _ = update(m, key("o"))

	assert.Equal(t, 0, m.cursor)
	out := view(m)
	assert.Contains(t, out, "❯ claude-sonnet-4-6")
	assert.NotContains(t, out, "gpt-5.5")

	m, _ = update(m, key("x"))

	assert.Contains(t, view(m), "No policies match your search")
}

func TestNoPoliciesLandsOnModelsTab(t *testing.T) {
	m := loaded(t, lists{policies: []client.Model{}, models: models})

	assert.Equal(t, tabModels, m.tab)
	assert.Contains(t, view(m), "❯ anthropic/claude-sonnet-4-6")
}

func TestLoadErrorMovesToModelsAndIsShownOnItsTab(t *testing.T) {
	m := loaded(t, lists{models: models})

	assert.Equal(t, tabModels, m.tab)

	m, _ = update(m, key("tab"))

	assert.Equal(t, tabPolicies, m.tab)
	assert.Contains(t, view(m), "Could not load policies: status code not ok: 401")
}

func TestLoadingStateBeforeListsArrive(t *testing.T) {
	m := newModel(context.Background(), Options{Client: newClient(t, lists{}), Harness: "Codex"})

	assert.Contains(t, view(m), "Loading policies…")

	_, cmd := update(m, key("enter"))
	assert.Nil(t, cmd, "enter on an empty list does nothing")
}

func TestEscapeLeavesNothingChosen(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models})

	m, cmd := update(m, key("esc"))

	assert.Empty(t, m.chosen)
	assert.NotNil(t, cmd)
}

func TestCursorStaysWithinTheList(t *testing.T) {
	m := loaded(t, lists{policies: policies, models: models})

	for range 10 {
		m, _ = update(m, key("down"))
	}
	assert.Equal(t, 2, m.cursor)

	for range 10 {
		m, _ = update(m, key("up"))
	}
	assert.Equal(t, 0, m.cursor)
}
