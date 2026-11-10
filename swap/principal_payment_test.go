package swap_test

import (
	"math"
	"testing"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
)

func TestFinalPrincipalUsesDelayedPaymentDate(t *testing.T) {
	date := calendar.MustParseDate
	effective, maturity, payment := date("2026-01-05"), date("2027-01-05"), date("2027-01-07")
	leg := market.LegConvention{LegType: market.LegFixed, DayCount: market.Act365F, PayFrequency: market.FreqAnnual, Calendar: calendar.TARGET, PayDelayDays: 2, IncludeFinalPrincipal: true}
	disc := curve.NewCurveFromDFs(effective, map[time.Time]float64{effective: 1, maturity: .95, payment: .94}, calendar.TARGET, 0)
	spec := market.SwapSpec{Notional: 100, EffectiveDate: effective, MaturityDate: maturity, PayLeg: leg, RecLeg: leg}
	for _, valuation := range []time.Time{effective, date("2027-01-06"), payment, payment.AddDate(0, 0, 1)} {
		pv, err := swap.PVByLeg(spec, nil, nil, disc, valuation)
		if err != nil {
			t.Fatal(err)
		}
		want := 94.0
		if valuation.After(payment) {
			want = 0
		}
		if math.Abs(pv.RecLegPV-want) > 1e-10 || math.Abs(pv.PayLegPV+want) > 1e-10 {
			t.Errorf("valuation %s PV=%+v want +/-%.2f", valuation.Format("2006-01-02"), pv, want)
		}
	}
}
