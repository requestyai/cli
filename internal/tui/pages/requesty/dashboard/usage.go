package dashboard

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/tui/ui/card"
	"github.com/requestyai/cli/internal/tui/ui/chart"
	"github.com/requestyai/cli/internal/tui/ui/text"
	"github.com/requestyai/cli/internal/util"
	"github.com/shopspring/decimal"
)

const (
	usageDayCount       = 7
	usageChartMinHeight = 4
	usageChartMaxHeight = 8
)

type usageMetric uint8

const (
	usageMetricSpend usageMetric = iota
	usageMetricRequests
	usageMetricTokens
)

// usageDay contains one UTC calendar day's usage.
type usageDay struct {
	Date  time.Time
	Entry client.UsageEntry
}

// usageTotals contains aggregate usage values for the selected period.
type usageTotals struct {
	Spend    decimal.Decimal
	Requests int
	Tokens   int
}

// usageLoadedMsg carries loaded usage totals or an error back to the update loop.
type usageLoadedMsg struct {
	totals usageTotals
	days   []usageDay
	err    error
}

// usageState holds the usage client, totals, and loading state.
type usageState struct {
	client     *client.Client
	now        func() time.Time
	totals     usageTotals
	days       []usageDay
	metric     usageMetric
	loaded     bool
	err        error
	refreshing bool
}

// newUsageState creates usage state wired to the given API client.
func newUsageState(client *client.Client) usageState {
	return usageState{client: client, now: time.Now}
}

// load fetches and totals usage for the current window.
func (u usageState) load() tea.Msg {
	start, end := usageRange(u.now())
	usage, err := u.client.Usage(context.Background(), client.UsageInput{
		Start:      start,
		End:        end,
		Resolution: "day",
	})
	if err != nil {
		return usageLoadedMsg{err: err}
	}

	var totals usageTotals
	for _, entry := range usage {
		totals.Spend = totals.Spend.Add(entry.Spend)
		totals.Requests += entry.TotalRequests
		totals.Tokens += entry.TotalTokens
	}

	return usageLoadedMsg{
		totals: totals,
		days:   normalizeUsageDays(usage, start),
	}
}

// update applies a loaded usage result into state.
func (u *usageState) update(msg tea.Msg) {
	typed, ok := msg.(usageLoadedMsg)
	if !ok {
		return
	}

	u.refreshing = false
	u.err = typed.err

	if typed.err == nil {
		u.totals = typed.totals
		u.days = typed.days
		u.loaded = true
	}
}

// selectMetric applies a keyboard selection and reports whether it handled the key.
func (u *usageState) selectMetric(key string) bool {
	switch key {
	case "1":
		u.metric = usageMetricSpend
	case "2":
		u.metric = usageMetricRequests
	case "3":
		u.metric = usageMetricTokens
	default:
		return false
	}
	return true
}

// refresh reloads usage unless a refresh is already in flight.
func (u *usageState) refresh() tea.Cmd {
	if u.refreshing {
		return nil
	}

	u.refreshing = true
	return u.load
}

func (u usageState) view(width, height int) string {
	if height <= 0 {
		return ""
	}
	if u.err != nil && !u.loaded {
		return theme.Bad.Render("Could not load usage: " + u.err.Error())
	}

	cards := u.cards(width)
	if lipgloss.Height(cards) > height {
		return ""
	}

	header := text.RenderSplitLine("", theme.Muted.Render("last 7 days"), width)
	if lipgloss.Height(cards)+lipgloss.Height(header) > height {
		return cards
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, cards)
	chartHeight := min(height-lipgloss.Height(content)-2, usageChartMaxHeight)
	if !u.loaded || chartHeight < usageChartMinHeight {
		return content
	}

	renderedChart := u.chart(width, chartHeight)
	if renderedChart == "" {
		return content
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		u.metricSelector(),
		text.LineSeparator,
		renderedChart,
	)
}

