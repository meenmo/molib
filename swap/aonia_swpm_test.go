package swap_test

import (
	"encoding/json"
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/bootstrap"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
	"math"
	"os"
	"testing"
	"time"
)

type audSWPMFixture struct {
	CurveDate     string             `json:"curve_date"`
	ValuationDate string             `json:"valuation_date"`
	EffectiveDate string             `json:"effective_date"`
	MaturityDate  string             `json:"maturity_date"`
	Notional      float64            `json:"notional"`
	FixedRate     float64            `json:"fixed_rate_pct"`
	FixedPV       float64            `json:"fixed_leg_pv"`
	FloatPV       float64            `json:"floating_leg_pv"`
	NPV           float64            `json:"npv"`
	ParRate       float64            `json:"par_rate_pct"`
	Quotes        map[string]float64 `json:"quotes"`
	DFs           map[string]float64 `json:"curve_dfs"`
	Cashflows     []struct {
		End      string
		Pay      string
		Days     int
		Fixed    float64
		Floating float64
		DF       float64
	} `json:"cashflows"`
}

// Verifies the screenshot schedule and fixed PV independently of bootstrap.
// The remaining logs are diagnostics, not assertions of Bloomberg curve parity.
func TestAONIA_SWPM_20261110(t *testing.T) {
	raw, err := os.ReadFile("testdata/aonia_swpm_20261110.json")
	if err != nil {
		t.Fatal(err)
	}
	var f audSWPMFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	date := calendar.MustParseDate
	fixed, floating, _ := swaps.FixedFloatLegsByIndex("AONIA")
	// Both legs exchange final principal in the displayed cashflow table. The
	// effective-date exchange is excluded at valuation on the effective date.
	fixed.IncludeFinalPrincipal = true
	floating.IncludeInitialPrincipal = false
	periods, err := swap.GenerateSchedule(date(f.EffectiveDate), date(f.MaturityDate), fixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != len(f.Cashflows) {
		t.Fatalf("periods=%d want %d", len(periods), len(f.Cashflows))
	}
	for i, p := range periods {
		ref := f.Cashflows[i]
		if p.EndDate.Format("2006-01-02") != ref.End || p.PayDate.Format("2006-01-02") != ref.Pay || p.AccrualDays != ref.Days {
			t.Errorf("row %d schedule=%+v want %+v", i+1, p, ref)
		}
		cash := f.Notional * f.FixedRate / 100 * float64(p.AccrualDays) / 365
		if i == len(periods)-1 {
			cash += f.Notional
		}
		if math.Abs(cash-ref.Fixed) > 0.011 {
			t.Errorf("row %d fixed payment=%.4f want %.2f", i+1, cash, ref.Fixed)
		}
	}

	// Displayed floating coupons and DFs must reproduce the displayed float PV.
	referenceFloatPV := 0.0
	for _, ref := range f.Cashflows {
		referenceFloatPV += ref.Floating * ref.DF
	}
	if math.Abs(referenceFloatPV-f.FloatPV) > 10 {
		t.Errorf("reference float PV %.2f differs from displayed %.2f", referenceFloatPV, f.FloatPV)
	}
	// Check valuation independently of bootstrap using the displayed payment DFs.
	suppliedDFs := map[time.Time]float64{date(f.EffectiveDate): 1}
	for _, ref := range f.Cashflows {
		suppliedDFs[date(ref.Pay)] = ref.DF
	}
	suppliedCurve := curve.NewCurveFromDFs(date(f.EffectiveDate), suppliedDFs, calendar.AU, 0)
	spec := market.SwapSpec{Notional: f.Notional, EffectiveDate: date(f.EffectiveDate), MaturityDate: date(f.MaturityDate), PayLeg: fixed, RecLeg: fixed, RecLegSpreadBP: f.FixedRate * 100}
	isolated, err := swap.PVByLeg(spec, nil, nil, suppliedCurve, date(f.ValuationDate))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fixed PV with displayed DFs %.2f vs Bloomberg %.2f delta %.2f", isolated.RecLegPV, f.FixedPV, isolated.RecLegPV-f.FixedPV)
	if math.Abs(isolated.RecLegPV-f.FixedPV) > 10 {
		t.Errorf("fixed PV with displayed DFs differs by more than display rounding: %.2f", isolated.RecLegPV-f.FixedPV)
	}

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{CurveDate: date(f.CurveDate), TradeDate: date(f.CurveDate), ValuationDate: date(f.ValuationDate), EffectiveDate: date(f.EffectiveDate), MaturityDate: date(f.MaturityDate), Notional: f.Notional, PayLeg: floating, RecLeg: fixed, DiscountingOIS: floating, OISQuotes: f.Quotes, PayLegQuotes: f.Quotes, RecLegSpreadBP: f.FixedRate * 100})
	if err != nil {
		t.Fatal(err)
	}
	pv, err := trade.PVByLeg()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fixed PV %.2f (BBG %.2f delta %.2f); float PV %.2f (BBG %.2f delta %.2f); NPV %.2f (BBG %.2f delta %.2f)", pv.RecLegPV, f.FixedPV, pv.RecLegPV-f.FixedPV, pv.PayLegPV, f.FloatPV, pv.PayLegPV-f.FloatPV, pv.TotalPV, f.NPV, pv.TotalPV-f.NPV)
	// Principal PV cancels in net value; remove both before solving the par coupon.
	trade.Spec.PayLeg.IncludeFinalPrincipal = false
	trade.Spec.RecLeg.IncludeFinalPrincipal = false
	rate, _, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("par %.8f (BBG %.8f delta %.6f bp)", rate/100, f.ParRate, (rate/100-f.ParRate)*100)
	out, err := bootstrap.BootstrapParSwapCurve(bootstrap.Request{CurveDate: f.CurveDate, Index: "AONIA", CurveQuotes: f.Quotes})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range out.Nodes {
		t.Logf("%s DF %.9f BBG %.6f relative delta %.4f bp", n.Tenor, n.DF, f.DFs[n.Tenor], (n.DF/f.DFs[n.Tenor]-1)*10000)
	}
	for i, p := range periods {
		ref := f.Cashflows[i]
		cash := -f.Notional * (trade.PayProjCurve.DF(p.StartDate)/trade.PayProjCurve.DF(p.EndDate) - 1)
		if i == len(periods)-1 {
			cash -= f.Notional
		}
		t.Logf("row %d float %.2f BBG %.2f delta %.2f; pay DF %.9f BBG %.6f", i+1, cash, ref.Floating, cash-ref.Floating, trade.DiscountCurve.DF(p.PayDate), ref.DF)
	}
}

