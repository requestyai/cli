package util

import (
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
)

// FormatTokens abbreviates a context window: 131072 reads as "131K".
func FormatTokens(n int) string {
	if n >= 1000 {
		return strconv.Itoa(n/1000) + "K"
	}

	return strconv.Itoa(n)
}

// FormatPrice turns a per-token price into the dollars-per-million
// figure the pricing pages quote.
func FormatPrice(perToken float64) string {
	return fmt.Sprintf("$%.2f", perToken*1_000_000)
}

// FormatCachePrice returns a dash for zero value cache priced
// else returns FormatPrice.
func FormatCachePrice(price float64) string {
	if price == 0 {
		return "-"
	}

	return FormatPrice(price)
}

// FormatSpend renders an amount already expressed in dollars.
func FormatSpend(spend decimal.Decimal) string {
	return "$" + spend.StringFixed(2)
}

// FormatCount abbreviates dashboard totals while retaining one decimal place.
func FormatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}
