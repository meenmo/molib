package ktb_test

import (
	"math"
	"testing"
	"time"

	"github.com/meenmo/molib/bond/ktb"
	"github.com/meenmo/molib/calendar"
)

func mustParse(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestThirdTuesday(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  string
	}{
		{2026, time.March, "2026-03-17"},
		{2026, time.June, "2026-06-16"},
		{2026, time.September, "2026-09-15"},
		{2026, time.December, "2026-12-15"},
		{2032, time.September, "2032-09-17"}, // Chuseok: 09-20 and 09-21 are holidays
	}
	for _, tc := range cases {
		got := calendar.ThirdTuesday(tc.year, tc.month)
		if got.Format("2006-01-02") != tc.want {
			t.Errorf("ThirdTuesday(%d, %v) = %s, want %s",
				tc.year, tc.month, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestKTBFuturesExpiry(t *testing.T) {
	cases := []struct {
		today string
		want  string
	}{
		{"2026-03-12", "2026-03-17"},
		{"2026-03-18", "2026-06-16"},
		{"2026-01-15", "2026-03-17"},
		{"2026-04-01", "2026-06-16"},
	}
	for _, tc := range cases {
		got := calendar.KTBFuturesExpiry(mustParse(tc.today))
		if got.Format("2006-01-02") != tc.want {
			t.Errorf("KTBFuturesExpiry(%s) = %s, want %s",
				tc.today, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestKTBFuturesExpiryFarMonth(t *testing.T) {
	// Far month = ThirdTuesday of the quarter 3 months after the near-month expiry.
	// Verify against ThirdTuesday directly.
	cases := []struct {
		today   string
		wantFar string
	}{
		// near=2026-03-17 (March), far=ThirdTuesday(June 2026)=2026-06-16
		{"2026-03-12", calendar.ThirdTuesday(2026, time.June).Format("2006-01-02")},
		// near=2026-06-16 (June), far=ThirdTuesday(Sep 2026)=2026-09-15
		{"2026-03-18", calendar.ThirdTuesday(2026, time.September).Format("2006-01-02")},
		{"2026-04-13", calendar.ThirdTuesday(2026, time.September).Format("2006-01-02")},
		{"2026-04-01", calendar.ThirdTuesday(2026, time.September).Format("2006-01-02")},
	}
	for _, tc := range cases {
		got := calendar.KTBFuturesExpiryFarMonth(mustParse(tc.today))
		if got.Format("2006-01-02") != tc.wantFar {
			t.Errorf("KTBFuturesExpiryFarMonth(%s) = %s, want %s",
				tc.today, got.Format("2006-01-02"), tc.wantFar)
		}
	}
}

func TestKTBCashflows(t *testing.T) {
	issue := mustParse("2024-12-10")
	maturity := mustParse("2027-12-10")
	flows := ktb.KTBCashflows(issue, maturity, 2.875)

	if len(flows) != 6 {
		t.Fatalf("expected 6 cashflows, got %d", len(flows))
	}

	expectedDates := []string{
		"2025-06-10", "2025-12-10", "2026-06-10",
		"2026-12-10", "2027-06-10", "2027-12-10",
	}
	for i, cf := range flows {
		if cf.Date.Format("2006-01-02") != expectedDates[i] {
			t.Errorf("cashflow[%d] date = %s, want %s",
				i, cf.Date.Format("2006-01-02"), expectedDates[i])
		}
	}

	// couponAmt = 10000 * 2.875 / 200 = 143.75
	for i := 0; i < 5; i++ {
		if math.Abs(flows[i].Amount()-143.75) > 1e-9 {
			t.Errorf("cashflow[%d] amount = %f, want 143.75", i, flows[i].Amount())
		}
	}
	if math.Abs(flows[5].Amount()-10143.75) > 1e-9 {
		t.Errorf("last cashflow amount = %f, want 10143.75", flows[5].Amount())
	}
}

func TestKTBAdjacentPaymentDates(t *testing.T) {
	issue := mustParse("2024-12-10")
	maturity := mustParse("2027-12-10")
	flows := ktb.KTBCashflows(issue, maturity, 2.875)

	p1, n1 := ktb.KTBAdjacentPaymentDates(mustParse("2025-01-01"), flows, issue)
	if p1.Format("2006-01-02") != "2024-12-10" || n1.Format("2006-01-02") != "2025-06-10" {
		t.Errorf("case1: got (%s, %s), want (2024-12-10, 2025-06-10)",
			p1.Format("2006-01-02"), n1.Format("2006-01-02"))
	}

	p2, n2 := ktb.KTBAdjacentPaymentDates(mustParse("2026-03-12"), flows, issue)
	if p2.Format("2006-01-02") != "2025-12-10" || n2.Format("2006-01-02") != "2026-06-10" {
		t.Errorf("case2: got (%s, %s), want (2025-12-10, 2026-06-10)",
			p2.Format("2006-01-02"), n2.Format("2006-01-02"))
	}
}

func TestKTBMarketPrice(t *testing.T) {
	issue := mustParse("2024-12-10")
	maturity := mustParse("2027-12-10")
	pricingDate := mustParse("2026-03-12")
	flows := ktb.KTBCashflows(issue, maturity, 2.875)

	prev, next := ktb.KTBAdjacentPaymentDates(pricingDate, flows, issue)
	remaining := 0
	for _, cf := range flows {
		if cf.Date.After(pricingDate) {
			remaining++
		}
	}

	price := ktb.KTBMarketPrice(0.0303, 2.875, prev, next, pricingDate, remaining)
	if math.Abs(price-10045.925438) > 0.01 {
		t.Errorf("KTBMarketPrice = %.6f, want ~10045.925438", price)
	}
}

func TestKTBDerivativeFiniteDiff(t *testing.T) {
	issue := mustParse("2024-12-10")
	maturity := mustParse("2027-12-10")
	pricingDate := mustParse("2026-03-12")
	flows := ktb.KTBCashflows(issue, maturity, 2.875)
	prev, next := ktb.KTBAdjacentPaymentDates(pricingDate, flows, issue)
	remaining := 0
	for _, cf := range flows {
		if cf.Date.After(pricingDate) {
			remaining++
		}
	}

	y := 0.0303
	h := 1e-7
	_, dP := ktb.KTBMarketPriceAndDeriv(y, 2.875, prev, next, pricingDate, remaining)
	pPlus := ktb.KTBMarketPrice(y+h, 2.875, prev, next, pricingDate, remaining)
	pMinus := ktb.KTBMarketPrice(y-h, 2.875, prev, next, pricingDate, remaining)
	finiteDiff := (pPlus - pMinus) / (2 * h)

	if math.Abs(dP-finiteDiff) > 1.0 {
		t.Errorf("analytic = %.6f, finite diff = %.6f, diff = %.6f", dP, finiteDiff, math.Abs(dP-finiteDiff))
	}
}

func TestKTBSolveImpliedYield(t *testing.T) {
	issue := mustParse("2024-12-10")
	maturity := mustParse("2027-12-10")
	pricingDate := mustParse("2026-03-12")
	flows := ktb.KTBCashflows(issue, maturity, 2.875)
	prev, next := ktb.KTBAdjacentPaymentDates(pricingDate, flows, issue)

	remaining := 0
	for _, cf := range flows {
		if cf.Date.After(pricingDate) {
			remaining++
		}
	}

	knownYield := 0.0303
	price := ktb.KTBMarketPrice(knownYield, 2.875, prev, next, pricingDate, remaining)

	solved, iters, err := ktb.KTBSolveImpliedYield(price, 2.875, prev, next, pricingDate, remaining)
	if err != nil {
		t.Fatalf("KTBSolveImpliedYield failed: %v", err)
	}
	if math.Abs(solved-knownYield) > 1e-10 {
		t.Errorf("solved = %.12f, want %.12f (diff=%.2e, iters=%d)",
			solved, knownYield, math.Abs(solved-knownYield), iters)
	}
	t.Logf("converged in %d iterations, yield=%.12f", iters, solved)
}

func TestComputeKTBFairValues(t *testing.T) {
	input := ktb.KTBFuturesFairValueInput{
		Date: mustParse("2026-03-12"),
		CD91: 2.83,
		Baskets: []ktb.KTBFuturesBasket{
			{Tenor: 3, Bonds: []ktb.KTBBond{
				{ISIN: "KR103501GEC6", IssueDate: mustParse("2024-12-10"), MaturityDate: mustParse("2027-12-10"), CouponRate: 2.875, MarketYield: 3.03},
				{ISIN: "KR103501GF63", IssueDate: mustParse("2025-06-10"), MaturityDate: mustParse("2028-06-10"), CouponRate: 2.25, MarketYield: 3.19},
				{ISIN: "KR103503GF95", IssueDate: mustParse("2025-09-10"), MaturityDate: mustParse("2030-09-10"), CouponRate: 2.5, MarketYield: 3.524},
			}},
			{Tenor: 5, Bonds: []ktb.KTBBond{
				{ISIN: "KR103503GF38", IssueDate: mustParse("2025-03-10"), MaturityDate: mustParse("2030-03-10"), CouponRate: 2.625, MarketYield: 3.467},
				{ISIN: "KR103503GF95", IssueDate: mustParse("2025-09-10"), MaturityDate: mustParse("2030-09-10"), CouponRate: 2.5, MarketYield: 3.524},
			}},
			{Tenor: 10, Bonds: []ktb.KTBBond{
				{ISIN: "KR103502GEC4", IssueDate: mustParse("2024-12-10"), MaturityDate: mustParse("2034-12-10"), CouponRate: 3.0, MarketYield: 3.626},
				{ISIN: "KR103502GF62", IssueDate: mustParse("2025-06-10"), MaturityDate: mustParse("2035-06-10"), CouponRate: 2.625, MarketYield: 3.68},
			}},
			{Tenor: 30, Bonds: []ktb.KTBBond{
				{ISIN: "KR103502GF39", IssueDate: mustParse("2025-03-10"), MaturityDate: mustParse("2055-03-10"), CouponRate: 2.625, MarketYield: 3.572},
				{ISIN: "KR103502GF96", IssueDate: mustParse("2025-09-10"), MaturityDate: mustParse("2055-09-10"), CouponRate: 2.625, MarketYield: 3.565},
			}},
		},
	}

	expected := map[int]float64{
		3:  104.964157,
		5:  106.837720,
		10: 111.186792,
		30: 126.222322,
	}

	results, err := ktb.ComputeKTBFuturesFairValues(input)
	if err != nil {
		t.Fatalf("ComputeKTBFuturesFairValues failed: %v", err)
	}

	for _, r := range results {
		want := expected[r.Tenor]
		diff := math.Abs(r.FairValue - want)
		if diff > 1e-4 {
			t.Errorf("tenor %dY: fair_value=%.6f, want=%.6f, diff=%.2e", r.Tenor, r.FairValue, want, diff)
		} else {
			t.Logf("tenor %dY: fair_value=%.6f (diff=%.2e)", r.Tenor, r.FairValue, diff)
		}
	}
}
