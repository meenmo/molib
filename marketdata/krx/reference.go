package krx

import "time"

// ReferenceRateFeed supplies short-rate fixings (e.g., CD91) for discounting the first floating period.
type ReferenceRateFeed interface {
	RateOn(date time.Time) (float64, bool)
}

// MapReferenceRateFeed is a static map-backed implementation for development/testing.
type MapReferenceRateFeed struct {
	rates map[string]float64
}

func NewMapReferenceRateFeed(rates map[string]float64) *MapReferenceRateFeed {
	return &MapReferenceRateFeed{rates: rates}
}

func (m *MapReferenceRateFeed) RateOn(date time.Time) (float64, bool) {
	val, ok := m.rates[date.Format("2006-01-02")]
	return val, ok
}
