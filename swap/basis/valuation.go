package basis

import (
	"math"
	"time"

	"github.com/meenmo/molib/swap/benchmark"
	"github.com/meenmo/molib/utils"
)

type LegPV struct {
	PV       float64
	Cashflow float64
}

type SwapPV struct {
	PayLegPV float64
	RecLegPV float64
	TotalPV  float64
}

// forward from projection curve between accrual start/end.
func forwardRate(proj *Curve, start, end time.Time, dayCount string) float64 {
	dfStart := proj.DF(start)
	dfEnd := proj.DF(end)
	yearFrac := utils.YearFraction(start, end, dayCount)
	if yearFrac == 0 {
		return 0
	}
	return (dfStart/dfEnd - 1.0) / yearFrac
}

func priceLeg(spec benchmark.SwapSpec, leg benchmark.LegConvention, proj *Curve, disc *Curve, valuation time.Time, direction string, spreadBP float64) LegPV {
	periods := buildSchedule(spec.EffectiveDate, spec.MaturityDate, leg)
	spread := spreadBP * 1e-4
	totalPV := 0.0
	for _, p := range periods {
		if p.PaymentDate.Before(valuation) {
			continue
		}
		accrual := utils.YearFraction(p.AccrualStart, p.AccrualEnd, string(leg.DayCount))
		fwd := forwardRate(proj, p.AccrualStart, p.AccrualEnd, string(leg.DayCount))
		rate := fwd + spread
		payment := spec.Notional * accrual * rate
		sign := 1.0
		if direction == "PAY" {
			sign = -1.0
		}
		df := disc.DF(p.PaymentDate)
		pv := sign * payment * df
		totalPV += pv
	}
	// principals
	if leg.IncludeInitialPrincipal && !spec.EffectiveDate.Before(valuation) {
		sign := 1.0
		if direction == "RECEIVE" {
			sign = -1.0
		}
		df := disc.DF(spec.EffectiveDate)
		totalPV += sign * spec.Notional * df
	}
	if leg.IncludeFinalPrincipal && !spec.MaturityDate.Before(valuation) {
		sign := -1.0
		if direction == "RECEIVE" {
			sign = 1.0
		}
		df := disc.DF(spec.MaturityDate)
		totalPV += sign * spec.Notional * df
	}
	return LegPV{PV: totalPV}
}

func priceSwap(spec benchmark.SwapSpec, projPay *Curve, projRec *Curve, disc *Curve, valuation time.Time) SwapPV {
	pay := priceLeg(spec, spec.PayLeg, projPay, disc, valuation, "PAY", spec.PayLegSpreadBP)
	rec := priceLeg(spec, spec.RecLeg, projRec, disc, valuation, "RECEIVE", spec.RecLegSpreadBP)
	return SwapPV{
		PayLegPV: pay.PV,
		RecLegPV: rec.PV,
		TotalPV:  rec.PV + pay.PV,
	}
}

// solveReceiveSpread finds spread to zero NPV.
func solveReceiveSpread(spec benchmark.SwapSpec, projPay *Curve, projRec *Curve, disc *Curve, valuation time.Time) (float64, SwapPV) {
	lower := -500.0
	upper := 500.0
	tol := 1e-3
	var mid float64
	var res SwapPV
	f := func(spread float64) float64 {
		spec.RecLegSpreadBP = spread
		res = priceSwap(spec, projPay, projRec, disc, valuation)
		return res.TotalPV
	}
	lowVal := f(lower)
	upVal := f(upper)
	if math.Abs(lowVal) < tol {
		return lower, res
	}
	if math.Abs(upVal) < tol {
		return upper, res
	}
	for i := 0; i < 100; i++ {
		mid = 0.5 * (lower + upper)
		midVal := f(mid)
		if math.Abs(midVal) < tol {
			return mid, res
		}
		if lowVal*midVal <= 0 {
			upper = mid
			upVal = midVal
		} else {
			lower = mid
			lowVal = midVal
		}
	}
	return mid, res
}
