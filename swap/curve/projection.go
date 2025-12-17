package curve

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
)

// BuildProjectionCurve returns a projection curve for the given leg.
//
// For overnight indices (e.g., TONAR/ESTR/SOFR), the discount curve is also the projection curve.
// For IBOR indices, it builds a dual curve bootstrapped using OIS discounting.
func BuildProjectionCurve(curveDate time.Time, leg market.LegConvention, legQuotes map[string]float64, discount *Curve) *Curve {
	if market.IsOvernight(leg.ReferenceRate) {
		return discount
	}
	if discount == nil {
		panic("BuildProjectionCurve: nil discount curve")
	}
	if legQuotes == nil {
		panic(fmt.Sprintf("BuildProjectionCurve: nil quotes for %s", leg.ReferenceRate))
	}
	// Use the leg's pay frequency for the floating leg periods in bootstrap,
	// but use monthly grid for pillar interpolation (matches OIS curve precision).
	return BuildDualCurveWithFreq(curveDate, legQuotes, discount, leg.Calendar, int(leg.PayFrequency), 1)
}

// BuildDualCurveWithFreq creates an IBOR projection curve with separate control over
// the floating leg frequency (for bootstrap) and the pillar grid frequency (for interpolation).
func BuildDualCurveWithFreq(settlement time.Time, iborQuotes map[string]float64, oisCurve *Curve, cal calendar.CalendarID, floatFreqMonths, gridFreqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range iborQuotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement:    settlement,
		parQuotes:     parsed,
		cal:           cal,
		freqMonths:    gridFreqMonths, // Use finer grid for interpolation
		curveDayCount: oisCurve.curveDayCount,
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDualCurveDiscountFactorsWithFloatFreq(oisCurve, floatFreqMonths)
	c.zeros = c.buildZero()
	return c
}
