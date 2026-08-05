package models

import (
	"context"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/table"
	"github.com/requestyai/cli/internal/tui/ui/text"
	"github.com/requestyai/cli/internal/util"
)

// chrome is the number of lines the page spends on everything that is not a
// table row: heading, two spacers, column titles, rule and footer.
const chrome = 6

// loadedMsg carries the catalogue back from the API.
type loadedMsg struct {
	models []client.Model
	err    error
}

// Model is the catalogue page. A nil model list with no error means the
// fetch is still in flight.
type Model struct {
	client *client.Client
	models []client.Model
	cursor int
	err    error
}

func New(c *client.Client) Model {
	return Model{client: c}
}

func (m Model) Init() tea.Cmd {
	return m.load
}

func (m Model) load() tea.Msg {
	models, err := m.client.Models(context.Background())
	if err != nil {
		return loadedMsg{err: err}
	}
	slices.SortFunc(models, func(a, b client.Model) int {
		return strings.Compare(a.ID, b.ID)
	})

	return loadedMsg{models: models}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.models, m.err = msg.models, msg.err

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m Model) View(width, height int) string {
	switch {
	case m.err != nil:
		return theme.Bad.Render("Could not load models: " + m.err.Error())
	case m.models == nil:
		return theme.Muted.Render("Loading models…")
	}

	t := m.table(width, height-chrome)

	return lipgloss.JoinVertical(lipgloss.Left,
		text.RenderSplitHeaderSection("Catalogue", strconv.Itoa(len(m.models))+" models · prices per 1M tokens", t.Width()),
		text.LineSeparator,
		t.Render(),
		text.LineSeparator,
		text.RenderFooterHintList([2]string{"↑/↓", "select"}, [2]string{"tab", "switch"}, [2]string{"q", "quit"}),
	)
}

func (m Model) table(width, rows int) table.Table {
	// The model column takes whatever the fixed price columns leave behind.
	const fixed = 84
	name := max(width-fixed, 20)

	body := make([][]string, 0, len(m.models))
	for _, model := range m.models {
		body = append(body, []string{
			model.ID,
			util.FormatTokens(model.ContextWindow),
			util.FormatPrice(model.InputPrice),
			util.FormatPrice(model.OutputPrice),
			util.FormatCachePrice(model.CacheReadPrice),
			util.FormatCachePrice(model.CacheWritePrice),
			strconv.FormatBool(model.DataRetention),
		})
	}

	return table.Table{
		Cols: []table.Column{
			{Title: "MODEL", Width: name, Align: table.Left},
			{Title: "CONTEXT", Width: 10, Align: table.Right},
			{Title: "IN $/M", Width: 10, Align: table.Right},
			{Title: "OUT $/M", Width: 11, Align: table.Right},
			{Title: "CACHE READ $/M", Width: 16, Align: table.Right},
			{Title: "CACHE WRITE $/M", Width: 17, Align: table.Right},
			{Title: "DATA RETENTION", Width: 16, Align: table.Right},
		},
		Rows:   body,
		Cursor: m.cursor,
		Height: max(rows, 1),
		Style:  table.CellStyle(m.cursor),
	}
}
