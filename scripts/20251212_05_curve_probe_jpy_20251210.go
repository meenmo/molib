package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/basis"
	basisdata "github.com/meenmo/molib/swap/basis/data"
	"github.com/meenmo/molib/swap/benchmark"
	"github.com/meenmo/molib/utils"
)

// localPeriod mirrors the Period type used in swap/basis/schedule.go.
type localPeriod struct {
	AccrualStart time.Time
	AccrualEnd   time.Time
	PaymentDate  time.Time
	ResetDate    time.Time
}

// buildScheduleLocal reproduces the basis.buildSchedule logic for this probe.
func buildScheduleLocal(effective, maturity time.Time, leg benchmark.LegConvention) []localPeriod {
	periods := []localPeriod{}
	months := int(leg.PayFrequency)
	start := effective
	for {
		var next time.Time
		if leg.RollConvention == benchmark.BackwardEOM {
			next = utils.AddMonth(start, months)
		} else {
			next = start.AddDate(0, months, 0)
		}
		if next.After(maturity.AddDate(0, 0, 1)) {
			break
		}
		accrualEnd := calendar.Adjust(leg.Calendar, next)
		resetDate := calendar.AddBusinessDays(leg.Calendar, calendar.Adjust(leg.Calendar, start), -leg.FixingLagDays)
		paymentDate := calendar.AddBusinessDays(leg.Calendar, accrualEnd, leg.PayDelayDays)
		periods = append(periods, localPeriod{
			AccrualStart: calendar.Adjust(leg.Calendar, start),
			AccrualEnd:   accrualEnd,
			PaymentDate:  paymentDate,
			ResetDate:    resetDate,
		})
		start = next
	}
	return periods
}

// forwardRateTenorAlignedLocal matches the IBOR forward logic in basis.
func forwardRateTenorAlignedLocal(proj *basis.Curve, start time.Time, leg benchmark.LegConvention, dayCount string) float64 {
	tenorMonths := int(leg.PayFrequency)
	tenorEnd := start.AddDate(0, tenorMonths, 0)
	tenorEnd = calendar.Adjust(leg.Calendar, tenorEnd)

	dfStart := proj.DF(start)
	dfEnd := proj.DF(tenorEnd)

	alpha := utils.YearFraction(start, tenorEnd, dayCount)
	if alpha == 0 {
		return 0
	}
	return (dfStart/dfEnd - 1.0) / alpha
}

func main() {
	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)

	// Use JPN calendar for TONAR / TIBOR curves.
	cal := calendar.JPN

	// Build TONAR OIS curve (discounting) and TIBOR6M dual curve.
	discCurve := basis.BuildCurve(curveDate, basisdata.BGNTonar, cal, 1)

	tiborLeg := benchmark.TIBOR6MFloat
	projTibor6M := basis.BuildDualCurve(curveDate, basisdata.BGNSTibor6M, discCurve, tiborLeg.Calendar, int(tiborLeg.PayFrequency))

	fmt.Println("=== Curve probe: JPY TONAR OIS and TIBOR6M on 2025-12-10 ===")
	fmt.Printf("Curve date: %s\n\n", curveDate.Format("2006-01-02"))

	// Key dates from the SWPM 5x5 export (pay / receive leg payment dates).
	keyDates := []time.Time{
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2031, 6, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2031, 12, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2032, 6, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2032, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2033, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2033, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2034, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2034, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2035, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2035, 12, 19, 0, 0, 0, 0, time.UTC),
	}

	fmt.Println("[TONAR OIS curve: discount factors and zero rates]")
	fmt.Println("Date, DF, ZeroRatePct")
	for _, d := range keyDates {
		df := discCurve.DF(d)
		z := discCurve.ZeroRateAt(d)
		fmt.Printf("%s,%.9f,%.9f\n", d.Format("01/02/2006"), df, z)
	}
	fmt.Println()

	// Now probe TIBOR6M forwards on the same 5x5 structure:
	// - Trade date: 2025-12-12
	// - Spot: T+2 on JPN calendar
	// - Effective: spot + 5Y, maturity: +5Y
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	spot := calendar.AddBusinessDays(cal, tradeDate, 2)
	unadjEff := spot.AddDate(5, 0, 0)
	eff := calendar.AdjustFollowing(cal, unadjEff)
	unadjMat := eff.AddDate(5, 0, 0)
	mat := calendar.AdjustFollowing(cal, unadjMat)

	payLeg := tiborLeg
	payPeriods := buildScheduleLocal(eff, mat, payLeg)

	fmt.Println("[TIBOR6M projection curve: forwards on 5x5 pay periods]")
	fmt.Println("ResetDate,AccrualStart,AccrualEnd,AccrualDays,FwdRatePct")
	for _, p := range payPeriods {
		accrDays := utils.Days(p.AccrualStart, p.AccrualEnd)
		fwd := forwardRateTenorAlignedLocal(projTibor6M, p.AccrualStart, payLeg, string(payLeg.DayCount))
		fmt.Printf("%s,%s,%s,%.0f,%.5f\n",
			p.ResetDate.Format("01/02/2006"),
			p.AccrualStart.Format("01/02/2006"),
			p.AccrualEnd.Format("01/02/2006"),
			accrDays,
			fwd*100.0,
		)
	}
}

