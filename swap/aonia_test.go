package swap_test

import (
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"math"
	"testing"
)

func TestAONIATradeAndSchedule(t *testing.T) {
	fixed, floating, _ := swaps.FixedFloatLegsByIndex("AONIA")
	floating.IncludeInitialPrincipal = false
	floating.IncludeFinalPrincipal = false
	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		CurveDate: calendar.MustParseDate("2026-11-10"),
		TradeDate: calendar.MustParseDate("2026-11-10"),
		Notional:  1e6, SwapTenorYears: 5, PayLeg: fixed, RecLeg: floating,
		RecLegQuotes:   map[string]float64{"1M": 4.1, "1Y": 4.2, "2Y": 4.3, "5Y": 4.6895},
		DiscountingOIS: floating, OISQuotes: map[string]float64{"1M": 4.1, "1Y": 4.2, "2Y": 4.3, "5Y": 4.6895},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := trade.SpotDate.Format("2006-01-02"); got != "2026-11-11" {
		t.Fatalf("spot = %s", got)
	}
	if got := trade.Spec.MaturityDate.Format("2006-01-02"); got != "2031-11-11" {
		t.Fatalf("maturity = %s", got)
	}
	rate, pv, err := trade.SolveParSpread(swap.SpreadTargetPayLeg)
	if err != nil || math.IsNaN(rate) || rate < 400 || rate > 500 || math.Abs(pv.TotalPV) > 1e-6 {
		t.Fatalf("rate=%v pv=%+v err=%v", rate, pv, err)
	}
	periods, err := swap.GenerateSchedule(calendar.MustParseDate("2025-12-24"), calendar.MustParseDate("2026-12-24"), floating)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 {
		t.Fatalf("period count = %d", len(periods))
	}
	p := periods[0]
	if p.PayDate.Format("2006-01-02") != "2026-12-30" || p.FixingDate.Format("2006-01-02") != "2026-12-23" {
		t.Fatalf("wrong payment/cutoff dates: %+v", p)
	}
}
