package calendar

import "time"

// ThirdTuesday returns the 3rd Tuesday of the given month/year.
// If that Tuesday is a Korean holiday, it rolls backward to the prior business day.
// This matches the KRX KTB futures expiry rule.
func ThirdTuesday(year int, month time.Month) time.Time {
	t := time.Date(year, month, 15, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Tuesday {
		t = t.AddDate(0, 0, 1)
	}
	for !IsBusinessDay(KR, t) {
		t = t.AddDate(0, 0, -1)
	}
	return t
}

// KTBFuturesExpiry returns the nearest KTB futures expiry date on or after today.
// KTB futures expire on the 3rd Tuesday of Mar/Jun/Sep/Dec.
func KTBFuturesExpiry(today time.Time) time.Time {
	tt := ThirdTuesday(today.Year(), today.Month())

	if int(tt.Month())%3 == 0 {
		if today.Before(tt) {
			return tt
		}
		next := tt.AddDate(0, 3, 0)
		return ThirdTuesday(next.Year(), next.Month())
	}

	for int(tt.Month())%3 != 0 {
		tt = tt.AddDate(0, 1, 0)
	}
	return ThirdTuesday(tt.Year(), tt.Month())
}
