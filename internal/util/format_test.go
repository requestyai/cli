package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTokens(t *testing.T) {
	testCases := []struct {
		input    int
		expected string
	}{
		{11900, "11K"},
		{128972, "128K"},
		{400, "400"},
	}
	for _, tc := range testCases {
		actual := FormatTokens(tc.input)
		assert.Equal(t, tc.expected, actual)
	}
}

func TestFormatPrice(t *testing.T) {
	testCases := []struct {
		input    float64
		expected string
	}{
		{0.000005, "$5.00"},
		{0.00005, "$50.00"},
		{0.0005, "$500.00"},
	}
	for _, tc := range testCases {
		actual := FormatPrice(tc.input)
		assert.Equal(t, tc.expected, actual)
	}
}

func TestFormatCachePrice(t *testing.T) {
	assert.Equal(t, "-", FormatCachePrice(0))
	assert.Equal(t, "$5.00", FormatCachePrice(0.000005))
}
