package curve_test

import (
	"math"
	"testing"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
)

func TestBuildCurve_RedundantInterpolatedNodeHasSmallImpactOnForwardParRate(t *testing.T) {
	settlement := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	valuationDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	effectiveDate := time.Date(2028, 3, 14, 0, 0, 0, 0, time.UTC)
	maturityDate := time.Date(2033, 3, 14, 0, 0, 0, 0, time.UTC)

	baseQuotes := map[string]float64{
		"1Y":  2.06795,
		"3Y":  2.24,
		"5Y":  2.3495,
		"7Y":  2.484,
		"10Y": 2.6955,
		"20Y": 2.98995,
		"30Y": 2.9435,
	}
	withTwoYearQuotes := map[string]float64{
		"1Y":  2.06795,
		"2Y":  2.153975,
		"3Y":  2.24,
		"5Y":  2.3495,
		"7Y":  2.484,
		"10Y": 2.6955,
		"20Y": 2.98995,
		"30Y": 2.9435,
	}

	baseCurve := curve.BuildCurve(settlement, baseQuotes, calendar.TARGET, 1)
	withTwoYearCurve := curve.BuildCurve(settlement, withTwoYearQuotes, calendar.TARGET, 1)

	spec := market.SwapSpec{
		EffectiveDate: effectiveDate,
		MaturityDate:  maturityDate,
	}

	baseParRate, err := swap.ComputeOISParRateWithDiscount(spec, baseCurve, baseCurve, valuationDate, swaps.ESTRFloating)
	if err != nil {
		t.Fatalf("base par rate: %v", err)
	}
	withTwoYearParRate, err := swap.ComputeOISParRateWithDiscount(spec, withTwoYearCurve, withTwoYearCurve, valuationDate, swaps.ESTRFloating)
	if err != nil {
		t.Fatalf("2Y par rate: %v", err)
	}

	diffBP := math.Abs(withTwoYearParRate-baseParRate) * 10000
	if diffBP > 0.02 {
		t.Fatalf("redundant 2Y node moved 2Yx5Y par rate by %.6f bp", diffBP)
	}
}
