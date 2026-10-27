// Package modelpicker is the dialog that asks which model a harness should
// use. It lists the managed policies the profile can route to on one tab and
// every model on another. Run shows it full screen on its own; Picker embeds
// it in another program, such as the dashboard's configure wizard.
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

// Options says what to pick for and what to prefer.
type Options struct {
	// Client fetches the two lists the picker shows, as the profile the
	// harness will run with.
	Client *client.Client
	// Harness is the display name shown in the title.
	Harness string
	// Preferred lists models in order of preference: what was picked last
	// time, or the harness's defaults. The first one the profile can route
	// to is returned without asking, or with Confirm set, is where the
	// cursor starts.
	Preferred []string
	// Confirm always shows the dialog, for when the user asked to choose.
	Confirm bool
	// Action names what enter does in the footer. Empty means "launch".
	Action string
}

// ChosenMsg reports that a model was chosen in a Picker.
type ChosenMsg struct {
	Model client.Model
}

// CancelledMsg reports that a Picker was closed without choosing.
type CancelledMsg struct{}

// Run returns the model to launch with. Unless Confirm is set, the first of
// Preferred that the profile can route to is returned without asking, and
// asked reports false. Otherwise the dialog is shown, drawn on stderr so the
// harness's own stdout is untouched.
func Run(ctx context.Context, opts Options) (chosen string, asked bool, err error) {
	if !opts.Confirm {
		if preferred := routablePreferred(ctx, opts); preferred != "" {
			return preferred, false, nil
		}
	}

	final, err := tea.NewProgram(newProgram(ctx, opts), tea.WithContext(ctx), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return "", true, fmt.Errorf("failed to run model picker: %w", err)
	}

	finished, ok := final.(program)
	if !ok || finished.chosen == "" {
		return "", true, ErrCancelled
	}

	return finished.chosen, true, nil
}

// routablePreferred is the first of Preferred that the profile can route to,
// or empty. Both lists come back filtered by the key's access list, so
// anything found is known to be permitted. A list that cannot be fetched
// counts as empty here; the dialog will show the error.
func routablePreferred(ctx context.Context, opts Options) string {
	if len(opts.Preferred) == 0 {
		return ""
	}

	policies, _ := opts.Client.ManagedPolicies(ctx)
	models, _ := opts.Client.Models(ctx)
	listed := slices.Concat(policies, models)
	for _, id := range opts.Preferred {
		if slices.ContainsFunc(listed, func(m client.Model) bool { return m.ID == id }) {
			return id
		}
	}

	return ""
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

// program is the full-screen dialog Run shows: a Picker centred in the
// terminal, quitting once it reports a choice or a cancel.
type program struct {
	picker Picker
	width  int
	height int
	chosen string
}

func newProgram(ctx context.Context, opts Options) program {
	return program{picker: New(ctx, opts), width: defaultWidth, height: defaultHeight}
}

func (p program) Init() tea.Cmd {
	return p.picker.Init()
}

func (p program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = max(typedMsg.Width-2*framePadX, 0)
		p.height = max(typedMsg.Height-2*framePadY, 0)
		return p, nil
	case ChosenMsg:
		p.chosen = typedMsg.Model.ID
		return p, tea.Quit
	case CancelledMsg:
		return p, tea.Quit
	case tea.KeyPressMsg:
		if typedMsg.String() == "ctrl+c" {
			return p, tea.Quit
		}
	}

	var cmd tea.Cmd
	p.picker, cmd = p.picker.Update(msg)

	return p, cmd
}

func (p program) View() tea.View {
	view := tea.NewView(frame.Render(p.picker.View(p.width, p.height)))
	view.AltScreen = true

	return view
}

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

// Picker is the dialog as a component: the host forwards it messages, draws
// its View, and is told the outcome through ChosenMsg or CancelledMsg.
type Picker struct {
	ctx    context.Context
	opts   Options
	lists  [tabCount]list
	tab    tab
	cursor int
	search textinput.Model
}

// New returns a Picker; its Init starts loading both lists.
func New(ctx context.Context, opts Options) Picker {
	search := textinput.New()
	search.Prompt = "Search: "
	search.Placeholder = "type to filter"
	search.CharLimit = 200
	// The blink command Focus returns is issued from Init, so it is safe to
	// discard here; what matters is that the box is ready to type into.
	search.Focus()

	return Picker{
		ctx:    ctx,
		opts:   opts,
		search: search,
	}
}

func (m Picker) Init() tea.Cmd {
	return tea.Batch(m.load(tabPolicies), m.load(tabModels), textinput.Blink)
}

func (m Picker) load(t tab) tea.Cmd {
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

func (m Picker) Update(msg tea.Msg) (Picker, tea.Cmd) {
	switch typedMsg := msg.(type) {
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

func (m Picker) updateKey(msg tea.KeyPressMsg) (Picker, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return CancelledMsg{} }
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
			chosen := filtered[m.cursor]
			return m, func() tea.Msg { return ChosenMsg{Model: chosen} }
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
func (m Picker) filtered() []client.Model {
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

// preselectedCursor is where the cursor goes on a fresh list: on the first of
// Preferred that is there, else the top.
func (m Picker) preselectedCursor() int {
	filtered := m.filtered()
	for _, id := range m.opts.Preferred {
		if i := slices.IndexFunc(filtered, func(m client.Model) bool { return m.ID == id }); i >= 0 {
			return i
		}
	}

	return 0
}

// View draws the dialog centred in width by height.
func (m Picker) View(width, height int) string {
	inner := max(min(width-8, maxWidth), minWidth)
	m.search.SetWidth(inner - lipgloss.Width(m.search.Prompt))

	lines := []string{
		text.RenderSplitHeaderSection("Choose a model", m.opts.Harness, inner),
		text.LineSeparator,
		m.tabs(),
		lipgloss.NewStyle().Width(inner).Render(theme.Muted.Render(m.tab.description())),
		text.LineSeparator,
		m.search.View(),
		text.LineSeparator,
		m.body(inner, height),
		text.LineSeparator,
		text.RenderFooterHintList(inner,
			[2]string{"tab", "switch list"},
			[2]string{"↑/↓", "move"},
			[2]string{"enter", m.action()},
			[2]string{"esc", "cancel"},
		),
	}

	panel := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func (m Picker) action() string {
	if m.opts.Action == "" {
		return "launch"
	}

	return m.opts.Action
}

func (m Picker) tabs() string {
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

func (m Picker) body(inner, height int) string {
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
		Height: max(min(height-chromeRows, maxRows), minRows),
		Style:  table.CellStyle(m.cursor),
	}.Render()
}
