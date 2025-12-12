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

// forwardRateTenorAlignedLocal matches basis.forwardRateTenorAligned for IBOR legs.
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

// forwardRateLocal is the simple DF-based forward used for OIS legs.
func forwardRateLocal(curve *basis.Curve, start, end time.Time, dayCount string) float64 {
	dfStart := curve.DF(start)
	dfEnd := curve.DF(end)
	alpha := utils.YearFraction(start, end, dayCount)
	if alpha == 0 {
		return 0
	}
	return (dfStart/dfEnd - 1.0) / alpha
}

func main() {
	// SWPM setup (per user):
	// - Trade date:     2025-12-12
	// - Curve date:     2025-12-10
	// - Valuation date: 2025-12-12
	// - Notional:       10,000,000
	// - Pay:    TIBOR6M (spread = 0)
	// - Receive: TONAR  (spread = 57.351 bp)
	//
	// This diagnostic uses:
	//   - SWPM-style spot: T+2 from trade date on JPN calendar,
	//   - Both legs ACT/360 day count,
	//   - Pay leg payDelayDays = 2 (to match SWPM coupon pay dates),
	//   - TONAR OIS curve for discounting and TONAR projection,
	//   - BGNS TIBOR6M for the pay leg projection.
	//
	// It prints molib's cashflows and PVs for direct comparison
	// against the SWPM cashflow export at the same spread.

	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	notional := 10_000_000.0
	recSpreadBP := 57.351
	recSpreadDec := recSpreadBP * 1e-4

	// Leg conventions: start from presets, override to match SWPM.
	payLeg := benchmark.TIBOR6MFloat
	payLeg.DayCount = benchmark.Act360
	// SWPM cashflows show a two-business-day payment delay on the TIBOR leg.
	payLeg.PayDelayDays = 2

	recLeg := benchmark.TONARFloat
	recLeg.DayCount = benchmark.Act360

	// Discounting on TONAR OIS (BGN).
	oisLeg := benchmark.TONARFloat
	oisQuotes := basisdata.BGNTonar

	// Pay-leg IBOR curve: TIBOR6M from BGNS (via alias).
	payQuotes := basisdata.BGNSTibor6M

	// Forward and swap tenors: 5x5.
	forwardTenorYears := 5
	swapTenorYears := 5

	// SWPM-style dates: spot is T+2 from trade date.
	spotDate := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	unadjEff := spotDate.AddDate(forwardTenorYears, 0, 0)
	effective := calendar.AdjustFollowing(oisLeg.Calendar, unadjEff)
	unadjMat := effective.AddDate(swapTenorYears, 0, 0)
	maturity := calendar.AdjustFollowing(oisLeg.Calendar, unadjMat)

	// Build curves at curve date.
	discCurve := basis.BuildCurve(curveDate, oisQuotes, oisLeg.Calendar, 1)
	projPay := basis.BuildDualCurve(curveDate, payQuotes, discCurve, payLeg.Calendar, int(payLeg.PayFrequency))
	projRec := discCurve

	fmt.Println("=== JPY 5x5 TIBOR6M vs TONAR – molib diagnostics at SWPM spread ===")
	fmt.Printf("Curve date:     %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Trade date:     %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("Valuation date: %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("Spot date:      %s\n", spotDate.Format("2006-01-02"))
	fmt.Printf("Effective date: %s\n", effective.Format("2006-01-02"))
	fmt.Printf("Maturity date:  %s\n", maturity.Format("2006-01-02"))
	fmt.Printf("Notional:       %.0f\n", notional)
	fmt.Printf("Rec spread:     %.6f bp\n\n", recSpreadBP)

	// Build schedules.
	payPeriods := buildScheduleLocal(effective, maturity, payLeg)
	recPeriods := buildScheduleLocal(effective, maturity, recLeg)

	// Pay leg: TIBOR6M, ACT/360, semi-annual.
	fmt.Println("[Pay leg (TIBOR6M) cashflows, spread = 0]")
	fmt.Println("PayDate,AccrualStart,AccrualEnd,AccrualDays,Notional,Principal,ResetDate,ResetRate,Payment,Discount,ZeroRate,PV")

	var pvPay float64

	// Initial principal at effective (direction = PAY).
	if !effective.Before(valuationDate) && payLeg.IncludeInitialPrincipal {
		dfEff := discCurve.DF(effective)
		principal := notional
		pv := principal * dfEff
		pvPay += pv
		fmt.Printf("%s,,,,,%.0f,,,,%.0f,%.6f,?,?,%.0f\n",
			effective.Format("01/02/2006"),
			principal,
			principal,
			dfEff,
			pv,
		)
	}

	for _, p := range payPeriods {
		if p.PaymentDate.Before(valuationDate) {
			continue
		}
		accrDays := utils.Days(p.AccrualStart, p.AccrualEnd)
		accr := accrDays / 360.0

		fwd := forwardRateTenorAlignedLocal(projPay, p.AccrualStart, payLeg, string(payLeg.DayCount))
		df := discCurve.DF(p.PaymentDate)
		zero := projPay.ZeroRateAt(p.PaymentDate)

		coupon := -notional * fwd * accr
		pv := coupon * df
		pvPay += pv

		fmt.Printf("%s,%s,%s,%.0f,%.0f,0,%s,%.5f,%.0f,%.6f,%.6f,%.0f\n",
			p.PaymentDate.Format("01/02/2006"),
			p.AccrualStart.Format("01/02/2006"),
			p.AccrualEnd.Format("01/02/2006"),
			accrDays,
			-notional,
			p.ResetDate.Format("01/02/2006"),
			fwd*100.0,
			coupon,
			df,
			zero,
			pv,
		)
	}

	// Final principal at maturity (direction = PAY).
	if !maturity.Before(valuationDate) && payLeg.IncludeFinalPrincipal {
		dfMat := discCurve.DF(maturity)
		principal := -notional
		pv := principal * dfMat
		pvPay += pv
		fmt.Printf("%s,,,,,%.0f,,,,%.0f,%.6f,?,?,%.0f\n",
			maturity.Format("01/02/2006"),
			principal,
			principal,
			dfMat,
			pv,
		)
	}

	fmt.Printf("\nPay-leg total PV: %.2f\n\n", pvPay)

	// Receive leg: TONAR, ACT/360, annual, with SWPM spread applied.
	fmt.Println("[Receive leg (TONAR OIS) cashflows, spread = 57.351 bp]")
	fmt.Println("PayDate,AccrualStart,AccrualEnd,AccrualDays,Notional,Principal,ResetDate,EquivalentCoupon,Payment,Discount,ZeroRate,PV")

	var pvRec float64

	// Initial principal at effective (direction = RECEIVE).
	if !effective.Before(valuationDate) && recLeg.IncludeInitialPrincipal {
		dfEff := discCurve.DF(effective)
		principal := -notional
		pv := principal * dfEff
		pvRec += pv
		fmt.Printf("%s,,,,,%.0f,,,%.0f,%.6f,?,?,%.0f\n",
			effective.Format("01/02/2006"),
			principal,
			principal,
			dfEff,
			pv,
		)
	}

	for _, p := range recPeriods {
		if p.PaymentDate.Before(valuationDate) {
			continue
		}
		accrDays := utils.Days(p.AccrualStart, p.AccrualEnd)
		accr := accrDays / 360.0

		fwd := forwardRateLocal(projRec, p.AccrualStart, p.AccrualEnd, string(recLeg.DayCount))
		rate := fwd + recSpreadDec

		df := discCurve.DF(p.PaymentDate)
		zero := discCurve.ZeroRateAt(p.PaymentDate)

		coupon := notional * rate * accr
		pv := coupon * df
		pvRec += pv

		fmt.Printf("%s,%s,%s,%.0f,%.0f,0,%s,%.5f,%.0f,%.6f,%.6f,%.0f\n",
			p.PaymentDate.Format("01/02/2006"),
			p.AccrualStart.Format("01/02/2006"),
			p.AccrualEnd.Format("01/02/2006"),
			accrDays,
			notional,
			p.AccrualEnd.Format("01/02/2006"),
			rate*100.0,
			coupon,
			df,
			zero,
			pv,
		)
	}

	// Final principal at maturity (direction = RECEIVE).
	if !maturity.Before(valuationDate) && recLeg.IncludeFinalPrincipal {
		dfMat := discCurve.DF(maturity)
		principal := notional
		pv := principal * dfMat
		pvRec += pv
		fmt.Printf("%s,,,,,%.0f,,,%.0f,%.6f,?,?,%.0f\n",
			maturity.Format("01/02/2006"),
			principal,
			principal,
			dfMat,
			pv,
		)
	}

	fmt.Printf("\nReceive-leg total PV: %.2f\n\n", pvRec)

	totalPV := pvRec + pvPay
	fmt.Printf("Total swap PV (rec TONAR @ 57.351 bp, pay TIBOR6M): %.2f\n", totalPV)
}

