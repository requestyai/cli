package table

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/tui/theme"
)

// Align is how a column's cells sit within their width. Numbers read best
// right-aligned so their digits line up.
type Align int

const (
	Left Align = iota
	Right
)

// Column is one fixed-width column of a Table.
type Column struct {
	Title string
	Width int
	Align Align
}

// gutter is the width of the cursor marker to the left of every row.
const gutter = 2

// Table renders fixed-width columns with a selected row. Cells hold plain
// text: colour comes from Style, so a selected row can be repainted whole
// without fighting escape codes already embedded in the text.
type Table struct {
	Cols []Column
	Rows [][]string

	// Cursor is the index of the selected row, or -1 for none.
	Cursor int

	// Height caps how many rows are shown at once. Zero shows all of them.
	Height int

	// Style returns the style for a cell. Row is -1 for the header.
	Style func(row, col int) lipgloss.Style
}

// Width is the table's total width, including the cursor gutter.
func (t Table) Width() int {
	w := gutter
	for _, c := range t.Cols {
		w += c.Width
	}

	return w
}

// Render draws the header, a rule, and as many rows as fit.
func (t Table) Render() string {
	lines := []string{t.header(), t.Rule()}

	first, last := t.window()
	for i := first; i < last; i++ {
		lines = append(lines, t.row(i))
	}

	return strings.Join(lines, "\n")
}

// Rule is a divider as wide as the table.
func (t Table) Rule() string {
	return theme.Rule.Render(strings.Repeat("─", t.Width()))
}

// Row draws cells outside the table body, such as a totals line, using the
// table's own column widths.
func (t Table) Row(cells []string, style lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(style.Render(strings.Repeat(" ", gutter)))

	for i, c := range t.Cols {
		var text string
		if i < len(cells) {
			text = cells[i]
		}
		b.WriteString(style.Render(pad(text, c.Width, c.Align)))
	}

	return b.String()
}

// window is the half-open range of rows to draw, scrolled to keep the cursor
// in view.
func (t Table) window() (first, last int) {
	if t.Height <= 0 || len(t.Rows) <= t.Height {
		return 0, len(t.Rows)
	}

	first = t.Cursor - t.Height + 1
	if first < 0 {
		first = 0
	}
	if max := len(t.Rows) - t.Height; first > max {
		first = max
	}

	return first, first + t.Height
}

func (t Table) header() string {
	titles := make([]string, 0, len(t.Cols))
	for _, c := range t.Cols {
		titles = append(titles, c.Title)
	}

	return t.Row(titles, t.style(-1, 0))
}

func (t Table) row(i int) string {
	marker := strings.Repeat(" ", gutter)
	if i == t.Cursor {
		marker = "❯ "
	}

	var b strings.Builder
	b.WriteString(t.style(i, 0).Render(marker))

	for col, c := range t.Cols {
		var text string
		if col < len(t.Rows[i]) {
			text = t.Rows[i][col]
		}
		b.WriteString(t.style(i, col).Render(pad(text, c.Width, c.Align)))
	}

	return b.String()
}

func (t Table) style(row, col int) lipgloss.Style {
	if t.Style == nil {
		return theme.Body
	}

	return t.Style(row, col)
}

// pad fits text to a width, truncating anything too long to keep the columns
// from drifting.
func pad(text string, width int, align Align) string {
	if lipgloss.Width(text) > width {
		text = lipgloss.NewStyle().MaxWidth(width).Render(text)
	}

	space := strings.Repeat(" ", width-lipgloss.Width(text))
	if align == Right {
		return space + text
	}

	return text + space
}

// CellStyle is the standard table styling: a bold dim header and a blue wash
// across the selected row. Pages wrap it to tint individual columns.
func CellStyle(cursor int) func(row, col int) lipgloss.Style {
	return func(row, _ int) lipgloss.Style {
		switch {
		case row < 0:
			return theme.Label.Bold(true)
		case row == cursor:
			return lipgloss.NewStyle().Foreground(theme.BlueLite).Background(theme.BgSelect)
		default:
			return theme.Body
		}
	}
}
