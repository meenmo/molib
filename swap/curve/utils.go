package curve

import (
	"sort"
	"time"
)

// findBracket finds two adjacent dates in a sorted slice that bracket the target.
// Returns (d1, d2, true) where d1 <= target <= d2, or (zero, zero, false) if not found.
//
// This uses binary search for O(log n) complexity instead of O(n) linear search.
func findBracket(dates []time.Time, target time.Time) (d1, d2 time.Time, found bool) {
	if len(dates) < 2 {
		return time.Time{}, time.Time{}, false
	}

	// Binary search for first date >= target
	idx := sort.Search(len(dates), func(i int) bool {
		return !dates[i].Before(target)
	})

	// Handle boundary cases
	if idx == 0 {
		// target is before or equal to first date
		if dates[0].Equal(target) && len(dates) > 1 {
			return dates[0], dates[1], true
		}
		return time.Time{}, time.Time{}, false
	}

	if idx >= len(dates) {
		// target is after all dates
		return time.Time{}, time.Time{}, false
	}

	// Normal case: dates[idx-1] < target <= dates[idx]
	return dates[idx-1], dates[idx], true
}

// findBracketOrBoundary finds two adjacent dates that bracket the target.
// If the target is outside the range, returns the nearest boundary pair.
//
// This is useful for extrapolation where we still want the two nearest dates.
func findBracketOrBoundary(dates []time.Time, target time.Time) (d1, d2 time.Time) {
	if len(dates) < 2 {
		panic("findBracketOrBoundary: need at least 2 dates")
	}

	// Binary search for first date >= target
	idx := sort.Search(len(dates), func(i int) bool {
		return !dates[i].Before(target)
	})

	// Handle boundary cases
	if idx <= 0 {
		// target is before or equal to first date
		return dates[0], dates[1]
	}

	if idx >= len(dates) {
		// target is after all dates
		return dates[len(dates)-2], dates[len(dates)-1]
	}

	// Normal case: dates[idx-1] < target <= dates[idx]
	return dates[idx-1], dates[idx]
}

// binarySearchDate finds the index of the first date >= target using binary search.
// Returns len(dates) if all dates are before target.
func binarySearchDate(dates []time.Time, target time.Time) int {
	return sort.Search(len(dates), func(i int) bool {
		return !dates[i].Before(target)
	})
}

// findExactOrBracket searches for an exact match first, then falls back to bracket.
// Returns (date, true, -1, time.Time{}) if exact match found at returned date.
// Returns (time.Time{}, false, idx1, d2) if bracket found.
// Returns (time.Time{}, false, -1, time.Time{}) if neither found.
func findExactOrBracket(dates []time.Time, target time.Time) (exact time.Time, isExact bool, bracketIdx int, d2 time.Time) {
	if len(dates) == 0 {
		return time.Time{}, false, -1, time.Time{}
	}

	idx := binarySearchDate(dates, target)

	// Check for exact match
	if idx < len(dates) && dates[idx].Equal(target) {
		return dates[idx], true, -1, time.Time{}
	}

	// Check if we can form a bracket
	if idx > 0 && idx < len(dates) {
		return time.Time{}, false, idx - 1, dates[idx]
	}

	return time.Time{}, false, -1, time.Time{}
}
