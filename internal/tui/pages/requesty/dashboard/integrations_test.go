package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHarness struct {
	configured []harnesses.ConfigureOptions
}

func (h *fakeHarness) Name() string          { return "Claude Code" }
func (h *fakeHarness) Description() []string { return nil }
func (h *fakeHarness) Status() (harnesses.Status, error) {
	return harnesses.Status{Executable: true}, nil
}
func (h *fakeHarness) Launch(harnesses.LaunchOptions) error { return nil }
func (h *fakeHarness) Configure(o harnesses.ConfigureOptions) error {
	h.configured = append(h.configured, o)
	return nil
}

// newIntegrationTestState is the integrations page with one installed
// harness, backed by a router that lists one policy and one model.
func newIntegrationTestState(t *testing.T) (integrationState, *fakeHarness) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := "anthropic/claude-sonnet-4-6"
		if r.URL.Path == "/v1/models/managed" {
			id = "claude-sonnet-4-6"
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []client.Model{{ID: id}}}))
	}))
	t.Cleanup(server.Close)

	cfg := config.Config{RouterBaseURL: server.URL, APIKey: "test-key"}
	harness := &fakeHarness{}
	m := newIntegrationState(client.New(cfg), cfg)
	m, _ = m.update(integrationsLoadedMsg{items: []integrationItem{{harness: harness, status: harnesses.Status{Executable: true}}}})

	return m, harness
}

// run feeds msg to m, then every message its commands produce, the way the
// program would. Cursor blinks are dropped so the loop ends, and so is the
// harness reload after configuring, which would look at this machine.
func run(m integrationState, msg tea.Msg) integrationState {
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		var cmd tea.Cmd
		m, cmd = m.update(queue[0])
		queue = append(queue[1:], drain(cmd)...)
	}

	return m
}

func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	switch msg := cmd().(type) {
	case nil, integrationsLoadedMsg:
		return nil
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, c := range msg {
			msgs = append(msgs, drain(c)...)
		}
		return msgs
	default:
		if strings.HasPrefix(reflect.TypeOf(msg).PkgPath(), "charm.land/bubbles") {
			return nil
		}
		return []tea.Msg{msg}
	}
}

func key(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

func wizardText(m integrationState) string {
	return ansi.Strip(m.wizardView(76, 22))
}

func TestConfigureOpensThePickerWithManagedPolicies(t *testing.T) {
	m, _ := newIntegrationTestState(t)

	m = run(m, key("enter"))

	require.True(t, m.wizard.open)
	out := wizardText(m)
	assert.Contains(t, out, "Choose a model")
	assert.Contains(t, out, "Claude Code")
	assert.Contains(t, out, "Policies (1)")
	assert.Contains(t, out, "Models (1)")
	assert.Contains(t, out, "❯ claude-sonnet-4-6")
	assert.Contains(t, out, "enter continue")
}

func TestConfigureWithAManagedPolicy(t *testing.T) {
	m, harness := newIntegrationTestState(t)

	m = run(m, key("enter"))
	m = run(m, key("enter"))

	assert.Equal(t, integrationModeWizardStep, m.wizard.step)
	assert.Contains(t, wizardText(m), "Choose how to configure the harness")

	m = run(m, key("enter"))

	assert.Equal(t, []harnesses.ConfigureOptions{{Model: "claude-sonnet-4-6"}}, harness.configured)
	assert.False(t, m.wizard.open)
}

func TestConfigureWithAModelFromTheModelsTab(t *testing.T) {
	m, harness := newIntegrationTestState(t)

	m = run(m, key("enter"))
	m = run(m, key("tab"))
	m = run(m, key("enter"))
	m = run(m, key("enter"))

	assert.Equal(t, []harnesses.ConfigureOptions{{Model: "anthropic/claude-sonnet-4-6"}}, harness.configured)
}

func TestEscapeFromModeStepReturnsToThePicker(t *testing.T) {
	m, _ := newIntegrationTestState(t)

	m = run(m, key("enter"))
	m = run(m, key("enter"))
	m = run(m, key("esc"))

	assert.True(t, m.wizard.open)
	assert.Equal(t, integrationModelWizardStep, m.wizard.step)
	assert.Empty(t, m.wizard.options.Model)
	assert.Contains(t, wizardText(m), "❯ claude-sonnet-4-6")
}

func TestEscapeFromThePickerClosesTheWizard(t *testing.T) {
	m, harness := newIntegrationTestState(t)

	m = run(m, key("enter"))
	m = run(m, key("esc"))

	assert.False(t, m.wizard.open)
	assert.Empty(t, harness.configured)
}
