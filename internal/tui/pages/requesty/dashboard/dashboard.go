package dashboard

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/card"
	"github.com/requestyai/cli/internal/tui/ui/text"
	"github.com/requestyai/cli/internal/util"
)

const integrationMinHeight = 13

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
	return m.integrations.pickerOpen
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
		}
	}

	var cmd tea.Cmd
	m.integrations, cmd = m.integrations.update(msg)
	return m, cmd
}

func (m Model) View(width, height int) string {
	if m.ModalOpen() {
		return m.integrations.pickerView(width, height)
	}

	// Give usage only the space left after reserving the integrations minimum and separator.
	usageBudget := max(height-integrationMinHeight-1, 0)
	usage := m.usageView(width, usageBudget)
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

func (m Model) usageView(width, height int) string {
	if height <= 0 {
		return ""
	}
	if m.usage.err != nil && !m.usage.loaded {
		return theme.Bad.Render("Could not load usage: " + m.usage.err.Error())
	}

	cards := m.usageCards(width)
	if lipgloss.Height(cards) > height {
		return ""
	}

	header := text.RenderSplitLine("", theme.Muted.Render("last 30 days"), width)
	if lipgloss.Height(cards)+lipgloss.Height(header) > height {
		return cards
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, cards)
}

func (m Model) usageCards(width int) string {
	values := [3]string{"…", "…", "…"}
	if m.usage.loaded {
		values = [3]string{
			util.FormatSpend(m.usage.totals.Spend),
			util.FormatCount(m.usage.totals.Requests),
			util.FormatCount(m.usage.totals.Tokens),
		}
	}

	if width < 42 {
		line := fmt.Sprintf("Spend %s · Requests %s · Tokens %s", values[0], values[1], values[2])
		if m.usage.refreshing {
			line += " · refreshing"
		}
		return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(line)
	}

	gaps := 2
	baseWidth := (width - gaps) / 3
	lastWidth := width - gaps - 2*baseWidth
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		card.Render("SPEND", values[0], baseWidth),
		text.SpaceSeparator,
		card.Render("REQUESTS", values[1], baseWidth),
		text.SpaceSeparator,
		card.Render("TOKENS", values[2], lastWidth),
	)
}
