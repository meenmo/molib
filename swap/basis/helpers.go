package basis

import "time"

func adjacentDates(target time.Time, dates []time.Time) (time.Time, time.Time) {
	d1 := dates[0]
	d2 := dates[1]
	for _, d := range dates[2:] {
		if d1.Before(target) && target.Before(d2) {
			return d1, d2
		}
		d1 = d2
		d2 = d
	}
	return d1, d2
}