// The reset screenshot is an excerpt, not a complete daily fixing series.
// We test the known outstanding coupon directly; do not reconstruct it by
// assuming the displayed latest overnight fixing of 4.85% applies to a year.
func TestAONIA_SWPM_Seasoned20261110(t *testing.T) {
	raw, err := os.ReadFile("testdata/aonia_swpm_20261110_seasoned.json")
	if err != nil {
		t.Fatal(err)
	}
	var f audSWPMFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	date := calendar.MustParseDate
	fixed, floating, _ := swaps.FixedFloatLegsByIndex("AONIA")
	fixed.IncludeFinalPrincipal = true
	floating.IncludeInitialPrincipal = false
	ps, err := swap.GenerateSchedule(date(f.EffectiveDate), date(f.MaturityDate), fixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 31 {
		t.Fatalf("full schedule periods=%d want 31", len(ps))
	}
	if ps[0].StartDate.Format("2006-01-02") != "2025-01-10" || ps[0].EndDate.Format("2006-01-02") != "2025-11-11" {
		t.Fatalf("historical front stub: %+v", ps[0])
	}
	dfs := map[time.Time]float64{date(f.ValuationDate): 1}
	floatRefPV := 0.0
	for i, ref := range f.Cashflows {
		p := ps[i+1]
		if p.EndDate.Format("2006-01-02") != ref.End || p.PayDate.Format("2006-01-02") != ref.Pay || p.AccrualDays != ref.Days {
			t.Errorf("row %d schedule=%+v want %+v", i+1, p, ref)
		}
		amount := f.Notional * f.FixedRate / 100 * float64(p.AccrualDays) / 365
		if i == len(f.Cashflows)-1 {
			amount += f.Notional
		}
		if math.Abs(amount-ref.Fixed) > 0.011 {
			t.Errorf("row %d fixed %.4f want %.2f", i+1, amount, ref.Fixed)
		}
		dfs[date(ref.Pay)] = ref.DF
		floatRefPV += ref.Floating * ref.DF
	}
	if math.Abs(floatRefPV-f.FloatPV) > 10 {
		t.Errorf("displayed float PV %.2f vs %.2f", floatRefPV, f.FloatPV)
	}
	c := curve.NewCurveFromDFs(date(f.ValuationDate), dfs, calendar.AU, 0)
	spec := market.SwapSpec{Notional: f.Notional, EffectiveDate: date(f.EffectiveDate), MaturityDate: date(f.MaturityDate), PayLeg: fixed, RecLeg: fixed, RecLegSpreadBP: f.FixedRate * 100}
	pv, err := swap.PVByLeg(spec, nil, nil, c, date(f.ValuationDate))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("displayed DFs fixed PV %.2f vs BBG %.2f delta %.2f; displayed float sum %.2f delta %.2f", pv.RecLegPV, f.FixedPV, pv.RecLegPV-f.FixedPV, floatRefPV, floatRefPV-f.FloatPV)
	if math.Abs(pv.RecLegPV-f.FixedPV) > 10 {
		t.Errorf("fixed PV outside display-rounding tolerance")
	}
	if math.Abs(pv.RecLegPV+floatRefPV-f.NPV) > 1 {
		t.Errorf("reference net PV %.2f vs %.2f", pv.RecLegPV+floatRefPV, f.NPV)
	}
	// Price only outstanding periods, supplying the observed fully accrued coupon
	// as an equivalent annual rate. This is a diagnostic, not a fixing engine.
	knownCouponPct := -f.Cashflows[0].Floating / f.Notional * 100
	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{CurveDate: date(f.CurveDate), TradeDate: date(f.CurveDate), ValuationDate: date(f.ValuationDate), EffectiveDate: date("2025-11-11"), MaturityDate: date(f.MaturityDate), Notional: f.Notional, PayLeg: floating, RecLeg: fixed, DiscountingOIS: floating, OISQuotes: f.Quotes, PayLegQuotes: f.Quotes, PayLegFirstResetPct: &knownCouponPct, RecLegSpreadBP: f.FixedRate * 100})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := trade.PVByLeg()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("bootstrap with known current coupon: fixed %.2f delta %.2f; float %.2f delta %.2f; net %.2f vs BBG %.2f delta %.2f", actual.RecLegPV, actual.RecLegPV-f.FixedPV, actual.PayLegPV, actual.PayLegPV-f.FloatPV, actual.TotalPV, f.NPV, actual.TotalPV-f.NPV)
	// Cashflow consistency checks above are asserted; bootstrap parity remains a
	// diagnostic until input-curve discrepancies are resolved.
}
