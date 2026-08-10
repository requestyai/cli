package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	usageChartMinHeight = 5
	usageChartMaxHeight = 8
)

type usageMetric uint8

const (
	usageMetricSpend usageMetric = iota
	usageMetricRequests
	usageMetricTokens
)

type usageGroup uint8

const (
	usageGroupModel usageGroup = iota
	usageGroupOrigin
	usageGroupProvider
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
	group  usageGroup
	err    error
}

// usageState holds the usage client, totals, and loading state.
type usageState struct {
	client     *client.Client
	now        func() time.Time
	totals     usageTotals
	days       []usageDay
	metric     usageMetric
	group      usageGroup
	dataGroup  usageGroup
	loaded     bool
	err        error
	refreshing bool
}

// newUsageState creates usage state wired to the given API client.
func newUsageState(client *client.Client) usageState {
	return usageState{client: client, now: time.Now, group: usageGroupModel}
}

// load fetches and totals usage for the current window.
func (u usageState) load() tea.Msg {
	start, end := usageRange(u.now())
	usage, err := u.client.Usage(context.Background(), client.UsageInput{
		Start:      start,
		End:        end,
		Resolution: "day",
		GroupBy:    u.group.field(),
	})
	if err != nil {
		return usageLoadedMsg{group: u.group, err: err}
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
		group:  u.group,
	}
}

