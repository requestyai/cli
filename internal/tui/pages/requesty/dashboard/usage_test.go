package dashboard

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/requestyai/cli/internal/client"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUsageDaysFillsAndCombinesDays(t *testing.T) {
	start := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	usage := map[string]client.UsageEntry{
		"2026-07-12T01:00:00Z": {
			TotalRequests: 2,
			TotalTokens:   20,
			Spend:         decimal.RequireFromString("1.25"),
		},
		"2026-07-12T23:00:00Z": {
			TotalRequests: 3,
			TotalTokens:   30,
			Spend:         decimal.RequireFromString("2.50"),
		},
		"2026-07-15T01:00:00+02:00": {
			TotalRequests: 4,
			TotalTokens:   40,
			Spend:         decimal.RequireFromString("4.00"),
		},
		"2026-07-16": {
			TotalRequests: 6,
		},
		"not-a-date": {
			TotalRequests: 100,
		},
	}

	days := normalizeUsageDays(usage, start)

	require.Len(t, days, usageDayCount)
	assert.Equal(t, start, days[0].Date)
	assert.Equal(t, 5, days[0].Entry.TotalRequests)
	assert.True(t, decimal.RequireFromString("3.75").Equal(days[0].Entry.Spend))
	assert.Zero(t, days[1].Entry.TotalRequests)
	assert.Equal(t, 4, days[2].Entry.TotalRequests, "timezone timestamps are grouped by UTC date")
	assert.Equal(t, 6, days[4].Entry.TotalRequests, "date-only timestamps are supported")
	assert.Equal(t, start.AddDate(0, 0, usageDayCount-1), days[usageDayCount-1].Date)
}

func TestUsageMetricSelection(t *testing.T) {
	state := usageState{}

	assert.True(t, state.selectMetric("2"))
	assert.Equal(t, usageMetricRequests, state.metric)
	assert.True(t, state.selectMetric("3"))
	assert.Equal(t, usageMetricTokens, state.metric)
	assert.True(t, state.selectMetric("1"))
	assert.Equal(t, usageMetricSpend, state.metric)
	assert.False(t, state.selectMetric("right"))
	assert.False(t, state.selectMetric("x"))

	assert.True(t, state.selectGroup("7"))
	assert.Equal(t, usageGroupOrigin, state.group)
	assert.True(t, state.selectGroup("8"))
	assert.Equal(t, usageGroupProvider, state.group)
	assert.True(t, state.selectGroup("6"))
	assert.Equal(t, usageGroupModel, state.group)
	assert.False(t, state.selectGroup("x"))
}

func TestUsageViewAddsChartWhenSpaceAllows(t *testing.T) {
	start := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	state := usageState{
		loaded: true,
		totals: usageTotals{
			Spend:    decimal.RequireFromString("12.34"),
			Requests: 120,
			Tokens:   1200,
		},
		days: normalizeUsageDays(map[string]client.UsageEntry{
			start.Format(time.RFC3339): {
				Spend: decimal.RequireFromString("2.50"),
				GroupedData: []client.UsageGrouped{{
					GroupByValues: map[string]any{"model_used": "openai/gpt-5"},
					Spend:         decimal.RequireFromString("2.50"),
				}},
			},
		}, start),
	}

	withChart := state.view(76, 14)
	withoutChart := state.view(76, 5)
	lines := strings.Split(ansi.Strip(withChart), "\n")

	assert.Contains(t, ansi.Strip(withChart), "1 Spend")
	assert.Contains(t, ansi.Strip(withChart), "6 Model")
	assert.Contains(t, ansi.Strip(withChart), "$5.00")
	assert.Contains(t, ansi.Strip(withChart), "openai/gpt-5")
	require.Greater(t, len(lines), 7)
	assert.Empty(t, strings.TrimSpace(lines[6]), "cards are separated from chart controls")
	assert.Empty(t, strings.TrimSpace(lines[len(lines)-2]), "legend is separated from the plot")
	assert.Contains(t, withChart, "█")
	assert.LessOrEqual(t, lipgloss.Height(withChart), 14)
	assert.Contains(t, ansi.Strip(withoutChart), "SPEND")

	state.refreshing = true
	loading := state.view(76, 14)
	assert.NotContains(t, ansi.Strip(loading), "Unknown")
	assert.NotContains(t, ansi.Strip(loading), "openai/gpt-5")
	assert.Contains(t, loading, "└", "the ungrouped chart remains visible while loading")
	assert.Equal(t, lipgloss.Height(withChart), lipgloss.Height(loading))
}
