package basis

import (
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/benchmark"
)

// CalculateSpread builds curves and solves for receive-leg spread.
func CalculateSpread(curveDate time.Time, forwardTenorYears int, swapTenorYears int, payLeg benchmark.LegConvention, recLeg benchmark.LegConvention, oisLeg benchmark.LegConvention, oisQuotes map[string]float64, payQuotes map[string]float64, recQuotes map[string]float64, notional float64) (float64, SwapPV) {
	tradeDate := curveDate
	spot := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	effective := calendar.AddYearsWithRoll(oisLeg.Calendar, spot, forwardTenorYears)
	maturity := calendar.AddYearsWithRoll(oisLeg.Calendar, effective, swapTenorYears)

	// Build OIS discount curve (single-curve is correct for OIS)
	discCurve := BuildCurve(curveDate, oisQuotes, oisLeg.Calendar, 1)

	// Build IBOR projection curves (temporarily using single-curve for testing)
	projPay := BuildCurve(curveDate, payQuotes, payLeg.Calendar, int(payLeg.PayFrequency))
	projRec := BuildCurve(curveDate, recQuotes, recLeg.Calendar, int(recLeg.PayFrequency))

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
