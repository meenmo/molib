package utils

import (
	"time"

	"github.com/meenmo/molib/calendar"
)

// Note: These functions are re-exported from calendar package for backward compatibility.
// New code should import github.com/meenmo/molib/calendar directly.

// SortDates sorts a slice of time.Time in ascending order.
// Deprecated: Use calendar.SortDates instead.
func SortDates(dates []time.Time) {
	calendar.SortDates(dates)
}

// AdjacentDates returns the two dates from a sorted date slice that bracket target.
// Deprecated: Use calendar.AdjacentDates instead.
func AdjacentDates(target time.Time, dates []time.Time) (time.Time, time.Time) {
	return calendar.AdjacentDates(target, dates)
}

// DateParser converts YYYY-MM-DD to time.Time or panics on error.
// Deprecated: Use calendar.MustParseDate or calendar.ParseDate instead.
func DateParser(strDate string) time.Time {
	return calendar.MustParseDate(strDate)
}

// Days returns the day count fraction in days between two dates.
// Deprecated: Use calendar.Days instead.
func Days(start, end time.Time) float64 {
	return calendar.Days(start, end)
}

// MonthInt returns the numeric month.
// Deprecated: Use calendar.MonthInt instead.
func MonthInt(t time.Time) int {
	return calendar.MonthInt(t)
}

// AddMonth behaves like Excel's EDATE, avoiding Go's month normalization surprises.
// Deprecated: Use calendar.AddMonth instead.
func AddMonth(t time.Time, months int) time.Time {
	return calendar.AddMonth(t, months)
}

// RoundTo rounds a float to the specified decimal places.
// Deprecated: Use calendar.RoundTo instead.
func RoundTo(val float64, decimals uint32) float64 {
	return calendar.RoundTo(val, decimals)
}