// update applies a loaded usage result into state.
func (u *usageState) update(msg tea.Msg) {
	typed, ok := msg.(usageLoadedMsg)
	if !ok {
		return
	}
	if typed.group != u.group {
		return
	}

	u.refreshing = false
	u.err = typed.err

	if typed.err == nil {
		u.totals = typed.totals
		u.days = typed.days
		u.dataGroup = typed.group
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

// selectGroup applies a grouping selection and reports whether it changed.
func (u *usageState) selectGroup(key string) bool {
	var group usageGroup
	switch key {
	case "6":
		group = usageGroupModel
	case "7":
		group = usageGroupOrigin
	case "8":
		group = usageGroupProvider
	default:
		return false
	}
	if group == u.group {
		return false
	}
	u.group = group
	return true
}

// refresh reloads usage unless a refresh is already in flight.
func (u *usageState) refresh() tea.Cmd {
	if u.refreshing {
		return nil
	}

	return u.reload()
}

// reload starts a request for the current grouping, superseding older responses.
func (u *usageState) reload() tea.Cmd {
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
	spacedContent := lipgloss.JoinVertical(lipgloss.Left, content, text.LineSeparator)
	chartHeight := min(height-lipgloss.Height(spacedContent)-2, usageChartMaxHeight)
	if !u.loaded || chartHeight < usageChartMinHeight {
		return content
	}

	renderedChart := u.chart(width, chartHeight)
	if renderedChart == "" {
		return content
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		spacedContent,
		text.RenderSplitLine(u.metricSelector(), u.groupSelector(), width),
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
		current.GroupedData = append(current.GroupedData, entry.GroupedData...)
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

func (m usageMetric) groupedValue(entry client.UsageGrouped) float64 {
	switch m {
	case usageMetricRequests:
		return float64(entry.TotalRequests)
	case usageMetricTokens:
		return float64(entry.TotalTokens)
	default:
		return entry.Spend.InexactFloat64()
	}
}

func (g usageGroup) field() string {
	switch g {
	case usageGroupOrigin:
		return "origin_title"
	case usageGroupProvider:
		return "provider_used"
	default:
		return "model_used"
	}
}

func (g usageGroup) label() string {
	switch g {
	case usageGroupOrigin:
		return "Origin"
	case usageGroupProvider:
		return "Provider"
	default:
		return "Model"
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

func (u usageState) groupSelector() string {
	groups := []usageGroup{usageGroupModel, usageGroupOrigin, usageGroupProvider}
	choices := make([]string, 0, len(groups))
	for i, group := range groups {
		style := theme.PillOff
		if group == u.group {
			style = theme.Pill
		}
		choices = append(choices, style.Render(fmt.Sprintf("%d %s", i+6, group.label())))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, choices...)
}

func (u usageState) chart(width, height int) string {
	styles := usageGroupStyles()
	var names []string
	if u.dataGroup == u.group && !u.refreshing {
		names = u.groupNames()
	}
	styleByName := make(map[string]lipgloss.Style, len(names))
	for i, name := range names {
		styleByName[name] = styles[i%len(styles)]
	}

	bars := make([]chart.Bar, 0, len(u.days))
	for _, day := range u.days {
		valuesByName := make(map[string]float64, len(names))
		for _, grouped := range day.Entry.GroupedData {
			name := groupedName(grouped.GroupByValues[u.group.field()])
			if name == "" {
				continue
			}
			valuesByName[name] += u.metric.groupedValue(grouped)
		}

		values := make([]chart.BarValue, 0, len(names))
		for _, name := range names {
			values = append(values, chart.BarValue{
				Name:  name,
				Value: valuesByName[name],
				Style: styleByName[name],
			})
		}
		bar := chart.Bar{Label: day.Date.Format("Mon"), Values: values}
		if len(values) == 0 {
			bar.Value = u.metric.value(day.Entry)
		}
		bars = append(bars, bar)
	}

	legend := renderUsageLegend(names, styleByName, width)
	legendBlock := lipgloss.JoinVertical(lipgloss.Left, text.LineSeparator, legend)
	plotHeight := height - lipgloss.Height(legendBlock)
	if plotHeight < 3 {
		return ""
	}

	rendered := chart.Render(
		bars,
		width,
		plotHeight,
		theme.Muted,
		theme.Muted,
		styles[0],
		u.metric.formatAxisValue,
	)
	if rendered == "" || legend == "" {
		return rendered
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered, legendBlock)
}

func (u usageState) groupNames() []string {
	seen := make(map[string]struct{})
	for _, day := range u.days {
		for _, grouped := range day.Entry.GroupedData {
			name := groupedName(grouped.GroupByValues[u.group.field()])
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

func groupedName(value any) string {
	name := strings.TrimSpace(fmt.Sprint(value))
	if value == nil || name == "" || name == "<nil>" {
		return ""
	}
	return name
}

func usageGroupStyles() []lipgloss.Style {
	return []lipgloss.Style{
		lipgloss.NewStyle().Foreground(theme.Blue).Background(theme.Blue),
		lipgloss.NewStyle().Foreground(theme.Green).Background(theme.Green),
		lipgloss.NewStyle().Foreground(theme.Amber).Background(theme.Amber),
		lipgloss.NewStyle().Foreground(theme.Red).Background(theme.Red),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Background(lipgloss.Color("#A78BFA")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Background(lipgloss.Color("#22D3EE")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F472B6")).Background(lipgloss.Color("#F472B6")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FB923C")).Background(lipgloss.Color("#FB923C")),
	}
}

func renderUsageLegend(names []string, styles map[string]lipgloss.Style, width int) string {
	if len(names) == 0 || width <= 0 {
		return " "
	}

	lines := make([]string, 0, 2)
	line := ""
	for _, name := range names {
		item := styles[name].UnsetBackground().Render("█") + " " + name
		candidate := item
		if line != "" {
			candidate = line + "  " + item
		}
		if line != "" && lipgloss.Width(candidate) > width {
			lines = append(lines, line)
			line = item
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}

	centered := make([]string, 0, len(lines))
	for _, line := range lines {
		centered = append(centered, lipgloss.NewStyle().
			Width(width).
			MaxWidth(width).
			Align(lipgloss.Center).
			Render(line))
	}

	return lipgloss.JoinVertical(lipgloss.Center, centered...)
}

func (m usageMetric) formatAxisValue(value float64) string {
	if m == usageMetricSpend {
		return util.FormatSpend(decimal.NewFromFloat(value))
	}

	return util.FormatCount(int(value))
}
