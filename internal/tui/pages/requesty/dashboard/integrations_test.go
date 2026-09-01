package dashboard

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHarness records what the wizard asked for instead of writing any files.
type stubHarness struct {
	options harnesses.ConfigureOptions
}

func (s *stubHarness) Name() string          { return "Stub" }
func (s *stubHarness) Description() []string { return nil }

func (s *stubHarness) Status() (harnesses.Status, error) {
	return harnesses.Status{Executable: true}, nil
}

func (s *stubHarness) Configure(options harnesses.ConfigureOptions) error {
	s.options = options
	return nil
}

// TestIntegrationWizardLeavesAttributionOff pins that walking the wizard without
// choosing anything configures the harness the way it always was: a user has to
// ask for their repository and branch to be sent.
func TestIntegrationWizardLeavesAttributionOff(t *testing.T) {
	harness := &stubHarness{}
	state := openWizard(t, harness)

	state = press(t, state, "enter") // pick the model
	require.Equal(t, integrationAttributionWizardStep, state.wizard.step)

	state = press(t, state, "enter") // keep requests unattributed
	require.Equal(t, integrationModeWizardStep, state.wizard.step)
	assert.Empty(t, state.wizard.options.Attribution)

	state = configureFrom(t, state)
	assert.Empty(t, harness.options.Attribution)
	assert.Equal(t, "anthropic/claude-sonnet-4-5", harness.options.Model)
}

// TestIntegrationWizardOptsIntoAttribution walks the same steps, choosing the
// second row of the attribution step.
func TestIntegrationWizardOptsIntoAttribution(t *testing.T) {
	harness := &stubHarness{}
	state := openWizard(t, harness)

	state = press(t, state, "enter")
	state = press(t, state, "down")
	state = press(t, state, "enter")
	require.Equal(t, integrationModeWizardStep, state.wizard.step)

	headers := make([]string, 0, len(state.wizard.options.Attribution))
	for _, dimension := range state.wizard.options.Attribution {
		headers = append(headers, dimension.Header)
	}
	assert.Equal(t, []string{"X-Requesty-Repo", "X-Requesty-Branch", "X-Requesty-User"}, headers)
}

// TestIntegrationWizardForgetsAttributionOnTheWayBack covers stepping back out of
// the last step, which has to leave nothing behind from the answer given before.
func TestIntegrationWizardForgetsAttributionOnTheWayBack(t *testing.T) {
	state := openWizard(t, &stubHarness{})

	state = press(t, state, "enter")
	state = press(t, state, "down")
	state = press(t, state, "enter")
	require.NotEmpty(t, state.wizard.options.Attribution)

	state = press(t, state, "esc")
	assert.Equal(t, integrationAttributionWizardStep, state.wizard.step)
	assert.Empty(t, state.wizard.options.Attribution)

	state = press(t, state, "esc")
	assert.Equal(t, integrationModelWizardStep, state.wizard.step)
	assert.Empty(t, state.wizard.options.Model)
}

// TestIntegrationWizardRendersEveryStep is a smoke test over the wizard pages,
// as a step that renders nothing is invisible in the flow tests above.
func TestIntegrationWizardRendersEveryStep(t *testing.T) {
	state := openWizard(t, &stubHarness{})

	for _, step := range []integrationWizardStep{
		integrationModelWizardStep,
		integrationAttributionWizardStep,
		integrationModeWizardStep,
	} {
		state.wizard.step = step
		assert.NotEmpty(t, state.wizardView(96, 40), "step %d rendered nothing", step)
	}
}

// openWizard opens the wizard on a harness with the model catalogue already
// loaded, which is where a user starts once they press enter on the list.
func openWizard(t *testing.T, harness harnesses.Harness) integrationState {
	t.Helper()

	state := integrationState{
		items: []integrationItem{{
			harness: harness,
			status:  harnesses.Status{Executable: true},
		}},
	}

	state.wizard, _ = newIntegrationWizardState()
	state.wizard.models = []client.Model{{ID: "anthropic/claude-sonnet-4-5"}}

	return state
}

// configureFrom presses enter on the last step and applies the message the
// resulting command produces, as the wizard writes files off the update loop.
func configureFrom(t *testing.T, state integrationState) integrationState {
	t.Helper()

	state, cmd := state.update(keyPress(t, "enter"))
	require.True(t, state.wizard.configuring)
	require.NotNil(t, cmd)

	state, _ = state.update(cmd())
	require.False(t, state.wizard.configuring)
	require.NoError(t, state.wizard.configureErr)

	return state
}

func press(t *testing.T, state integrationState, key string) integrationState {
	t.Helper()

	state, _ = state.update(keyPress(t, key))

	return state
}

func keyPress(t *testing.T, key string) tea.KeyPressMsg {
	t.Helper()

	msg := tea.KeyPressMsg{}
	switch key {
	case "enter":
		msg.Code = tea.KeyEnter
	case "esc":
		msg.Code = tea.KeyEscape
	case "down":
		msg.Code = tea.KeyDown
	default:
		t.Fatalf("unsupported key %q", key)
	}

	require.Equal(t, key, msg.String())

	return msg
}