func (u usageState) cards(width int) string {
	values := [3]string{"…", "…", "…"}
	if u.loaded {
		values = [3]string{
			util.FormatSpend(u.totals.Spend),
			util.FormatCount(u.totals.Requests),
			util.FormatCount(u.totals.Tokens),
		}
	}

	if width < 42 {
		line := fmt.Sprintf("Spend %s · Requests %s · Tokens %s", values[0], values[1], values[2])
		if u.refreshing {
			line += " · refreshing"
		}
		return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(line)
	}

	gaps := 2
	baseWidth := (width - gaps) / 3
	lastWidth := width - gaps - 2*baseWidth
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		card.Render("SPEND", values[0], baseWidth),
		text.SpaceSeparator,
		card.Render("REQUESTS", values[1], baseWidth),
		text.SpaceSeparator,
		card.Render("TOKENS", values[2], lastWidth),
	)
}

// usageRange returns the inclusive start and end of the usage window ending at now.
func usageRange(now time.Time) (time.Time, time.Time) {
	end := now.UTC()
	today := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(usageDayCount - 1)), end
}

// normalizeUsageDays converts timestamp-keyed API data into a dense UTC day series.
func normalizeUsageDays(usage map[string]client.UsageEntry, start time.Time) []usageDay {
	start = time.Date(start.UTC().Year(), start.UTC().Month(), start.UTC().Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, usageDayCount)
	entries := make(map[string]client.UsageEntry, usageDayCount)

	for timestamp, entry := range usage {
		at, err := parseUsageTimestamp(timestamp)
		if err != nil {
			continue
		}
		at = at.UTC()
		date := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
		if date.Before(start) || !date.Before(end) {
			continue
		}

		key := date.Format(time.DateOnly)
		current := entries[key]
		current.Spend = current.Spend.Add(entry.Spend)
		current.TotalRequests += entry.TotalRequests
		current.TotalTokens += entry.TotalTokens
		entries[key] = current
	}

	days := make([]usageDay, usageDayCount)
	for i := range days {
		date := start.AddDate(0, 0, i)
		days[i] = usageDay{
			Date:  date,
			Entry: entries[date.Format(time.DateOnly)],
		}
	}
	return days
}

func parseUsageTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.DateOnly} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported usage timestamp %q", value)
}

func (m usageMetric) label() string {
	switch m {
	case usageMetricRequests:
		return "Requests"
	case usageMetricTokens:
		return "Tokens"
	default:
		return "Spend"
	}
}

func (m usageMetric) value(entry client.UsageEntry) float64 {
	switch m {
	case usageMetricRequests:
		return float64(entry.TotalRequests)
	case usageMetricTokens:
		return float64(entry.TotalTokens)
	default:
		return entry.Spend.InexactFloat64()
	}
}

func (u usageState) metricSelector() string {
	metrics := []usageMetric{usageMetricSpend, usageMetricRequests, usageMetricTokens}
	choices := make([]string, 0, len(metrics))
	for i, metric := range metrics {
		style := theme.PillOff
		if metric == u.metric {
			style = theme.Pill
		}
		choices = append(choices, style.Render(fmt.Sprintf("%d %s", i+1, metric.label())))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, choices...)
}

func (u usageState) chart(width, height int) string {
	bars := make([]chart.Bar, 0, len(u.days))
	for _, day := range u.days {
		bars = append(bars, chart.Bar{
			Label: day.Date.Format("Mon"),
			Value: u.metric.value(day.Entry),
		})
	}

	barStyle := lipgloss.NewStyle().
		Foreground(theme.Blue).
		Background(theme.Blue)
	return chart.Render(
		bars,
		width,
		height,
		theme.Muted,
		theme.Muted,
		barStyle,
		u.metric.formatAxisValue,
	)
}

func (m usageMetric) formatAxisValue(value float64) string {
	if m == usageMetricSpend {
		return util.FormatSpend(decimal.NewFromFloat(value))
	}

	return util.FormatCount(int(value))
}
