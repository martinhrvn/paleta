package history

import (
	"fmt"
	"time"
)

// FormatSince renders a duration as a short, human-friendly "… ago" string
// (e.g. "3h ago", "2d ago", "just now"). Durations under a minute — and any
// negative duration from clock skew — render as "just now".
func FormatSince(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	hours := d.Hours()
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(hours))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(hours/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(hours/24/7))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(hours/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(hours/24/365))
	}
}
