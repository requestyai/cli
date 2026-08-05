package table

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestTableWidth(t *testing.T) {
	table := Table{
		Cols: []Column{
			{Width: 8},
			{Width: 12},
		},
	}

	assert.Equal(t, gutter+20, table.Width())
}

func TestPad(t *testing.T) {
	testCases := []struct {
		name     string
		text     string
		width    int
		align    Align
		expected string
	}{
		{name: "left", text: "cat", width: 5, align: Left, expected: "cat  "},
		{name: "right", text: "cat", width: 5, align: Right, expected: "  cat"},
		{name: "exact width", text: "cat", width: 3, align: Left, expected: "cat"},
		{name: "truncated", text: "catalogue", width: 4, align: Left, expected: "cata"},
		{name: "wide rune", text: "界", width: 4, align: Right, expected: "  界"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ansi.Strip(pad(tc.text, tc.width, tc.align)))
			assert.Equal(t, tc.width, lipgloss.Width(pad(tc.text, tc.width, tc.align)))
		})
	}
}

func TestTableWindow(t *testing.T) {
	rows := [][]string{{"one"}, {"two"}, {"three"}, {"four"}}

	testCases := []struct {
		name          string
		height        int
		cursor        int
		expectedFirst int
		expectedLast  int
	}{
		{name: "unlimited", height: 0, cursor: 0, expectedFirst: 0, expectedLast: 4},
		{name: "all rows fit", height: 4, cursor: 2, expectedFirst: 0, expectedLast: 4},
		{name: "cursor near start", height: 2, cursor: 0, expectedFirst: 0, expectedLast: 2},
		{name: "cursor in middle", height: 2, cursor: 2, expectedFirst: 1, expectedLast: 3},
		{name: "cursor at end", height: 2, cursor: 3, expectedFirst: 2, expectedLast: 4},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			table := Table{Rows: rows, Height: tc.height, Cursor: tc.cursor}
			first, last := table.window()

			assert.Equal(t, tc.expectedFirst, first)
			assert.Equal(t, tc.expectedLast, last)
		})
	}
}

func TestTableRender(t *testing.T) {
	table := Table{
		Cols: []Column{
			{Title: "NAME", Width: 7, Align: Left},
			{Title: "PRICE", Width: 7, Align: Right},
		},
		Rows: [][]string{
			{"Alice", "$1"},
			{"Bob", "$20"},
			{"Carol", "$300"},
		},
		Cursor: 2,
		Height: 2,
		Style: func(_, _ int) lipgloss.Style {
			return lipgloss.NewStyle()
		},
	}

	expected := "" +
		"  NAME     PRICE\n" +
		"────────────────\n" +
		"  Bob        $20\n" +
		"❯ Carol     $300"

	assert.Equal(t, expected, ansi.Strip(table.Render()))
}

func TestTableRowFillsMissingCells(t *testing.T) {
	table := Table{
		Cols: []Column{
			{Width: 4, Align: Left},
			{Width: 4, Align: Right},
		},
	}

	assert.Equal(t, "  one     ", ansi.Strip(table.Row([]string{"one"}, lipgloss.NewStyle())))
}
