package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/tui/pages/requesty/models"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

// tabs are the app's sections, in the order they appear in the tab bar.
// Integrations will join Models here.
var tabs = []string{"Models"}

// tabBarHeight is the tab bar plus the blank line under it.
const tabBarHeight = 2

// RequestyApp is the main UI once onboarding is out of the way: a tab bar
// above the selected page.
type RequestyApp struct {
	tab    int
	models models.Model

	width  int
	height int
}

func NewRequestyApp(c *client.Client) RequestyApp {
	return RequestyApp{
		models: models.New(c),
		width:  contentWidth,
		height: contentHeight,
	}
}

func (a RequestyApp) Init() tea.Cmd {
	return a.models.Init()
}

func (a RequestyApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return a, tea.Quit
		case "tab":
			a.tab = (a.tab + 1) % len(tabs)
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.models, cmd = a.models.Update(msg)

	return a, cmd
}

func (a RequestyApp) View() tea.View {
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		a.tabBar(),
		text.LineSeparator,
		a.models.View(a.width, a.height-tabBarHeight),
	))
}

func (a RequestyApp) tabBar() string {
	rendered := make([]string, 0, len(tabs))
	for i, name := range tabs {
		style := theme.TabOff
		if i == a.tab {
			style = theme.TabOn
		}
		rendered = append(rendered, style.Render(name))
	}

	return strings.Join(rendered, " ")
}
