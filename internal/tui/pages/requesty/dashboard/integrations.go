package dashboard

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/table"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

const (
	checkWidth  = 4
	configWidth = 40
	statusWidth = 10
)

type integrationWizardStep uint8

const (
	integrationModelWizardStep integrationWizardStep = iota
	integrationModeWizardStep
)

// integrationWizardState tracks progress and selections in the configuration wizard.
type integrationWizardState struct {
	open bool
	step integrationWizardStep

	options     harnesses.ConfigureOptions
	models      []client.Model
	modelCursor int
	modeCursor  int

	modelsErr    error
	configureErr error
	configuring  bool
}

// integrationWizardPage contains the content and controls rendered for one wizard step.
type integrationWizardPage struct {
	title      string
	body       string
	enterHint  string
	escapeHint string
}

// integrationItem pairs a registered harness with its latest detected status.
type integrationItem struct {
	harness harnesses.Harness
	status  harnesses.Status
	err     error
}

// integrationsLoadedMsg carries refreshed harness statuses back to the page.
type integrationsLoadedMsg struct {
	items []integrationItem
	err   error
}

// integrationModelsLoadedMsg carries the model catalogue back to the wizard.
type integrationModelsLoadedMsg struct {
	models []client.Model
	err    error
}

// integrationConfiguredMsg carries the result of configuring the selected harness
type integrationConfiguredMsg struct {
	err error
}

// integrationState holds the integrations list and configuration wizard state.
type integrationState struct {
	client *client.Client
	config config.Config

	items      []integrationItem
	cursor     int
	refreshing bool
	loadErr    error

	wizard integrationWizardState
}

func newIntegrationState(client *client.Client, config config.Config) integrationState {
	return integrationState{client: client, config: config}
}

func (m integrationState) init() tea.Cmd {
	return m.load
}

func (m integrationState) load() tea.Msg {
	registered, err := harnesses.Harnesses(m.config)
	if err != nil {
		return integrationsLoadedMsg{err: err}
	}

	items := make([]integrationItem, 0, len(registered))
	for _, harness := range registered {
		status, err := harness.Status()
		items = append(items, integrationItem{harness: harness, status: status, err: err})
	}

	slices.SortStableFunc(items, func(a, b integrationItem) int {
		switch {
		case a.status.Executable == b.status.Executable:
			return 0
		case a.status.Executable:
			return -1
		default:
			return 1
		}
	})

	return integrationsLoadedMsg{items: items}
}

func (m integrationState) refresh() tea.Cmd {
	if m.refreshing {
		return nil
	}
	m.refreshing = true
	return m.load
}

func (m integrationState) loadModels() tea.Msg {
	models, err := m.client.Models(context.Background())
	if err == nil {
		slices.SortFunc(models, func(a, b client.Model) int {
			return strings.Compare(a.ID, b.ID)
		})
	}
	return integrationModelsLoadedMsg{models: models, err: err}
}

func (m integrationState) configure(harness harnesses.Harness, options harnesses.ConfigureOptions) tea.Cmd {
	return func() tea.Msg {
		err := harness.Configure(options)
		return integrationConfiguredMsg{err: err}
	}
}

func (m integrationState) update(msg tea.Msg) (integrationState, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case integrationsLoadedMsg:
		selected := ""
		if m.cursor >= 0 && m.cursor < len(m.items) {
			selected = m.items[m.cursor].harness.Name()
		}
		m.items = typedMsg.items
		m.loadErr = typedMsg.err
		m.refreshing = false
		m.cursor = 0
		for i := range m.items {
			if m.items[i].harness.Name() == selected {
				m.cursor = i
				break
			}
		}

	case integrationModelsLoadedMsg:
		m.wizard.models, m.wizard.modelsErr = typedMsg.models, typedMsg.err
		m.wizard.modelCursor = 0

	case integrationConfiguredMsg:
		m.wizard.configuring = false
		if typedMsg.err != nil {
			m.wizard.configureErr = fmt.Errorf("could not configure harness: %w", typedMsg.err)
			return m, nil
		}
		m.wizard = integrationWizardState{}
		m.refreshing = true
		return m, m.load

	case tea.KeyPressMsg:
		if m.wizard.open {
			return m.updateWizard(typedMsg)
		}
		switch typedMsg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ", "space":
			if m.canConfigure() {
				m.wizard = integrationWizardState{open: true}
				return m, m.loadModels
			}
		}
	}

	return m, nil
}

