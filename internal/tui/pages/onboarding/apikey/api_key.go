package apikey

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

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
	input textinput.Model
	err   error
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

	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "enter":
			apiKey := strings.TrimSpace(m.input.Value())
			if apiKey == "" {
				return m, nil
			}

			return m, save(apiKey)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

// save persists the key before onboarding reports itself done, so the next
// run can skip this step.
func save(apiKey string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.Config{
			APIKey:        apiKey,
			RouterBaseURL: config.DefaultRouterBaseURL,
			APIBaseURL:    config.DefaultAPIBaseURL,
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
	if m.err != nil {
		lines = append(lines, wrap.Render(theme.Bad.Render(m.err.Error())), "")
	}
	lines = append(lines, text.RenderFooterHintList(inner, [2]string{"enter", "continue"}, [2]string{"ctrl+c", "quit"}))

	body := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}
