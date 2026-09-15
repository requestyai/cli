package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAPIBaseURL(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "explicit api base url wins", config: Config{APIBaseURL: "http://localhost:40003", RouterBaseURL: "https://router.requesty.ai"}, want: "http://localhost:40003"},
		{name: "derived from router", config: Config{RouterBaseURL: "https://router.requesty.ai"}, want: "https://api-v2.requesty.ai"},
		{name: "derived from staging router", config: Config{RouterBaseURL: "https://router.staging.requesty.ai"}, want: "https://api-v2.staging.requesty.ai"},
		{name: "zero config uses production default", config: Config{}, want: DefaultAPIBaseURL},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.config.ResolveAPIBaseURL())
		})
	}
}
