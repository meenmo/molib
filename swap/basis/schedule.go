package basis

import (
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/benchmark"
)

type Period struct {
	AccrualStart time.Time
	AccrualEnd   time.Time
	PaymentDate  time.Time
	ResetDate    time.Time
}

func buildSchedule(effective, maturity time.Time, leg benchmark.LegConvention) []Period {
	periods := []Period{}
	months := int(leg.PayFrequency)
	start := effective
	for {
		next := start.AddDate(0, months, 0)
		if next.After(maturity.AddDate(0, 0, 1)) {
			break
		}
		accrualEnd := calendar.Adjust(leg.Calendar, next)
		resetDate := calendar.AddBusinessDays(leg.Calendar, calendar.Adjust(leg.Calendar, start), -leg.FixingLagDays)
		paymentDate := calendar.AddBusinessDays(leg.Calendar, accrualEnd, leg.PayDelayDays)
		periods = append(periods, Period{
			AccrualStart: calendar.Adjust(leg.Calendar, start),
			AccrualEnd:   accrualEnd,
			PaymentDate:  paymentDate,
			ResetDate:    resetDate,
		})
		start = next
	}
	return periods
}
