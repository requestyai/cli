package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/pages/requesty/dashboard"
)

// RequestyApp is the main UI once onboarding is out of the way.
type RequestyApp struct {
	dashboard dashboard.Model

	width  int
	height int
}

func NewRequestyApp(cfg config.Config) RequestyApp {
	client := client.New(cfg)
	return RequestyApp{
		dashboard: dashboard.New(client, cfg),
		width:     contentWidth,
		height:    contentHeight,
	}
}

func (a RequestyApp) Init() tea.Cmd {
	return a.dashboard.Init()
}

func (a RequestyApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = typedMsg.Width, typedMsg.Height

	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "q":
			return a, tea.Quit
		case "esc":
			if !a.dashboard.ModalOpen() {
				return a, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	a.dashboard, cmd = a.dashboard.Update(msg)
	return a, cmd
}

func (a RequestyApp) View() tea.View {
	return tea.NewView(a.dashboard.View(a.width, a.height))
}