func (m integrationState) updateWizard(msg tea.KeyPressMsg) (integrationState, tea.Cmd) {
	if m.wizard.configuring {
		return m, nil
	}

	switch m.wizard.step {
	case integrationModelWizardStep:
		return m.updateModelStep(msg)
	case integrationModeWizardStep:
		return m.updateModeStep(msg)
	default:
		return m, nil
	}
}

func (m integrationState) updateModelStep(msg tea.KeyPressMsg) (integrationState, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.wizard = integrationWizardState{}
	case "up", "k":
		if m.wizard.modelCursor > 0 {
			m.wizard.modelCursor--
		}
	case "down", "j":
		if m.wizard.modelCursor < len(m.wizard.models)-1 {
			m.wizard.modelCursor++
		}
	case "enter":
		if m.wizard.modelsErr == nil &&
			len(m.wizard.models) > 0 &&
			m.cursor < len(m.items) {
			m.wizard.options.Model = m.wizard.models[m.wizard.modelCursor].ID
			m.wizard.step = integrationModeWizardStep
			m.wizard.modeCursor = 0
			m.wizard.configureErr = nil
		}
	}

	return m, nil
}

func (m integrationState) updateModeStep(msg tea.KeyPressMsg) (integrationState, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.wizard.step = integrationModelWizardStep
		m.wizard.options.Model = ""
		m.wizard.modeCursor = 0
		m.wizard.configureErr = nil
	case "up", "k":
		if m.wizard.modeCursor > 0 {
			m.wizard.modeCursor--
		}
	case "down", "j":
		if m.wizard.modeCursor < 1 {
			m.wizard.modeCursor++
		}
	case "enter":
		if m.wizard.options.Model != "" && m.cursor < len(m.items) {
			m.wizard.options.Overwrite = m.wizard.modeCursor == 1
			m.wizard.configuring = true
			m.wizard.configureErr = nil
			return m, m.configure(m.items[m.cursor].harness, m.wizard.options)
		}
	}

	return m, nil
}

func (m integrationState) canConfigure() bool {
	if m.refreshing || m.cursor < 0 || m.cursor >= len(m.items) {
		return false
	}

	selected := m.items[m.cursor]
	return selected.err == nil && selected.status.Executable
}

