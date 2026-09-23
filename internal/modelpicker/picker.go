// Package modelpicker is the full-screen dialog that asks which model a
// harness should launch with. It lists the managed policies the profile can
// route to on one tab and every model on another.
package modelpicker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/table"
	"github.com/requestyai/cli/internal/tui/ui/text"
	"github.com/requestyai/cli/internal/util"
)

// ErrCancelled reports that the dialog was closed without choosing.
var ErrCancelled = errors.New("no model chosen")

// Options says what to pick for and what to suggest.
type Options struct {
	// Client fetches the two lists the picker shows, as the profile the
	// harness will run with.
	Client *client.Client
	// Harness is the display name shown in the title.
	Harness string
	// Preselect is the model the cursor starts on when it is listed: the
	// harness's recommended default, or what was picked last time.
	Preselect string
}

// Run shows the picker and returns the chosen model id. It draws on stderr so
// the harness's own stdout is untouched.
func Run(ctx context.Context, opts Options) (string, error) {
	final, err := tea.NewProgram(newModel(ctx, opts), tea.WithContext(ctx), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return "", fmt.Errorf("failed to run model picker: %w", err)
	}

	finished, ok := final.(model)
	if !ok || finished.chosen == "" {
		return "", ErrCancelled
	}

	return finished.chosen, nil
}

const (
	minWidth     = 40
	maxWidth     = 96
	contextWidth = 9
	priceWidth   = 9
	minRows      = 3
	maxRows      = 15
	// chromeRows is how many lines the dialog spends around the table.
	chromeRows = 12

	framePadX     = 2
	framePadY     = 1
	defaultWidth  = 80 - 2*framePadX
	defaultHeight = 24 - 2*framePadY
)

var frame = lipgloss.NewStyle().Padding(framePadY, framePadX)

type tab uint8

const (
	tabPolicies tab = iota
	tabModels
	tabCount
)

func (t tab) title() string {
	if t == tabPolicies {
		return "Policies"
	}

	return "Models"
}

func (t tab) description() string {
	if t == tabPolicies {
		return "Managed policies: one name per model, routed across providers."
	}

	return "Every model and organization policy this profile can route to."
}

// list is one tab's contents. loaded distinguishes an empty list from one
// still on its way.
type list struct {
	models []client.Model
	loaded bool
	err    error
}

type loadedMsg struct {
	tab    tab
	models []client.Model
	err    error
}

type model struct {
	ctx    context.Context
	opts   Options
	lists  [tabCount]list
	tab    tab
	cursor int
	search textinput.Model
	width  int
	height int
	chosen string
}

func newModel(ctx context.Context, opts Options) model {
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "type to filter"
	search.CharLimit = 200
	// The blink command Focus returns is issued from Init, so it is safe to
	// discard here; what matters is that the box is ready to type into.
	search.Focus()

	return model{
		ctx:    ctx,
		opts:   opts,
		search: search,
		width:  defaultWidth,
		height: defaultHeight,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.load(tabPolicies), m.load(tabModels), textinput.Blink)
}

func (m model) load(t tab) tea.Cmd {
	ctx, apiClient := m.ctx, m.opts.Client
	return func() tea.Msg {
		var models []client.Model
		var err error
		if t == tabPolicies {
			models, err = apiClient.ManagedPolicies(ctx)
		} else {
			models, err = apiClient.Models(ctx)
		}
		slices.SortFunc(models, func(a, b client.Model) int {
			return strings.Compare(a.ID, b.ID)
		})
		return loadedMsg{tab: t, models: models, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(typedMsg.Width-2*framePadX, 0)
		m.height = max(typedMsg.Height-2*framePadY, 0)

	case loadedMsg:
		m.lists[typedMsg.tab] = list{models: typedMsg.models, loaded: true, err: typedMsg.err}
		// Someone with no managed policies, or whose list failed to load,
		// should not land on an empty tab.
		if typedMsg.tab == tabPolicies && m.tab == tabPolicies && len(typedMsg.models) == 0 {
			m.tab = tabModels
		}
		if typedMsg.tab == m.tab {
			m.cursor = m.preselectedCursor()
		}

	case tea.KeyPressMsg:
		return m.updateKey(typedMsg)
	}

	return m, nil
}

func (m model) updateKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.tab = (m.tab + 1) % tabCount
		m.cursor = m.preselectedCursor()
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.filtered())-1 {
			m.cursor++
		}
	case "enter":
		if filtered := m.filtered(); len(filtered) > 0 {
			m.chosen = filtered[m.cursor].ID
			return m, tea.Quit
		}
	default:
		previous := m.search.Value()
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		if m.search.Value() != previous {
			m.cursor = 0
		}
		return m, cmd
	}

	return m, nil
}

