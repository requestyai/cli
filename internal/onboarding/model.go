package onboarding

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/table"
	"github.com/requestyai/cli/internal/tui/ui/text"
)

const (
	pageWidth       = 64
	dialogMinWidth  = 24
	dialogMaxWidth  = 80
	groupListHeight = 8
	membersWidth    = 10

	framePadX     = 2
	framePadY     = 1
	defaultWidth  = 80 - 2*framePadX
	defaultHeight = 24 - 2*framePadY
)

var frame = lipgloss.NewStyle().Padding(framePadY, framePadX)

type dialogStep uint8

const (
	dialogSigningIn dialogStep = iota
	dialogChooseGroup
	dialogSaving
)

type dialogState struct {
	open         bool
	step         dialogStep
	cancel       context.CancelFunc
	authorizeURL string
	session      *session
	groupCursor  int
}

type authorizeURLMsg struct{ url string }

type signedInMsg struct {
	session *session
	err     error
}

type savedMsg struct {
	result result
	err    error
}

// model owns the complete interactive onboarding program.
type model struct {
	ctx    context.Context
	opts   Options
	dialog dialogState
	err    error
	width  int
	height int
	done   *config.Config
}

func newModel(ctx context.Context, opts Options) model {
	return model{
		ctx:    ctx,
		opts:   opts,
		width:  defaultWidth,
		height: defaultHeight,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(typedMsg.Width-2*framePadX, 0)
		m.height = max(typedMsg.Height-2*framePadY, 0)

	case authorizeURLMsg:
		if m.dialog.open && m.dialog.step == dialogSigningIn {
			m.dialog.authorizeURL = typedMsg.url
		}

	case signedInMsg:
		return m.onSignedIn(typedMsg)

	case savedMsg:
		if typedMsg.err != nil {
			return m.fail(typedMsg.err), nil
		}
		m.done = &typedMsg.result.Config
		return m, tea.Quit

	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.dialog.open {
				return m, tea.Quit
			}
		case "esc":
			if !m.dialog.open {
				return m, tea.Quit
			}
		}

		if m.dialog.open {
			return m.updateDialog(typedMsg)
		}
		if typedMsg.String() == "enter" {
			return m.signIn()
		}
	}

	return m, nil
}

func (m model) updateDialog(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch m.dialog.step {
	case dialogSigningIn:
		if msg.String() == "esc" {
			m.dialog.cancel()
			m.dialog = dialogState{}
		}
	case dialogChooseGroup:
		groups := m.dialog.session.groups
		switch msg.String() {
		case "esc":
			m.dialog = dialogState{}
		case "up", "k":
			if m.dialog.groupCursor > 0 {
				m.dialog.groupCursor--
			}
		case "down", "j":
			if m.dialog.groupCursor < len(groups)-1 {
				m.dialog.groupCursor++
			}
		case "enter":
			return m.save(&groups[m.dialog.groupCursor])
		}
	case dialogSaving:
		// Nothing to do but wait.
	}
	return m, nil
}

func (m model) signIn() (model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.dialog = dialogState{open: true, step: dialogSigningIn, cancel: cancel}
	m.err = nil

	opts := m.opts
	urls := make(chan string, 1)
	return m, tea.Batch(
		func() tea.Msg {
			defer close(urls)
			session, err := signIn(ctx, opts, func(url string) { urls <- url })
			return signedInMsg{session: session, err: err}
		},
		func() tea.Msg {
			url, ok := <-urls
			if !ok {
				return nil
			}
			return authorizeURLMsg{url: url}
		},
	)
}

func (m model) onSignedIn(msg signedInMsg) (model, tea.Cmd) {
	if !m.dialog.open || m.dialog.step != dialogSigningIn {
		return m, nil
	}
	if errors.Is(msg.err, context.Canceled) {
		m.dialog = dialogState{}
		return m, nil
	}
	if msg.err != nil {
		return m.fail(msg.err), nil
	}

	m.dialog.session = msg.session
	group, err := msg.session.chooseGroup()
	if errors.Is(err, errGroupChoiceRequired) {
		m.dialog.step = dialogChooseGroup
		m.dialog.groupCursor = 0
		return m, nil
	}
	if err != nil {
		return m.fail(err), nil
	}
	return m.save(group)
}

