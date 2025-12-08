package basis

import (
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/benchmark"
)

// CalculateSpread builds curves and solves for receive-leg spread.
func CalculateSpread(curveDate time.Time, forwardTenorYears int, swapTenorYears int, payLeg benchmark.LegConvention, recLeg benchmark.LegConvention, oisLeg benchmark.LegConvention, oisQuotes map[string]float64, payQuotes map[string]float64, recQuotes map[string]float64, notional float64) (float64, SwapPV) {
	// Apply T+2 spot convention
	spotDate := calendar.AddBusinessDays(oisLeg.Calendar, curveDate, 2)

	// Calculate effective date from spot (not trade date)
	var effective time.Time
	if forwardTenorYears > 0 {
		unadjEff := spotDate.AddDate(forwardTenorYears, 0, 0)
		effective = calendar.AdjustFollowing(oisLeg.Calendar, unadjEff)
	} else {
		effective = spotDate
	}
	unadjMat := effective.AddDate(swapTenorYears, 0, 0)
	maturity := calendar.AdjustFollowing(oisLeg.Calendar, unadjMat)

	// Build OIS discount curve (single-curve is correct for OIS)
	discCurve := BuildCurve(curveDate, oisQuotes, oisLeg.Calendar, 1)

	// Build IBOR projection curves using dual-curve bootstrap
	projPay := BuildDualCurve(curveDate, payQuotes, discCurve, payLeg.Calendar, int(payLeg.PayFrequency))
	projRec := BuildDualCurve(curveDate, recQuotes, discCurve, recLeg.Calendar, int(recLeg.PayFrequency))

	spec := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  effective,
		MaturityDate:   maturity,
		PayLeg:         payLeg,
		RecLeg:         recLeg,
		DiscountingOIS: oisLeg,
	}

	spread, pv := solveReceiveSpread(spec, projPay, projRec, discCurve, curveDate)
	return spread, pv
}
