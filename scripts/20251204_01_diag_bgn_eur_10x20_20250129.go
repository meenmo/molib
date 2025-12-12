package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/basis"
	"github.com/meenmo/molib/swap/basis/data"
	"github.com/meenmo/molib/swap/benchmark"
	"github.com/meenmo/molib/utils"
)

// localPeriod mirrors basis.Period for diagnostics.
type localPeriod struct {
	AccrualStart time.Time
	AccrualEnd   time.Time
	PaymentDate  time.Time
	ResetDate    time.Time
}

// buildScheduleLocal copies basis.buildSchedule for local inspection.
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

// forwardRateTenorAlignedLocal mirrors basis.forwardRateTenorAligned.
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
	// Hard-coded scenario: BGN EUR 10x20, curve/valuation date 2025-01-29.
	curveDate := time.Date(2025, time.January, 29, 0, 0, 0, 0, time.UTC)

	oisLeg := benchmark.ESTRFloat
	payLeg := benchmark.EURIBOR6MFloat
	recLeg := benchmark.EURIBOR3MFloat

	tradeDate := curveDate
	spotDate := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)

	unadjEff := spotDate.AddDate(10, 0, 0)
	effective := calendar.AdjustFollowing(oisLeg.Calendar, unadjEff)
	unadjMat := effective.AddDate(20, 0, 0)
	maturity := calendar.AdjustFollowing(oisLeg.Calendar, unadjMat)

	discCurve := basis.BuildCurve(curveDate, data.BGNEstr, oisLeg.Calendar, 1)
	projPay := basis.BuildDualCurve(curveDate, data.BGNEuribor6M, discCurve, payLeg.Calendar, int(payLeg.PayFrequency))
	projRec := basis.BuildDualCurve(curveDate, data.BGNEuribor3M, discCurve, recLeg.Calendar, int(recLeg.PayFrequency))

	spec := benchmark.SwapSpec{
		Notional:       10_000_000.0,
		EffectiveDate:  effective,
		MaturityDate:   maturity,
		PayLeg:         payLeg,
		RecLeg:         recLeg,
		DiscountingOIS: oisLeg,
	}

	// Sanity check: reproduce molib spread.
	spreadBP, _ := basis.CalculateSpread(
		curveDate,
		10, 20,
		payLeg,
		recLeg,
		oisLeg,
		data.BGNEstr,
		data.BGNEuribor6M,
		data.BGNEuribor3M,
		spec.Notional,
	)

	fmt.Println("=== BGN EUR 10x20 diagnostics for 2025-01-29 (molib stack) ===")
	fmt.Printf("Curve/valuation date: %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Spot date (T+2):      %s\n", spotDate.Format("2006-01-02"))
	fmt.Printf("Effective date:       %s\n", effective.Format("2006-01-02"))
	fmt.Printf("Maturity date:        %s\n", maturity.Format("2006-01-02"))
	fmt.Printf("Molib spread:         %.6f bp\n\n", spreadBP)

	// Build local schedules (identical logic to basis.buildSchedule).
	payPeriods := buildScheduleLocal(effective, maturity, payLeg)
	recPeriods := buildScheduleLocal(effective, maturity, recLeg)

	// Aggregates for closed-form spread.
	var (
		PPay        float64 // pay-leg total PV (incl. principal)
		BRec        float64 // sum δ f D for rec leg (per unit notional)
		ARec        float64 // sum δ D for rec leg (per unit notional)
		RPrinRec    float64 // rec-leg principal PV
		notional    = spec.Notional
		recDayCount = string(recLeg.DayCount)
		payDayCount = string(payLeg.DayCount)
	)

	// Buckets for rec leg by payment time from curve date.
	type bucketAgg struct {
		A float64
		B float64
	}
	var (
		b10to15 bucketAgg
		b15to30 bucketAgg
		other   bucketAgg
	)

	valuation := curveDate

	fmt.Println("Pay leg (6M) key periods near long end (s=0):")
	for idx, p := range payPeriods {
		if p.PaymentDate.Before(valuation) {
			continue
		}
		accrual := utils.YearFraction(p.AccrualStart, p.AccrualEnd, payDayCount)
		fwd := forwardRateTenorAlignedLocal(projPay, p.AccrualStart, payLeg, payDayCount)
		df := discCurve.DF(p.PaymentDate)
		cf := -notional * accrual * fwd // pay leg, spread = 0
		pv := cf * df
		PPay += pv

		// Print only last few periods to focus on long end.
		if idx >= len(payPeriods)-4 {
			fmt.Printf("  [%2d] %s -> %s | PayDate %s | accr=%.8f | fwd=%.6f%% | DF=%.8f | pv=%.2f\n",
				idx+1,
				p.AccrualStart.Format("2006-01-02"),
				p.AccrualEnd.Format("2006-01-02"),
				p.PaymentDate.Format("2006-01-02"),
				accrual,
				fwd*100.0,
				df,
				pv,
			)
		}
	}

	fmt.Println("\nRec leg (3M) key periods near long end (s=0):")
	for idx, p := range recPeriods {
		if p.PaymentDate.Before(valuation) {
			continue
		}
		accrual := utils.YearFraction(p.AccrualStart, p.AccrualEnd, recDayCount)
		fwd := forwardRateTenorAlignedLocal(projRec, p.AccrualStart, recLeg, recDayCount)
		df := discCurve.DF(p.PaymentDate)

		// Per-unit-notional contributions
		dB := accrual * fwd * df
		dA := accrual * df

		BRec += dB
		ARec += dA

		// Bucket by time-to-payment from curve date (in years).
		tYears := utils.Days(curveDate, p.PaymentDate) / 365.0
		switch {
		case tYears >= 10.0 && tYears < 15.0:
			b10to15.A += dA
			b10to15.B += dB
		case tYears >= 15.0 && tYears <= 30.0:
			b15to30.A += dA
			b15to30.B += dB
		default:
			other.A += dA
			other.B += dB
		}

		cf := notional * accrual * fwd
		pv := cf * df

		// Print only last few periods to focus on long end.
		if idx >= len(recPeriods)-4 {
			fmt.Printf("  [%2d] %s -> %s | PayDate %s | accr=%.8f | fwd=%.6f%% | DF=%.8f | pv=%.2f\n",
				idx+1,
				p.AccrualStart.Format("2006-01-02"),
				p.AccrualEnd.Format("2006-01-02"),
				p.PaymentDate.Format("2006-01-02"),
				accrual,
				fwd*100.0,
				df,
				pv,
			)
		}
	}

	// Principal PVs (same pattern as basis.priceLeg).
	// Pay leg principals (direction = PAY).
	if payLeg.IncludeInitialPrincipal && !effective.Before(valuation) {
		df := discCurve.DF(effective)
		PPay += notional * df
	}
	if payLeg.IncludeFinalPrincipal && !maturity.Before(valuation) {
		df := discCurve.DF(maturity)
		PPay -= notional * df
	}
	// Rec leg principals (direction = RECEIVE).
	if recLeg.IncludeInitialPrincipal && !effective.Before(valuation) {
		df := discCurve.DF(effective)
		RPrinRec -= notional * df
	}
	if recLeg.IncludeFinalPrincipal && !maturity.Before(valuation) {
		df := discCurve.DF(maturity)
		RPrinRec += notional * df
	}

	fmt.Println("\nAggregates for closed-form spread (molib stack, s in bp):")
	fmt.Printf("  P_pay       = %.6f (per unit notional)\n", PPay/notional)
	fmt.Printf("  B_rec       = %.6f (per unit notional)\n", BRec)
	fmt.Printf("  A_rec       = %.6f (per unit notional)\n", ARec)
	fmt.Printf("  R_prin_rec  = %.6f (per unit notional)\n", RPrinRec/notional)

	sDec := -(PPay/notional + BRec + RPrinRec/notional) / ARec
	sBP := sDec * 10_000.0
	fmt.Printf("\n  Implied spread from aggregates = %.6f bp\n", sBP)
	fmt.Printf("  Direct molib CalculateSpread   = %.6f bp\n", spreadBP)

	// Bucket breakdown for rec leg.
	fmt.Println("\nRec leg bucketed contributions (per unit notional):")
	fmt.Printf("  10-15y: A = %.6f, B = %.6f, A%% = %.2f%%, B%% = %.2f%%\n",
		b10to15.A, b10to15.B,
		100.0*b10to15.A/ARec,
		100.0*b10to15.B/BRec,
	)
	fmt.Printf("  15-30y: A = %.6f, B = %.6f, A%% = %.2f%%, B%% = %.2f%%\n",
		b15to30.A, b15to30.B,
		100.0*b15to30.A/ARec,
		100.0*b15to30.B/BRec,
	)
	fmt.Printf("  other:  A = %.6f, B = %.6f, A%% = %.2f%%, B%% = %.2f%%\n",
		other.A, other.B,
		100.0*other.A/ARec,
		100.0*other.B/BRec,
	)
}
