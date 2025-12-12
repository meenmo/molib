package utils

import (
	"log"
	"math"
	"sort"
	"time"
)

// SortDates sorts a slice of time.Time in ascending order.
func SortDates(dates []time.Time) {
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
}

// DateParser converts YYYY-MM-DD to time.Time or exits on error.
func DateParser(strDate string) time.Time {
	const layout = "2006-01-02"
	t, err := time.Parse(layout, strDate)
	if err != nil {
		log.Fatal(err)
	}
	return t
}

// Days returns the day count fraction in days between two dates.
func Days(start, end time.Time) float64 {
	return end.Sub(start).Hours() / 24
}

// MonthInt returns the numeric month.
func MonthInt(t time.Time) int {
	return int(t.Month())
}

// AddMonth behaves like Excel's EDATE, avoiding Go's month normalization surprises.
func AddMonth(t time.Time, months int) time.Time {
	target := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	if target.Month() == t.AddDate(0, months, 0).Month() {
		return t.AddDate(0, months, 0)
	}

	d := t.AddDate(0, months, 0)
	origMonth := MonthInt(d)
	for MonthInt(d) == origMonth {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

// RoundTo rounds a float to the specified decimal places.
func RoundTo(val float64, decimals uint32) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
