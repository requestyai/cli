package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/tui/pages/onboarding/apikey"
)

// OnboardingApp collects the configuration needed to enter RequestyApp.
type OnboardingApp struct {
	apiKey apikey.Model
	width  int
	height int
}

func NewOnboardingApp() OnboardingApp {
	return OnboardingApp{
		apiKey: apikey.New(),
		width:  contentWidth,
		height: contentHeight,
	}
}

func (a OnboardingApp) Init() tea.Cmd {
	return a.apiKey.Init()
}

func (a OnboardingApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	}

	var cmd tea.Cmd
	a.apiKey, cmd = a.apiKey.Update(msg)

	return a, cmd
}

func (a OnboardingApp) View() tea.View {
	return tea.NewView(a.apiKey.View(a.width, a.height))
}
