package utils

import (
	"time"
)

// YearFraction computes year fraction between two dates using ACT/360 or ACT/365F.
func YearFraction(start, end time.Time, convention string) float64 {
	days := end.Sub(start).Hours() / 24
	switch convention {
	case "ACT/360":
		return days / 360.0
	case "ACT/365F":
		return days / 365.0
	default:
		return days / 365.0
	}
}
