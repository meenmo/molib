package calendar

import "time"

// CalendarID identifies a holiday calendar.
type CalendarID string

const (
	TARGET CalendarID = "TARGET"
	JP     CalendarID = "JPN"
	FD     CalendarID = "FD" // Federal Reserve (Fedwire-style) calendar
	GT     CalendarID = "GT" // US Government bond calendar
	KR     CalendarID = "KOR"
)

// buildHolidayMap creates a holiday lookup map from a list of date strings.
// This is a shared factory function to eliminate duplicate init code.
func buildHolidayMap(holidays []string) map[string]struct{} {
	m := make(map[string]struct{}, len(holidays))
	for _, h := range holidays {
		m[h] = struct{}{}
	}
	return m
}

// Holiday maps are initialized using buildHolidayMap.
// Each calendar file (japan.go, target.go, korea.go) defines its holiday list.
var targetHolidays = map[string]struct{}{}
var jpHolidays = map[string]struct{}{}
var fdHolidays = map[string]struct{}{}
var gtHolidays = map[string]struct{}{}
var krHolidays = buildHolidayMap(krHolidayList)

func isHoliday(cal CalendarID, t time.Time) bool {
	key := t.Format("2006-01-02")
	switch cal {
	case TARGET:
		_, ok := targetHolidays[key]
		return ok
	case JP:
		_, ok := jpHolidays[key]
		return ok
	case FD:
		_, ok := fdHolidays[key]
		return ok
	case GT:
		_, ok := gtHolidays[key]
		return ok
	case KR:
		_, ok := krHolidays[key]
		return ok
	default:
		return false
	}
}

// IsBusinessDay checks weekends and holiday sets.
func IsBusinessDay(cal CalendarID, t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}
	return !isHoliday(cal, t)
}

// Adjust applies Modified Following.
func Adjust(cal CalendarID, t time.Time) time.Time {
	origMonth := t.Month()
	for !IsBusinessDay(cal, t) {
		t = t.AddDate(0, 0, 1)
	}
	if t.Month() != origMonth {
		t = t.AddDate(0, 0, -1)
		for !IsBusinessDay(cal, t) {
			t = t.AddDate(0, 0, -1)
		}
	}
	return t
}

// AdjustFollowing applies a simple Following convention (no month preservation).
func AdjustFollowing(cal CalendarID, t time.Time) time.Time {
	for !IsBusinessDay(cal, t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// AddBusinessDays advances n business days (n can be negative).
func AddBusinessDays(cal CalendarID, t time.Time, n int) time.Time {
	step := 1
	if n < 0 {
		step = -1
	}
	for n != 0 {
		t = t.AddDate(0, 0, step)
		if IsBusinessDay(cal, t) {
			n -= step
		}
	}
	return t
}

// AddYearsWithRoll adds years and applies backward EOM adjustment then Modified Following.
func AddYearsWithRoll(cal CalendarID, t time.Time, years int) time.Time {
	target := t.AddDate(years, 0, 0)
	if t.Day() >= daysInMonth(t.Year(), t.Month()) {
		target = time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	}
	return Adjust(cal, target)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// LastBusinessDayOfMonth returns the last business day of the month containing t.
func LastBusinessDayOfMonth(cal CalendarID, t time.Time) time.Time {
	// Move to first day of next month
	nextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	// Go back one day and find the prior business day
	return AddBusinessDays(cal, nextMonth, -1)
}

// IsEndOfMonth checks if t is the last business day of its month.
func IsEndOfMonth(cal CalendarID, t time.Time) bool {
	return t.Equal(LastBusinessDayOfMonth(cal, t))
}