func (m integrationState) view(width, height int) string {
	if m.items == nil {
		return theme.Muted.Render("Loading harnesses…")
	}

	t := m.table(width, max(height-13, 1))
	configured := 0
	for _, item := range m.items {
		if item.status.Configured {
			configured++
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		text.RenderSplitHeaderSection(
			"harnesses on this machine",
			fmt.Sprintf("%d of %d routing through requesty", configured, len(m.items)),
			t.Width(),
		),
		text.LineSeparator,
		t.Render(),
		text.LineSeparator,
		m.detail(t.Width()),
		text.LineSeparator,
		text.RenderFooterHintList(
			t.Width(),
			[2]string{"↑/↓", "move"},
			[2]string{"space", "configure"},
			[2]string{"r", "refresh"},
			[2]string{"q/esc", "quit"},
		),
	)
}

func (m integrationState) table(width, rows int) table.Table {
	nameWidth := max(width-checkWidth-configWidth-statusWidth-2, 18)
	body := make([][]string, 0, len(m.items))
	for _, item := range m.items {
		check := "[ ]"
		if item.status.Configured {
			check = "[✓]"
		}

		config := ""
		if len(item.status.Files) > 0 {
			config = item.status.Files[0]
		}

		status := "inactive"
		if item.status.Configured {
			status = "active"
		}

		body = append(body, []string{check, item.harness.Name(), config, status})
	}

	return table.Table{
		Cols: []table.Column{
			{Title: "", Width: checkWidth, Align: table.Left},
			{Title: "HARNESS", Width: nameWidth, Align: table.Left},
			{Title: "CONFIG", Width: configWidth, Align: table.Left},
			{Title: "STATUS", Width: statusWidth, Align: table.Left},
		},
		Rows:   body,
		Cursor: m.cursor,
		Height: rows,
		Style:  m.cellStyle,
	}
}

func (m integrationState) cellStyle(row, col int) lipgloss.Style {
	if row < 0 {
		return theme.Label.Bold(true)
	}

	style := theme.Body
	if !m.items[row].status.Executable || m.items[row].err != nil {
		style = theme.Muted
	} else if m.items[row].status.Configured && (col == 0 || col == 3) {
		style = theme.Good
	}

	if row == m.cursor {
		style = style.Background(theme.BgSelect)
	}
	return style
}

func (m integrationState) detail(width int) string {
	if m.loadErr != nil {
		return theme.Panel.Render(theme.Bad.Render("Could not load harnesses: " + m.loadErr.Error()))
	}
	if len(m.items) == 0 {
		return theme.Panel.Render(theme.Muted.Render("No harnesses found"))
	}

	selected := m.items[m.cursor]
	inner := max(width-4, 1)
	wrap := lipgloss.NewStyle().Width(inner)
	files := strings.Join(selected.status.Files, " · ")
	lines := []string{
		text.RenderSplitLine(
			theme.Heading.Render(selected.harness.Name()),
			theme.Muted.Render(files),
			inner,
		),
	}

	for _, description := range selected.harness.Description() {
		lines = append(lines, wrap.Render(theme.Body.Render(description)))
	}
	if selected.err != nil {
		lines = append(lines, theme.Bad.Render("Could not read status: "+selected.err.Error()))
	}
	if m.canConfigure() {
		lines = append(lines, text.LineSeparator, theme.Key.Render("space")+" "+theme.Footer.Render("to configure"))
	}
	if m.refreshing {
		lines = append(lines, theme.Muted.Render("Refreshing…"))
	}

	return theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m integrationState) wizardView(width, height int) string {
	inner := max(min(width-8, 80), 24)
	page := m.wizardPage(inner, height)
	if m.wizard.configuring {
		page.body = theme.Muted.Render("Configuring harness…")
	}

	lines := []string{
		text.RenderSplitHeaderSection(
			page.title,
			m.items[m.cursor].harness.Name(),
			inner,
		),
		text.LineSeparator,
		page.body,
	}

	if m.wizard.configureErr != nil {
		lines = append(lines, text.LineSeparator, theme.Bad.Render(m.wizard.configureErr.Error()))
	}

	lines = append(lines,
		text.LineSeparator,
		text.RenderFooterHintList(
			inner,
			[2]string{"↑/↓", "move"},
			[2]string{"enter", page.enterHint},
			[2]string{"esc", page.escapeHint},
		),
	)

	body := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

func (m integrationState) wizardPage(inner, height int) integrationWizardPage {
	switch m.wizard.step {
	case integrationModelWizardStep:
		return m.modelStepPage(inner, height)
	case integrationModeWizardStep:
		return m.modeStepPage(inner)
	default:
		return integrationWizardPage{}
	}
}

func (m integrationState) modelStepPage(inner, height int) integrationWizardPage {
	page := integrationWizardPage{
		title:      "Choose a Requesty model",
		enterHint:  "continue",
		escapeHint: "cancel",
	}

	switch {
	case m.wizard.modelsErr != nil:
		page.body = theme.Bad.Render(m.wizard.modelsErr.Error())
	case m.wizard.models == nil:
		page.body = theme.Muted.Render("Loading models…")
	case len(m.wizard.models) == 0:
		page.body = theme.Muted.Render("No models available")
	default:
		rows := make([][]string, 0, len(m.wizard.models))
		for _, model := range m.wizard.models {
			rows = append(rows, []string{model.ID})
		}
		page.body = table.Table{
			Cols: []table.Column{
				{Title: "MODEL", Width: inner - 2, Align: table.Left},
			},
			Rows:   rows,
			Cursor: m.wizard.modelCursor,
			Height: max(height-10, 3),
			Style:  table.CellStyle(m.wizard.modelCursor),
		}.Render()
	}

	return page
}

func (m integrationState) modeStepPage(inner int) integrationWizardPage {
	body := table.Table{
		Cols: []table.Column{
			{Title: "CONFIGURATION", Width: inner - 2, Align: table.Left},
		},
		Rows: [][]string{
			{"Merge existing config files (recommended)"},
			{"Overwrite config files"},
		},
		Cursor: m.wizard.modeCursor,
		Height: 2,
		Style:  table.CellStyle(m.wizard.modeCursor),
	}.Render()

	description := "Preserve existing settings and update Requesty values."
	if m.wizard.modeCursor == 1 {
		description = "Replace existing config files with Requesty settings."
	}

	return integrationWizardPage{
		title:      "Choose how to configure the harness",
		body:       lipgloss.JoinVertical(lipgloss.Left, body, text.LineSeparator, theme.Muted.Render(description)),
		enterHint:  "configure",
		escapeHint: "back",
	}
}
