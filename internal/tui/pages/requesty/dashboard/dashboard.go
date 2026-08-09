package dashboard

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

const integrationMinHeight = 10

// Model is the single Requesty page. Integrations are the primary section;
// usage is added above them when the terminal has room.
type Model struct {
	usage        usageState
	integrations integrationState
}

func New(client *client.Client, config config.Config) Model {
	return Model{
		usage:        newUsageState(client),
		integrations: newIntegrationState(client, config),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.usage.load, m.integrations.init())
}

func (m Model) ModalOpen() bool {
	return m.integrations.wizard.open
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	m.usage.update(msg)

	switch typedMsg := msg.(type) {
	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "r":
			if !m.ModalOpen() {
				return m, tea.Batch(m.usage.refresh(), m.integrations.refresh())
			}
		default:
			if !m.ModalOpen() && m.usage.selectMetric(typedMsg.String()) {
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.integrations, cmd = m.integrations.update(msg)
	return m, cmd
}

func (m Model) View(width, height int) string {
	if m.ModalOpen() {
		return m.integrations.wizardView(width, height)
	}

	// Give usage only the space left after reserving the integrations minimum and separator.
	usageBudget := max(height-integrationMinHeight-1, 0)
	usage := m.usage.view(width, usageBudget)
	if usage == "" {
		return m.integrations.view(width, height)
	}

	// Give integrations the remaining height after the rendered usage section and separator.
	integrationHeight := max(height-lipgloss.Height(usage)-1, 1)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		usage,
		text.LineSeparator,
		m.integrations.view(width, integrationHeight),
	)
}