func (m model) save(group *client.Group) (model, tea.Cmd) {
	m.dialog.step = dialogSaving
	session := m.dialog.session
	ctx := m.ctx
	return m, func() tea.Msg {
		result, err := session.createKey(ctx, group)
		return savedMsg{result: result, err: err}
	}
}

func (m model) fail(err error) model {
	m.dialog = dialogState{}
	m.err = err
	return m
}

func (m model) View() tea.View {
	view := tea.NewView(frame.Render(m.pageView()))
	view.AltScreen = true
	return view
}

func (m model) pageView() string {
	if m.dialog.open {
		return m.dialogView()
	}

	inner := max(min(m.width-4, pageWidth), dialogMinWidth)
	wrap := lipgloss.NewStyle().Width(inner)
	lines := []string{
		theme.Heading.Render("Welcome to Requesty"),
		wrap.Render(theme.Label.Render("One gateway for every model, in every tool you use, or app you build.")),
		text.LineSeparator,
		wrap.Render(theme.Body.Render(fmt.Sprintf(
			"Sign in with your browser to get started. This creates an API key named %q in your Requesty account and saves it to %s.",
			defaultKeyName(), config.DisplayPath()))),
	}
	if m.opts.Harness != "" {
		lines = append(lines, wrap.Render(theme.Body.Render(m.opts.Harness+" starts as soon as the key is saved.")))
	}
	lines = append(lines,
		text.LineSeparator,
		wrap.Render(theme.Muted.Render("Working over SSH, or already have a key? Quit and run `requesty login --api-key <key>` instead.")),
		text.LineSeparator,
	)
	if m.err != nil {
		lines = append(lines, wrap.Render(theme.Bad.Render(m.err.Error())), text.LineSeparator)
	}
	lines = append(lines, text.RenderFooterHintList(inner, [2]string{"enter", "sign in"}, [2]string{"q/esc", "quit"}))

	body := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

type dialogPage struct {
	title string
	body  string
	hints [][2]string
}

func (m model) dialogView() string {
	inner := max(min(m.width-8, dialogMaxWidth), dialogMinWidth)
	page := m.dialogPage(inner)
	lines := []string{
		text.RenderSplitHeaderSection(page.title, "", inner),
		text.LineSeparator,
		page.body,
		text.LineSeparator,
		text.RenderFooterHintList(inner, page.hints...),
	}
	body := theme.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

func (m model) dialogPage(inner int) dialogPage {
	wrap := lipgloss.NewStyle().Width(inner)
	switch m.dialog.step {
	case dialogSigningIn:
		body := []string{wrap.Render(theme.Body.Render("Waiting for you to finish signing in in your browser…"))}
		if m.dialog.authorizeURL != "" {
			body = append(body,
				text.LineSeparator,
				wrap.Render(theme.Muted.Render("If it did not open, visit this address:")),
				wrap.Render(theme.Accent.Render(m.dialog.authorizeURL)),
			)
		}
		return dialogPage{
			title: "Sign in with your browser",
			body:  lipgloss.JoinVertical(lipgloss.Left, body...),
			hints: [][2]string{{"esc", "cancel"}},
		}
	case dialogChooseGroup:
		return dialogPage{
			title: "Choose a group for the API key",
			body:  m.groupTable(inner).Render(),
			hints: [][2]string{{"↑/↓", "move"}, {"enter", "choose"}, {"esc", "cancel"}},
		}
	case dialogSaving:
		return dialogPage{
			title: "Saving your API key",
			body:  wrap.Render(theme.Muted.Render("Creating the key and writing " + config.DisplayPath() + "…")),
			hints: [][2]string{{"ctrl+c", "quit"}},
		}
	default:
		return dialogPage{}
	}
}

func (m model) groupTable(inner int) table.Table {
	groups := m.dialog.session.groups
	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, []string{group.Name, fmt.Sprintf("%d", group.MembersCount)})
	}
	return table.Table{
		Cols: []table.Column{
			{Title: "GROUP", Width: inner - 2 - membersWidth, Align: table.Left},
			{Title: "MEMBERS", Width: membersWidth, Align: table.Right},
		},
		Rows:   rows,
		Cursor: m.dialog.groupCursor,
		Height: groupListHeight,
		Style:  table.CellStyle(m.dialog.groupCursor),
	}
}
