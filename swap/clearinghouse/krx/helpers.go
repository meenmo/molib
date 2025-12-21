package krx

import (
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

// paymentDatesToTenors converts payment dates to fractional year tenors (0, 0.25, ...).
func paymentDatesToTenors(dates []time.Time) map[time.Time]float64 {
	tenorMap := make(map[time.Time]float64)
	paymentDate := dates[0]
	termination := dates[len(dates)-1].AddDate(0, 0, 1)

	for i := 0.0; paymentDate.Before(termination); i++ {
		tenorMap[calendar.Adjust(calendar.KRW, paymentDate)] = i * 0.25
		paymentDate = paymentDate.AddDate(0, 3, 0)
	}
	return tenorMap
}

// adjacentQuotedDates finds the nearest quoted tenors surrounding a target date.
func adjacentQuotedDates(target time.Time, dates []time.Time, quotes ParSwapQuotes) (time.Time, time.Time) {
	d1 := dates[0]
	d2 := dates[1]

	dateTenor := paymentDatesToTenors(dates)
	for _, d := range dates[2:] {
		if d1.Before(target) && target.Before(d2) {
			return d1, d2
		}
		tenor := dateTenor[d]
		if _, ok := quotes[tenor]; ok {
			d1 = d2
			d2 = d
		}
	}
	return d1, d2
}

// priorPaymentDate computes the coupon date immediately preceding the settlement.
func priorPaymentDate(settlementDate, effectiveDate time.Time) time.Time {
	var candidate time.Time

	for i := 0; calendar.Adjust(calendar.KRW, utils.AddMonth(effectiveDate, 3*i)).Before(settlementDate.AddDate(0, 0, 1)); i++ {
		candidate = calendar.Adjust(calendar.KRW, utils.AddMonth(effectiveDate, 3*i))
	}

	if calendar.IsEndOfMonth(calendar.KRW, effectiveDate) {
		return calendar.LastBusinessDayOfMonth(calendar.KRW, candidate)
	}
	return candidate
}
