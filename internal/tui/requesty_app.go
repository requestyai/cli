package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/pages/requesty/integrations"
	"github.com/requestyai/cli/internal/tui/pages/requesty/models"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

// tabs are the app's sections, in the order they appear in the tab bar.
var tabs = []string{"Models", "Integrations"}

// tabBarHeight is the tab bar plus the blank line under it.
const tabBarHeight = 2

// RequestyApp is the main UI once onboarding is out of the way: a tab bar
// above the selected page.
type RequestyApp struct {
	tab          int
	models       models.Model
	integrations integrations.Model

	width  int
	height int
}

func NewRequestyApp(cfg config.Config) RequestyApp {
	client := client.New(cfg)
	return RequestyApp{
		models:       models.New(client),
		integrations: integrations.New(client, cfg),
		width:        contentWidth,
		height:       contentHeight,
	}
}

func (a RequestyApp) Init() tea.Cmd {
	return tea.Batch(a.models.Init(), a.integrations.Init())
}

func (a RequestyApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q":
			return a, tea.Quit
		case "esc":
			if a.tab != 1 || !a.integrations.ModalOpen() {
				return a, tea.Quit
			}
		case "tab":
			a.tab = (a.tab + 1) % len(tabs)
			return a, nil
		}
	}

	// Key presses belong to the visible page. Other messages may be results
	// from either page's Init command, so both pages must get a chance to
	// consume them even when they are not currently visible.
	if _, isKeyPress := msg.(tea.KeyPressMsg); isKeyPress {
		var cmd tea.Cmd
		switch a.tab {
		case 0:
			a.models, cmd = a.models.Update(msg)
		case 1:
			a.integrations, cmd = a.integrations.Update(msg)
		}
		return a, cmd
	}

	var modelsCmd, integrationsCmd tea.Cmd
	a.models, modelsCmd = a.models.Update(msg)
	a.integrations, integrationsCmd = a.integrations.Update(msg)
	return a, tea.Batch(modelsCmd, integrationsCmd)
}

func (a RequestyApp) View() tea.View {
	var page string
	switch a.tab {
	case 0:
		page = a.models.View(a.width, a.height-tabBarHeight)
	case 1:
		page = a.integrations.View(a.width, a.height-tabBarHeight)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		a.tabBar(),
		text.LineSeparator,
		page,
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
