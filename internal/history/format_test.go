package history

import (
	"testing"
	"time"
)

func TestFormatSince(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"negative clamps", -5 * time.Minute, "just now"},
		{"zero", 0, "just now"},
		{"30 seconds", 30 * time.Second, "just now"},
		{"1 minute", time.Minute, "1m ago"},
		{"59 minutes", 59 * time.Minute, "59m ago"},
		{"1 hour", time.Hour, "1h ago"},
		{"23 hours", 23 * time.Hour, "23h ago"},
		{"1 day", 24 * time.Hour, "1d ago"},
		{"6 days", 6 * 24 * time.Hour, "6d ago"},
		{"1 week", 7 * 24 * time.Hour, "1w ago"},
		{"3 weeks", 21 * 24 * time.Hour, "3w ago"},
		{"1 month", 30 * 24 * time.Hour, "1mo ago"},
		{"11 months", 330 * 24 * time.Hour, "11mo ago"},
		{"1 year", 365 * 24 * time.Hour, "1y ago"},
		{"2 years", 2 * 365 * 24 * time.Hour, "2y ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSince(tt.d); got != tt.want {
				t.Errorf("FormatSince(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
