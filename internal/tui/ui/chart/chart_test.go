package chart

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestRenderUsesRequestedDimensions(t *testing.T) {
	bars := make([]Bar, 30)
	for i := range bars {
		bars[i] = Bar{Value: float64(i + 1)}
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	rendered := Render(bars, 60, 5, style, style, style, nil)

	assert.Equal(t, 60, lipgloss.Width(rendered))
	assert.Equal(t, 5, lipgloss.Height(rendered))
	assert.Contains(t, rendered, "█")
	assert.Contains(t, rendered, "50")
	assert.Contains(t, rendered, "└")
}

func TestRenderHandlesAllZeroData(t *testing.T) {
	bars := []Bar{{Label: "01"}, {Label: "02"}}
	style := lipgloss.NewStyle()

	assert.NotEmpty(t, Render(bars, 20, 4, style, style, style, nil))
}

func TestRenderStacksNamedValues(t *testing.T) {
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Background(lipgloss.Color("4"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Background(lipgloss.Color("2"))
	bars := []Bar{{
		Label: "Mon",
		Values: []BarValue{
			{Name: "Model A", Value: 20, Style: blue},
			{Name: "Model B", Value: 15, Style: green},
		},
	}}

	rendered := Render(bars, 20, 5, blue, blue, blue, nil)

	assert.Contains(t, rendered, "50", "the scale uses the stacked total")
	assert.Contains(t, ansi.Strip(rendered), "Mon")
	assert.Contains(t, rendered, "\x1b[34;44m")
	assert.Contains(t, rendered, "\x1b[32;42m")
}

func TestRenderRequiresRoomForYAxisAndBars(t *testing.T) {
	bars := make([]Bar, 30)
	for i := range bars {
		bars[i].Value = float64(i + 1)
	}
	style := lipgloss.NewStyle()

	rendered := Render(bars, 34, 4, style, style, style, nil)

	assert.Equal(t, 34, lipgloss.Width(rendered))
	assert.Equal(t, 4, lipgloss.Height(rendered))
	assert.Empty(t, Render(bars, 33, 4, style, style, style, nil))
}

func TestRenderRejectsInsufficientSpace(t *testing.T) {
	style := lipgloss.NewStyle()

	assert.Empty(t, Render([]Bar{{Value: 1}}, 2, 4, style, style, style, nil))
	assert.Empty(t, Render([]Bar{{Value: 1}}, 20, 2, style, style, style, nil))
	assert.Empty(t, Render(nil, 20, 4, style, style, style, nil))
}

func TestNiceMax(t *testing.T) {
	assert.Equal(t, 1.0, niceMax(0))
	assert.Equal(t, 2.0, niceMax(1.2))
	assert.Equal(t, 50.0, niceMax(42))
	assert.Equal(t, 100.0, niceMax(99))
}
