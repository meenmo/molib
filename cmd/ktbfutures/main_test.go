package main

import (
	"testing"
)

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestIntegration(t *testing.T) {
	input := ktbInput{
		Date: "2026-03-12",
		CD91: 2.83,
		Baskets: []basketInput{
			{Tenor: 3, Bonds: []bondInput{
				{ISIN: "KR103501GEC6", IssueDate: "2024-12-10", MaturityDate: "2027-12-10", CouponRate: 2.875, MarketYield: 3.03},
				{ISIN: "KR103501GF63", IssueDate: "2025-06-10", MaturityDate: "2028-06-10", CouponRate: 2.25, MarketYield: 3.19},
				{ISIN: "KR103503GF95", IssueDate: "2025-09-10", MaturityDate: "2030-09-10", CouponRate: 2.5, MarketYield: 3.524},
			}},
			{Tenor: 5, Bonds: []bondInput{
				{ISIN: "KR103503GF38", IssueDate: "2025-03-10", MaturityDate: "2030-03-10", CouponRate: 2.625, MarketYield: 3.467},
				{ISIN: "KR103503GF95", IssueDate: "2025-09-10", MaturityDate: "2030-09-10", CouponRate: 2.5, MarketYield: 3.524},
			}},
			{Tenor: 10, Bonds: []bondInput{
				{ISIN: "KR103502GEC4", IssueDate: "2024-12-10", MaturityDate: "2034-12-10", CouponRate: 3.0, MarketYield: 3.626},
				{ISIN: "KR103502GF62", IssueDate: "2025-06-10", MaturityDate: "2035-06-10", CouponRate: 2.625, MarketYield: 3.68},
			}},
			{Tenor: 30, Bonds: []bondInput{
				{ISIN: "KR103502GF39", IssueDate: "2025-03-10", MaturityDate: "2055-03-10", CouponRate: 2.625, MarketYield: 3.572},
				{ISIN: "KR103502GF96", IssueDate: "2025-09-10", MaturityDate: "2055-09-10", CouponRate: 2.625, MarketYield: 3.565},
			}},
		},
	}

	expected := map[int]float64{
		3:  104.964157,
		5:  106.837720,
		10: 111.186792,
		30: 126.222322,
	}

	out, err := process(input)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}

	if out.FuturesExpiry != "2026-03-17" {
		t.Errorf("futures_expiry = %s, want 2026-03-17", out.FuturesExpiry)
	}

	for _, r := range out.Results {
		want, ok := expected[r.Tenor]
		if !ok {
			t.Errorf("unexpected tenor %d", r.Tenor)
			continue
		}
		diff := absFloat(r.FairValue - want)
		if diff > 1e-4 {
			t.Errorf("tenor %dY: fair_value=%.6f, want=%.6f, diff=%.2e", r.Tenor, r.FairValue, want, diff)
		} else {
			t.Logf("tenor %dY: fair_value=%.6f (diff=%.2e)", r.Tenor, r.FairValue, diff)
		}
	}
}
