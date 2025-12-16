package swap_test

import (
	"math"
	"testing"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
)

func TestGenerateSchedule_SinglePeriod(t *testing.T) {
	t.Parallel()

	effective := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maturity := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	leg := market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqAnnual,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.USD,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: false,
		IncludeFinalPrincipal:   false,
	}

	periods, err := swap.GenerateSchedule(effective, maturity, leg)
	if err != nil {
		t.Fatalf("GenerateSchedule error: %v", err)
	}
	if len(periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(periods))
	}
	p := periods[0]
	if !p.StartDate.Equal(effective) {
		t.Fatalf("StartDate mismatch: got %s", p.StartDate.Format("2006-01-02"))
	}
	if !p.EndDate.Equal(maturity) {
		t.Fatalf("EndDate mismatch: got %s", p.EndDate.Format("2006-01-02"))
	}
	if !p.PayDate.Equal(maturity) {
		t.Fatalf("PayDate mismatch: got %s", p.PayDate.Format("2006-01-02"))
	}
	if p.AccrualDays != 365 {
		t.Fatalf("AccrualDays mismatch: got %d", p.AccrualDays)
	}
	if !p.FixingDate.Equal(effective) {
		t.Fatalf("FixingDate mismatch: got %s", p.FixingDate.Format("2006-01-02"))
	}
}

func TestGetDiscountFactorsAndZeroRates(t *testing.T) {
	t.Parallel()

	settlement := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maturity := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	dfs := map[time.Time]float64{
		settlement: 1.0,
		maturity:   0.95,
	}
	crv := curve.NewCurveFromDFs(settlement, dfs, calendar.USD, 0)

	out, err := swap.GetDiscountFactors(crv, []time.Time{settlement, maturity})
	if err != nil {
		t.Fatalf("GetDiscountFactors error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 DFs, got %d", len(out))
	}
	if math.Abs(out[0]-1.0) > 1e-12 {
		t.Fatalf("DF(settlement) mismatch: got %.12f", out[0])
	}
	if math.Abs(out[1]-0.95) > 1e-12 {
		t.Fatalf("DF(maturity) mismatch: got %.12f", out[1])
	}

	zs, err := swap.GetZeroRates(crv, []time.Time{maturity})
	if err != nil {
		t.Fatalf("GetZeroRates error: %v", err)
	}
	if len(zs) != 1 {
		t.Fatalf("expected 1 zero rate, got %d", len(zs))
	}
	wantZero := -math.Log(0.95) * 100.0
	if math.Abs(zs[0]-wantZero) > 1e-9 {
		t.Fatalf("ZeroRate mismatch: got %.12f want %.12f", zs[0], wantZero)
	}
}

func TestGetForwardRates_SinglePeriod(t *testing.T) {
	t.Parallel()

	effective := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maturity := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Build a projection curve with a 2% simple forward over 1Y.
	dfEnd := 1.0 / 1.02
	proj := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  dfEnd,
	}, calendar.USD, 0)

	leg := market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqAnnual,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.USD,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: false,
		IncludeFinalPrincipal:   false,
	}

	fwds, err := swap.GetForwardRates(proj, effective, maturity, leg)
	if err != nil {
		t.Fatalf("GetForwardRates error: %v", err)
	}
	if len(fwds) != 1 {
		t.Fatalf("expected 1 forward, got %d", len(fwds))
	}
	if math.Abs(fwds[0].Rate-0.02) > 1e-12 {
		t.Fatalf("forward rate mismatch: got %.12f", fwds[0].Rate)
	}
}

