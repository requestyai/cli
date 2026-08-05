package util

import (
	"fmt"
	"strconv"
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
