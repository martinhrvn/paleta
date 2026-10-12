package shelllog

import "time"

// Period is the time window the Ctrl+R picker lists recent commands from.
type Period int

const (
	PeriodHour Period = iota
	PeriodToday
	PeriodWeek
	PeriodMonth
	PeriodAll
	periodCount
)

// DefaultPeriod is what the picker starts with before the user picks one.
const DefaultPeriod = PeriodWeek

var periodNames = [...]string{"1h", "today", "7d", "30d", "all"}
var periodLabels = [...]string{"last hour", "today", "last 7 days", "last 30 days", "all time"}

// String is the period's stored form ("1h", "today", "7d", "30d", "all").
func (p Period) String() string {
	if p < 0 || p >= periodCount {
		return DefaultPeriod.String()
	}
	return periodNames[p]
}

// Label is the period as the picker shows it.
func (p Period) Label() string {
	if p < 0 || p >= periodCount {
		return DefaultPeriod.Label()
	}
	return periodLabels[p]
}

// ParsePeriod reads a stored period; anything unknown is the default.
func ParsePeriod(s string) Period {
	for i, n := range periodNames {
		if n == s {
			return Period(i)
		}
	}
	return DefaultPeriod
}

// Since is the cutoff for the period relative to now; zero for all time.
func (p Period) Since(now time.Time) time.Time {
	switch p {
	case PeriodHour:
		return now.Add(-time.Hour)
	case PeriodToday:
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case PeriodWeek:
		return now.AddDate(0, 0, -7)
	case PeriodMonth:
		return now.AddDate(0, 0, -30)
	}
	return time.Time{}
}

// Next is the next wider period, wrapping from all time back to the last hour.
func (p Period) Next() Period { return (p + 1) % periodCount }

// Prev is the next narrower period, wrapping from the last hour to all time.
func (p Period) Prev() Period { return (p + periodCount - 1) % periodCount }
