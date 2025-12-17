package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
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
func buildScheduleLocal(effective, maturity time.Time, leg market.LegConvention) []localPeriod {
	periods := []localPeriod{}
	months := int(leg.PayFrequency)
	start := effective
	for {
		var next time.Time
		if leg.RollConvention == market.BackwardEOM {
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

// forwardRateLocal matches the forward logic in swap/basis/valuation.go.
func forwardRateLocal(crv *curve.Curve, start, end time.Time, dayCount string) float64 {
	dfStart := crv.DF(start)
	dfEnd := crv.DF(end)
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
	// - Receive: TONAR  (spread = 0)
	//
	// This diagnostic computes molib's cashflows and PVs for the same
	// configuration, using the mixed-source fixtures:
	//   - TONAR OIS from BGN (BGNTonar),
	//   - TIBOR6M from BGNS (BGNSTibor6M alias).

	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	notional := 10_000_000.0

	// Leg conventions: start from presets, override daycount to ACT/360 on both legs
	// to match the SWPM cashflows (accrualDays / 360).
	payLeg := swaps.TIBOR6MFloat
	payLeg.DayCount = market.Act360

	recLeg := swaps.TONARFloat
	recLeg.DayCount = market.Act360

	// Discounting on TONAR OIS (BGN)
	oisLeg := swaps.TONARFloat
	oisQuotes := basisdata.BGNTonar

	// Pay-leg IBOR curve: TIBOR6M from BGNS (via alias)
	payQuotes := basisdata.BGNSTibor6M

	// Rec-leg projection: use the same TONAR OIS curve as for discounting
	// (no separate quotes needed; projRec will be the discount curve).

	// Forward and swap tenors: 5x5
	forwardTenorYears := 5
	swapTenorYears := 5

	// Date construction: T+2 from trade date on JPN calendar, then
	// add years and apply Following (as in CalculateSpread).
	spotDate := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	unadjEff := spotDate.AddDate(forwardTenorYears, 0, 0)
	effective := calendar.AdjustFollowing(oisLeg.Calendar, unadjEff)
	unadjMat := effective.AddDate(swapTenorYears, 0, 0)
	maturity := calendar.AdjustFollowing(oisLeg.Calendar, unadjMat)

	// Build curves at curve date (settlement = curve date).
	discCurve := curve.BuildCurve(curveDate, oisQuotes, oisLeg.Calendar, 1)
	projPay := curve.BuildProjectionCurve(curveDate, payLeg, payQuotes, discCurve)
	// TONAR projection uses the same OIS curve.
	projRec := discCurve

	fmt.Println("=== JPY 5x5 TIBOR6M vs TONAR – molib zero-spread diagnostics ===")
	fmt.Printf("Curve date:     %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Trade date:     %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("Valuation date: %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("Spot date:      %s\n", spotDate.Format("2006-01-02"))
	fmt.Printf("Effective date: %s\n", effective.Format("2006-01-02"))
	fmt.Printf("Maturity date:  %s\n", maturity.Format("2006-01-02"))
	fmt.Printf("Notional:       %.0f\n\n", notional)

	// Build schedules.
	payPeriods := buildScheduleLocal(effective, maturity, payLeg)
	recPeriods := buildScheduleLocal(effective, maturity, recLeg)

	// Pay leg: TIBOR6M, ACT/360, semi-annual.
	fmt.Println("[Pay leg (TIBOR6M) cashflows, spread = 0]")
	fmt.Println("PayDate, AccrualStart, AccrualEnd, AccrualDays, Notional, Principal, ResetDate, ResetRate, Payment, Discount, ZeroRate, PV")

	var pvPay float64

	// Initial principal at effective (same sign convention as basis.priceLeg, direction = PAY).
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

		// Forward rate from TIBOR6M dual curve.
		fwd := forwardRateLocal(projPay, p.AccrualStart, p.AccrualEnd, string(payLeg.DayCount))
		df := discCurve.DF(p.PaymentDate)
		zero := projPay.ZeroRateAt(p.PaymentDate)

		// Direction PAY ⇒ negative coupon cashflow.
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

	// Receive leg: TONAR, ACT/360, annual.
	fmt.Println("[Receive leg (TONAR OIS) cashflows, spread = 0]")
	fmt.Println("PayDate, AccrualStart, AccrualEnd, AccrualDays, Notional, Principal, ResetDate, EquivalentCoupon, Payment, Discount, ZeroRate, PV")

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

		// Approximate compounded TONAR coupon via simple forward on OIS curve.
		fwd := forwardRateLocal(projRec, p.AccrualStart, p.AccrualEnd, string(recLeg.DayCount))
		df := discCurve.DF(p.PaymentDate)
		zero := discCurve.ZeroRateAt(p.PaymentDate)

		coupon := notional * fwd * accr
		pv := coupon * df
		pvRec += pv

		fmt.Printf("%s,%s,%s,%.0f,%.0f,0,%s,%.5f,%.0f,%.6f,%.6f,%.0f\n",
			p.PaymentDate.Format("01/02/2006"),
			p.AccrualStart.Format("01/02/2006"),
			p.AccrualEnd.Format("01/02/2006"),
			accrDays,
			notional,
			p.AccrualEnd.Format("01/02/2006"),
			fwd*100.0,
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
	fmt.Printf("Total swap PV (rec TONAR, pay TIBOR6M, zero spreads): %.2f\n", totalPV)
}
