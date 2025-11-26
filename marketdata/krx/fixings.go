package krx

import "time"

// DefaultReferenceFeed builds a map-backed feed using the bundled CD91 fixings.
func DefaultReferenceFeed() ReferenceRateFeed {
	return &MapReferenceRateFeed{rates: CD91Fixings}
}

// RateOnDate is a convenience helper when you don't want to wire a feed.
func RateOnDate(feed ReferenceRateFeed, date time.Time) (float64, bool) {
	return feed.RateOn(date)
}
