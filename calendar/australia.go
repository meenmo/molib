package calendar

import "time"

// isAustralianHoliday implements Sydney banking holidays. August Bank Holiday
// and October Labour Day are included; these are not full RITS closure days.
// Rules follow https://www.nsw.gov.au/about-nsw/public-holidays . Future
// one-off proclamations require updates. This is not a licensed Bloomberg AU feed.
func isAustralianHoliday(t time.Time) bool {
	y, m, d := t.Date()
	date := func(month time.Month, day int) time.Time { return time.Date(y, month, day, 0, 0, 0, 0, time.UTC) }
	observed := func(month time.Month, day int) time.Time {
		v := date(month, day)
		for v.Weekday() == time.Saturday || v.Weekday() == time.Sunday {
			v = v.AddDate(0, 0, 1)
		}
		return v
	}
	same := func(v time.Time) bool { return m == v.Month() && d == v.Day() }
	if same(observed(time.January, 1)) || same(observed(time.January, 26)) {
		return true
	}
	// Gregorian Easter (Meeus/Jones/Butcher).
	a, b, c := y%19, y/100, y%100
	dd, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - dd - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	mm := (a + 11*h + 22*l) / 451
	easter := date(time.Month((h+l-7*mm+114)/31), (h+l-7*mm+114)%31+1)
	if same(easter.AddDate(0, 0, -2)) || same(easter.AddDate(0, 0, 1)) {
		return true
	}
	if m == time.April && d == 25 {
		return true
	}
	// NSW proclaimed extra Anzac holidays for 2026 and 2027 only.
	if (y == 2026 && m == time.April && d == 27) || (y == 2027 && m == time.April && d == 26) {
		return true
	}
	if y == 2022 && m == time.September && d == 22 {
		return true
	}
	if t.Weekday() == time.Monday && ((m == time.June && d >= 8 && d <= 14) || (m == time.August && d <= 7) || (m == time.October && d <= 7)) {
		return true
	}
	if m == time.December {
		if d == 25 || d == 26 {
			return true
		}
		// Additional Christmas/Boxing holidays use Dec 27/28 when on weekends.
		christmas, boxing := date(time.December, 25), date(time.December, 26)
		if d == 27 && (christmas.Weekday() == time.Saturday || christmas.Weekday() == time.Sunday || boxing.Weekday() == time.Sunday) {
			return true
		}
		if d == 28 && (boxing.Weekday() == time.Saturday || christmas.Weekday() == time.Saturday) {
			return true
		}
	}
	return false
}
