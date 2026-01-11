package swap_test

import (
	"math"
	"testing"
	"time"

	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
)

func TestSOFROIS_SWPM_20260109(t *testing.T) {
	t.Parallel()

	// Bloomberg SWPM reference (USD SOFR ICVS, curve/val date 2026-01-09):
	// - Receive fixed 4.00% vs pay SOFR OIS, USD 10mm
	// - Effective 2026-01-13 (T+2), maturity 2056-01-13 (30Y)
	// - SWPM shows Par Cpn ~4.142295% and NPV ~-245,328.65 (receive fixed perspective).
	curveDate := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	tradeDate := curveDate
	valuationDate := curveDate

	oisQuotes := map[string]float64{
		"1W":  3.665,
		"2W":  3.66557,
		"3W":  3.67903,
		"1M":  3.67838,
		"2M":  3.68495,
		"3M":  3.67135,
		"4M":  3.65825,
		"5M":  3.64705,
		"6M":  3.62495,
		"7M":  3.6,
		"8M":  3.57745,
		"9M":  3.55325,
		"10M": 3.52975,
		"11M": 3.50815,
		"1Y":  3.48945,
		"18M": 3.39045,
		"2Y":  3.3717,
		"3Y":  3.3905,
		"4Y":  3.4369,
		"5Y":  3.49207,
		"6Y":  3.55575,
		"7Y":  3.62035,
		"8Y":  3.6828,
		"9Y":  3.74325,
		"10Y": 3.8005,
		"12Y": 3.90795,
		"15Y": 4.036,
		"20Y": 4.148,
		"25Y": 4.16755,
		"30Y": 4.14229,
		"40Y": 4.04628,
		"50Y": 3.94227,
	}

	const (
		notional      = 10_000_000.0
		fixedRatePct  = 4.0
		floatSpreadBP = 0.0
	)

	payLeg := swaps.SOFRFloating
	recLeg := swaps.SOFRFixed
	discLeg := swaps.SOFRFloating

	// Keep principal exchanges off for both legs so the total NPV reflects the
	// interest PV difference (principal exchanges cancel in SWPM outputs).
	payLeg.IncludeInitialPrincipal = false
	payLeg.IncludeFinalPrincipal = false
	recLeg.IncludeInitialPrincipal = false
	recLeg.IncludeFinalPrincipal = false
	discLeg.IncludeInitialPrincipal = false
	discLeg.IncludeFinalPrincipal = false

	// Trader receives fixed => pay float / receive fixed.
	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: 0,
		SwapTenorYears:    30,
		Notional:          notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    discLeg,
		OISQuotes:         oisQuotes,
		PayLegQuotes:      oisQuotes,
		PayLegSpreadBP:    floatSpreadBP,
		RecLegSpreadBP:    fixedRatePct * 100.0, // percent -> bp
	})
	if err != nil {
		t.Fatalf("InterestRateSwap error: %v", err)
	}

	pv, err := trade.PVByLeg()
	if err != nil {
		t.Fatalf("PVByLeg error: %v", err)
	}

	const (
		wantNPV = -245_328.65
		tolNPV  = 3_000.0
	)
	if math.Abs(pv.TotalPV-wantNPV) > tolNPV {
		t.Fatalf("SOFR OIS NPV mismatch: got %.2f want %.2f (tol %.2f)", pv.TotalPV, wantNPV, tolNPV)
	}

	// Validate key SWPM discount factors (converted to spot->pay) are close to the built curve.
	//
	// SWPM cashflow tables show discount factors from valuation date (curve date) to pay date.
	// Convert to spot->pay by dividing by DF(valuation->spot) shown in SWPM (~0.999593 at 2026-01-13).
	const swpmDFValToSpot = 0.999593
	for _, row := range []struct {
		payDate     time.Time
		dfValToPay  float64
		maxRelDiffB float64
	}{
		{payDate: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), dfValToPay: 0.965267, maxRelDiffB: 10},
		{payDate: time.Date(2028, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.934219, maxRelDiffB: 10},
		{payDate: time.Date(2029, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.902729, maxRelDiffB: 10},
		{payDate: time.Date(2030, 1, 16, 0, 0, 0, 0, time.UTC), dfValToPay: 0.871207, maxRelDiffB: 10},
		{payDate: time.Date(2035, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.712333, maxRelDiffB: 10},
		{payDate: time.Date(2040, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.566246, maxRelDiffB: 10},
		{payDate: time.Date(2045, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.448666, maxRelDiffB: 10},
		{payDate: time.Date(2050, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.360970, maxRelDiffB: 10},
		{payDate: time.Date(2056, 1, 18, 0, 0, 0, 0, time.UTC), dfValToPay: 0.285038, maxRelDiffB: 10},
	} {
		wantSpotToPay := row.dfValToPay / swpmDFValToSpot
		gotSpotToPay := trade.DiscountCurve.DF(row.payDate)

		relDiffB := math.Abs(gotSpotToPay/wantSpotToPay-1.0) * 1e4
		if relDiffB > row.maxRelDiffB {
			t.Fatalf("DF mismatch at %s: got %.9f want %.9f (rel %.3f bp, max %.3f bp)",
				row.payDate.Format("2006-01-02"),
				gotSpotToPay,
				wantSpotToPay,
				relDiffB,
				row.maxRelDiffB,
			)
		}
	}

	// Validate par fixed rate against SWPM (tight-ish tolerance; remaining gap is typically due to quote rounding).
	tradePar, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: 0,
		SwapTenorYears:    30,
		Notional:          notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    discLeg,
		OISQuotes:         oisQuotes,
		PayLegQuotes:      oisQuotes,
		PayLegSpreadBP:    floatSpreadBP,
		RecLegSpreadBP:    0,
	})
	if err != nil {
		t.Fatalf("InterestRateSwap(par) error: %v", err)
	}

	parBP, _, err := tradePar.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		t.Fatalf("SolveParSpread error: %v", err)
	}
	gotParPct := parBP / 100.0

	const (
		wantParPct = 4.142295
		tolParBP   = 0.5
	)
	if math.Abs((gotParPct-wantParPct)*100.0) > tolParBP {
		t.Fatalf("SOFR OIS par fixed mismatch: got %.6f%% want %.6f%% (tol %.3f bp)", gotParPct, wantParPct, tolParBP)
	}
}
