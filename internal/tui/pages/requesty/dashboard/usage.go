package dashboard

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/client"
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

// usageRange returns the inclusive start and end of the usage window ending at now.
func usageRange(now time.Time) (time.Time, time.Time) {
	end := now.UTC()
	today := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(usageDayCount - 1)), end
}
