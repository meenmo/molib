package utils

import (
	"time"
)

// YearFraction computes year fraction between two dates using the specified day count convention.
// Supported conventions: ACT/360, ACT/365F, 30E/360, 30/360
func YearFraction(start, end time.Time, convention string) float64 {
	switch convention {
	case "ACT/360":
		days := end.Sub(start).Hours() / 24
		return days / 360.0
	case "ACT/365F":
		days := end.Sub(start).Hours() / 24
		return days / 365.0
	case "30E/360", "30/360":
		// 30E/360 ISDA (Eurobond basis)
		// D1 and D2 are capped at 30
		d1 := start.Day()
		if d1 > 30 {
			d1 = 30
		}
		d2 := end.Day()
		if d2 > 30 {
			d2 = 30
		}
		y1, m1 := start.Year(), int(start.Month())
		y2, m2 := end.Year(), int(end.Month())
		return float64(360*(y2-y1)+30*(m2-m1)+(d2-d1)) / 360.0
	default:
		days := end.Sub(start).Hours() / 24
		return days / 365.0
	}
}
