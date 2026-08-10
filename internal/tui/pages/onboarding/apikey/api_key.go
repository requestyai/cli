package apikey

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

const checkTimeout = 10 * time.Second

// DoneMsg reports that onboarding finished and the config is on disk. The
// root model listens for it to hand over to the Requesty app.
type DoneMsg struct {
	Config config.Config
}

// failedMsg reports that the key could not be saved. The user stays on this
// step and can try again.
type failedMsg struct {
	err error
}

// Model is the API key step of the onboarding flow.
type Model struct {
	input    textinput.Model
	err      error
	checking bool
}

func New() Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.Placeholder = "rqsty-..."
	input.EchoMode = textinput.EchoPassword
	input.CharLimit = 200
	input.SetWidth(48)
	input.Focus()

	return Model{input: input}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case failedMsg:
		m.err = typedMsg.err
		m.checking = false

	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "enter":
			apiKey := strings.TrimSpace(m.input.Value())
			if apiKey == "" || m.checking {
				return m, nil
			}

			m.err = nil
			m.checking = true

			return m, checkAndSave(apiKey)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

// checkAndSave rejects a key the gateway does not recognise, so a typo is
// caught here rather than surfacing later as a failed request. The key is
// persisted before onboarding reports itself done, so the next run can skip
// this step.
func checkAndSave(apiKey string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.Config{
			APIKey:        strings.TrimSpace(apiKey),
			RouterBaseURL: config.DefaultRouterBaseURL,
		}

		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()

		// Anything other than a rejection (offline, gateway trouble) is not
		// the user's problem, so the key is accepted and the next request
		// reports the failure.
		if err := client.New(cfg).CheckAPIKey(ctx); errors.Is(err, client.ErrInvalidAPIKey) {
			return failedMsg{err: errors.New("That key was not recognised. Copy it again from https://app.requesty.ai/api-keys.")}
		}

		if err := config.Save(cfg); err != nil {
			return failedMsg{err: err}
		}

		return DoneMsg{Config: cfg}
	}
}

func (m Model) View(width, height int) string {
	// The panel's border and padding cost four columns. The input needs two
	// more for its prompt and one for the cursor sitting past the last rune.
	inner := min(width-4, 64)
	m.input.SetWidth(inner - 3)

	wrap := lipgloss.NewStyle().Width(inner)
	lines := []string{
		theme.Heading.Render("Welcome to Requesty"),
		wrap.Render(theme.Label.Render("One gateway for every model, in every tool you use, or app you build.")),
		text.LineSeparator,
		wrap.Render(theme.Body.Render("Paste an API key from https://app.requesty.ai/api-keys to begin.")),
		text.LineSeparator,
		m.input.View(),
		text.LineSeparator,
	}
	if m.checking {
		lines = append(lines, wrap.Render(theme.Label.Render("Checking key...")), "")
	}
	if m.err != nil {
		lines = append(lines, wrap.Render(theme.Bad.Render(m.err.Error())), "")
	}
	lines = append(lines, text.RenderFooterHintList(inner, [2]string{"enter", "continue"}, [2]string{"ctrl+c", "quit"}))

	body := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}
