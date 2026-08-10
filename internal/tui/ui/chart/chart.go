package chart

import (
	"fmt"
	"math"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
)

// Bar is one value and its optional axis label.
type Bar struct {
	Label string
	Value float64
	// Values turns the bar into a stacked bar. Value is used when Values is empty.
	Values []BarValue
}

// BarValue is one named, colored segment in a stacked bar.
type BarValue struct {
	Name  string
	Value float64
	Style lipgloss.Style
}

// Render draws a compact vertical bar chart within the requested dimensions.
func Render(
	bars []Bar,
	width, height int,
	axisStyle, labelStyle, barStyle lipgloss.Style,
	formatValue func(float64) string,
) string {
	if len(bars) == 0 || height < 3 {
		return ""
	}

	data := make([]barchart.BarData, 0, len(bars))
	maxValue := 0.0
	for _, bar := range bars {
		values := make([]barchart.BarValue, 0, max(len(bar.Values), 1))
		total := 0.0
		if len(bar.Values) == 0 {
			value := max(bar.Value, 0)
			total = value
			values = append(values, barchart.BarValue{Value: value, Style: barStyle})
		} else {
			for _, segment := range bar.Values {
				value := max(segment.Value, 0)
				total += value
				values = append(values, barchart.BarValue{
					Name:  segment.Name,
					Value: value,
					Style: segment.Style,
				})
			}
		}
		maxValue = max(maxValue, total)
		data = append(data, barchart.BarData{Label: bar.Label, Values: values})
	}

	// ntcharts cannot derive a useful scale from an all-zero data set.
	if maxValue == 0 {
		maxValue = 1
	}
	scaleMax := niceMax(maxValue)
	if formatValue == nil {
		formatValue = func(value float64) string {
			return fmt.Sprintf("%g", value)
		}
	}

	yAxis := renderYAxis(formatValue(scaleMax), height, axisStyle, labelStyle)
	plotWidth := width - lipgloss.Width(yAxis)
	if plotWidth < len(bars) {
		return ""
	}

	model := barchart.New(
		plotWidth,
		height,
		barchart.WithDataSet(data),
		barchart.WithMaxValue(scaleMax),
		barchart.WithNoAutoMaxValue(),
		barchart.WithBarGap(0),
		barchart.WithStyles(axisStyle, labelStyle),
	)
	model.Draw()
	return lipgloss.JoinHorizontal(lipgloss.Top, yAxis, model.View())
}

func renderYAxis(maxLabel string, height int, axisStyle, labelStyle lipgloss.Style) string {
	labelWidth := max(lipgloss.Width(maxLabel), 1)
	lines := make([]string, height)
	for row := range height {
		label := ""
		axis := "│"
		switch row {
		case 0:
			label = maxLabel
		case height - 2:
			label = "0"
			axis = "└"
		case height - 1:
			axis = " "
		}

		lines[row] = labelStyle.
			Width(labelWidth).
			Align(lipgloss.Right).
			Render(label) + " " + axisStyle.Render(axis)
	}
	return lipgloss.JoinVertical(lipgloss.Right, lines...)
}

func niceMax(value float64) float64 {
	if value <= 0 {
		return 1
	}

	magnitude := math.Pow(10, math.Floor(math.Log10(value)))
	normalized := value / magnitude
	switch {
	case normalized <= 1:
		normalized = 1
	case normalized <= 2:
		normalized = 2
	case normalized <= 5:
		normalized = 5
	default:
		normalized = 10
	}
	return normalized * magnitude
}