func TestNPVAndSolveParSpread_SinglePeriod(t *testing.T) {
	t.Parallel()

	effective := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maturity := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valuation := effective

	notional := 100.0

	leg := market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqAnnual,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.USD,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: false,
		IncludeFinalPrincipal:   false,
	}

	disc := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  0.95,
	}, calendar.USD, 0)

	// Pay projection curve: 2% forward over 1Y => DF(end) = 1 / (1 + 0.02).
	projPay := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  1.0 / 1.02,
	}, calendar.USD, 0)

	// Receive projection curve: 1% forward over 1Y => DF(end) = 1 / (1 + 0.01).
	projRec := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  1.0 / 1.01,
	}, calendar.USD, 0)

	spec := market.SwapSpec{
		Notional:      notional,
		EffectiveDate: effective,
		MaturityDate:  maturity,
		PayLeg:        leg,
		RecLeg:        leg,
	}

	npv0, err := swap.NPV(spec, projPay, projRec, disc, valuation)
	if err != nil {
		t.Fatalf("NPV error: %v", err)
	}
	// PV = DF * N * (fwd_rec - fwd_pay) = 0.95 * 100 * (0.01 - 0.02) = -0.95
	if math.Abs(npv0-(-0.95)) > 1e-12 {
		t.Fatalf("NPV mismatch: got %.12f want %.12f", npv0, -0.95)
	}

	spreadBP, err := swap.SolveParSpread(spec, projPay, projRec, disc, valuation, swap.SpreadTargetRecLeg)
	if err != nil {
		t.Fatalf("SolveParSpread error: %v", err)
	}
	if math.Abs(spreadBP-100.0) > 1e-9 {
		t.Fatalf("spread mismatch: got %.12f want 100.0", spreadBP)
	}

	spec.RecLegSpreadBP = spreadBP
	npvSolved, err := swap.NPV(spec, projPay, projRec, disc, valuation)
	if err != nil {
		t.Fatalf("NPV(solved) error: %v", err)
	}
	if math.Abs(npvSolved) > 1e-9 {
		t.Fatalf("expected solved NPV ~ 0, got %.12f", npvSolved)
	}
}

func TestSolveParSpread_JPY5x5_Tibor6M_Tonar_Fixtures20251210(t *testing.T) {
	// This is a lightweight integration test using the embedded fixtures from marketdata/.
	// It validates that the trade builder + par spread solve produces a sensible result
	// for the JPY TIBOR6M vs TONAR structure on the 2025-12-10 curve date.

	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	forwardTenorYears := 5
	swapTenorYears := 5

	oisLeg := swaps.TONARFloat

	payLeg := swaps.TIBOR6MFloat
	payLeg.PayDelayDays = 2 // match SWPM export configuration for this comparison

	recLeg := swaps.TONARFloat

	notional := 10_000_000.0

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: forwardTenorYears,
		SwapTenorYears:    swapTenorYears,
		Notional:          notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisLeg,
		OISQuotes:         basisdata.BGNTonar,
		PayLegQuotes:      basisdata.BGNSTibor6M,
	})
	if err != nil {
		t.Fatalf("InterestRateSwap error: %v", err)
	}

	gotSpread, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		t.Fatalf("SolveParSpread error: %v", err)
	}
	if math.Abs(pv.TotalPV) > 1e-6 {
		t.Fatalf("expected NPV ~ 0 at solved spread, got %.12f", pv.TotalPV)
	}

	// Sanity bound: SWPM fair receive-leg spread is ~57.33 bp for this setup.
	if gotSpread < 40 || gotSpread > 80 {
		t.Fatalf("spread out of expected range: got %.12f bp", gotSpread)
	}
}

func TestSolveParFixedRate_SinglePeriod(t *testing.T) {
	t.Parallel()

	effective := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maturity := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valuation := effective

	notional := 100.0

	fixedLeg := market.LegConvention{
		LegType:                 market.LegFixed,
		DayCount:                market.Act365F,
		PayFrequency:            market.FreqAnnual,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.USD,
		IncludeInitialPrincipal: false,
		IncludeFinalPrincipal:   false,
	}
	floatLeg := market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqAnnual,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.USD,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: false,
		IncludeFinalPrincipal:   false,
	}

	disc := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  0.95,
	}, calendar.USD, 0)

	// Receive float: 2% forward over 1Y => DF(end) = 1 / (1 + 0.02).
	projFloat := curve.NewCurveFromDFs(effective, map[time.Time]float64{
		effective: 1.0,
		maturity:  1.0 / 1.02,
	}, calendar.USD, 0)

	spec := market.SwapSpec{
		Notional:      notional,
		EffectiveDate: effective,
		MaturityDate:  maturity,
		PayLeg:        fixedLeg,
		RecLeg:        floatLeg,
	}

	fixedRateBP, err := swap.SolveParSpread(spec, nil, projFloat, disc, valuation, swap.SpreadTargetPayLeg)
	if err != nil {
		t.Fatalf("SolveParSpread error: %v", err)
	}
	if math.Abs(fixedRateBP-200.0) > 1e-9 {
		t.Fatalf("fixed rate mismatch: got %.12f want 200.0", fixedRateBP)
	}

	spec.PayLegSpreadBP = fixedRateBP
	npvSolved, err := swap.NPV(spec, nil, projFloat, disc, valuation)
	if err != nil {
		t.Fatalf("NPV(solved) error: %v", err)
	}
	if math.Abs(npvSolved) > 1e-9 {
		t.Fatalf("expected solved NPV ~ 0, got %.12f", npvSolved)
	}
}