// filtered is the current tab's list narrowed by the search box.
func (m model) filtered() []client.Model {
	models := m.lists[m.tab].models
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	if query == "" {
		return models
	}

	matches := make([]client.Model, 0, len(models))
	for _, model := range models {
		if strings.Contains(strings.ToLower(model.ID), query) {
			matches = append(matches, model)
		}
	}

	return matches
}

// preselectedCursor is where the cursor goes on a fresh list: on Preselect
// when it is there, else the top.
func (m model) preselectedCursor() int {
	if m.opts.Preselect == "" {
		return 0
	}
	for i, model := range m.filtered() {
		if model.ID == m.opts.Preselect {
			return i
		}
	}

	return 0
}

func (m model) View() tea.View {
	inner := max(min(m.width-8, maxWidth), minWidth)
	m.search.SetWidth(inner - lipgloss.Width(m.search.Prompt))

	lines := []string{
		text.RenderSplitHeaderSection("Choose a model", m.opts.Harness, inner),
		text.LineSeparator,
		m.tabs(),
		lipgloss.NewStyle().Width(inner).Render(theme.Muted.Render(m.tab.description())),
		text.LineSeparator,
		m.search.View(),
		text.LineSeparator,
		m.body(inner),
		text.LineSeparator,
		text.RenderFooterHintList(inner,
			[2]string{"tab", "switch list"},
			[2]string{"↑/↓", "move"},
			[2]string{"enter", "launch"},
			[2]string{"esc", "cancel"},
		),
	}

	panel := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	view := tea.NewView(frame.Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)))
	view.AltScreen = true

	return view
}

func (m model) tabs() string {
	pills := make([]string, 0, tabCount)
	for t := tabPolicies; t < tabCount; t++ {
		label := t.title()
		if l := m.lists[t]; l.loaded && l.err == nil {
			label = fmt.Sprintf("%s (%d)", label, len(l.models))
		}
		style := theme.PillOff
		if t == m.tab {
			style = theme.Pill
		}
		pills = append(pills, style.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, pills...)
}

func (m model) body(inner int) string {
	current := m.lists[m.tab]
	filtered := m.filtered()
	switch {
	case current.err != nil:
		return theme.Bad.Render(fmt.Sprintf("Could not load %s: %s", strings.ToLower(m.tab.title()), current.err))
	case !current.loaded:
		return theme.Muted.Render(fmt.Sprintf("Loading %s…", strings.ToLower(m.tab.title())))
	case len(current.models) == 0:
		return theme.Muted.Render(fmt.Sprintf("No %s available to this profile", strings.ToLower(m.tab.title())))
	case len(filtered) == 0:
		return theme.Muted.Render(fmt.Sprintf("No %s match your search", strings.ToLower(m.tab.title())))
	}

	rows := make([][]string, 0, len(filtered))
	for _, model := range filtered {
		rows = append(rows, []string{
			model.ID,
			util.FormatTokens(model.ContextWindow),
			util.FormatPrice(model.InputPrice),
			util.FormatPrice(model.OutputPrice),
		})
	}

	return table.Table{
		Cols: []table.Column{
			{Title: "MODEL", Width: inner - 2 - contextWidth - 2*priceWidth, Align: table.Left},
			{Title: "CONTEXT", Width: contextWidth, Align: table.Right},
			{Title: "IN /1M", Width: priceWidth, Align: table.Right},
			{Title: "OUT /1M", Width: priceWidth, Align: table.Right},
		},
		Rows:   rows,
		Cursor: m.cursor,
		Height: max(min(m.height-chromeRows, maxRows), minRows),
		Style:  table.CellStyle(m.cursor),
	}.Render()
}
