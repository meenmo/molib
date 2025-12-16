package curve

import (
	"fmt"
	"time"

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
	return BuildDualCurve(curveDate, legQuotes, discount, leg.Calendar, int(leg.PayFrequency))
}
