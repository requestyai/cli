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
	"github.com/requestyai/cli/internal/tui/ui/text"
	"github.com/requestyai/cli/internal/util"
	"github.com/shopspring/decimal"
)

const usageDayCount = 30

// usageTotals contains aggregate usage values for the selected period.
type usageTotals struct {
	Spend    decimal.Decimal
	Requests int
	Tokens   int
}

// usageLoadedMsg carries loaded usage totals or an error back to the update loop.
type usageLoadedMsg struct {
	totals usageTotals
	err    error
}

// usageState holds the usage client, totals, and loading state.
type usageState struct {
	client     *client.Client
	now        func() time.Time
	totals     usageTotals
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

	var totals usageTotals
	for _, entry := range usage {
		totals.Spend = totals.Spend.Add(entry.Spend)
		totals.Requests += entry.TotalRequests
		totals.Tokens += entry.TotalTokens
	}

	return usageLoadedMsg{totals: totals, err: err}
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
		u.loaded = true
	}
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

	header := text.RenderSplitLine("", theme.Muted.Render("last 30 days"), width)
	if lipgloss.Height(cards)+lipgloss.Height(header) > height {
		return cards
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, cards)
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
