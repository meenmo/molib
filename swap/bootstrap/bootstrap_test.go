package bootstrap

import (
	"math"
	"strings"
	"testing"
)

func TestBootstrapParSwapCurveESTR(t *testing.T) {
	req := estrRequest()
	out, err := BootstrapParSwapCurve(req)
	if err != nil {
		t.Fatalf("BootstrapParSwapCurve error: %v", err)
	}
	if out.Index != "ESTR" {
		t.Fatalf("Index = %q, want ESTR", out.Index)
	}
	if out.SettlementDate != "2026-05-22" {
		t.Fatalf("SettlementDate = %q, want 2026-05-22", out.SettlementDate)
	}
	if len(out.Nodes) != len(req.CurveQuotes) {
		t.Fatalf("nodes = %d, want %d", len(out.Nodes), len(req.CurveQuotes))
	}
	prev := ""
	for _, n := range out.Nodes {
		if n.DF <= 0 {
			t.Fatalf("%s DF = %v, want positive", n.Tenor, n.DF)
		}
		if prev != "" && n.Date < prev {
			t.Fatalf("nodes not sorted: %s before %s", n.Date, prev)
		}
		prev = n.Date
		if strings.HasSuffix(n.Tenor, "W") {
			if math.IsNaN(n.RepricingErrorBP) || math.IsInf(n.RepricingErrorBP, 0) {
				t.Fatalf("%s repricing error = %.12f bp, want finite", n.Tenor, n.RepricingErrorBP)
			}
			continue
		}
		if math.Abs(n.RepricingErrorBP) > 0.005 {
			t.Fatalf("%s repricing error = %.12f bp, want near zero", n.Tenor, n.RepricingErrorBP)
		}
	}
}

func TestBootstrapParSwapCurveSpotRatesDeriveParRates(t *testing.T) {
	req := estrRequest()
	req.QuoteType = "spot"
	out, err := BootstrapParSwapCurve(req)
	if err != nil {
		t.Fatalf("BootstrapParSwapCurve error: %v", err)
	}
	if out.QuoteType != "spot" {
		t.Fatalf("QuoteType = %q, want spot", out.QuoteType)
	}
	var checked bool
	for _, n := range out.Nodes {
		if n.DF <= 0 {
			t.Fatalf("%s DF = %v, want positive", n.Tenor, n.DF)
		}
		if n.Tenor == "5Y" {
			checked = true
			if math.Abs(n.ParPct-req.CurveQuotes[n.Tenor]) < 1e-6 {
				t.Fatalf("5Y par_pct = %.8f unexpectedly echoes spot input %.8f", n.ParPct, req.CurveQuotes[n.Tenor])
			}
		}
	}
	if !checked {
		t.Fatal("5Y node not found")
	}
}

func TestBootstrapParSwapCurveRejectsUnknownQuoteType(t *testing.T) {
	req := estrRequest()
	req.QuoteType = "zero"
	_, err := BootstrapParSwapCurve(req)
	if err == nil || !strings.Contains(err.Error(), "unknown quote_type") {
		t.Fatalf("unknown quote_type error = %v", err)
	}
}

func TestBootstrapParSwapCurveExplicitSettlement(t *testing.T) {
	req := estrRequest()
	req.SettlementDate = "2026-05-25"
	out, err := BootstrapParSwapCurve(req)
	if err != nil {
		t.Fatalf("BootstrapParSwapCurve error: %v", err)
	}
	if out.SettlementDate != "2026-05-25" {
		t.Fatalf("SettlementDate = %q, want explicit date", out.SettlementDate)
	}
}

func TestBootstrapParSwapCurveValidation(t *testing.T) {
	req := estrRequest()
	req.Index = "UNKNOWN"
	_, err := BootstrapParSwapCurve(req)
	if err == nil || !strings.Contains(err.Error(), "unknown index") {
		t.Fatalf("unknown index error = %v", err)
	}

	req = estrRequest()
	req.CurveQuotes["BAD"] = 1.0
	_, err = BootstrapParSwapCurve(req)
	if err == nil || !strings.Contains(err.Error(), "parse tenor") {
		t.Fatalf("malformed tenor error = %v", err)
	}
}

func TestBootstrapParSwapCurveKRX(t *testing.T) {
	out, err := BootstrapParSwapCurve(Request{
		CurveDate: "2026-05-20",
		Currency:  "KRW",
		Index:     "CD91D",
		CurveQuotes: map[string]float64{
			"91D": 2.76,
			"6M":  2.72,
			"1Y":  2.73,
			"2Y":  2.81,
		},
	})
	if err != nil {
		t.Fatalf("BootstrapParSwapCurve error: %v", err)
	}
	if out.Index != "CD91D" {
		t.Fatalf("Index = %q, want CD91D", out.Index)
	}
	if len(out.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4", len(out.Nodes))
	}
	for _, n := range out.Nodes {
		if n.DF <= 0 {
			t.Fatalf("%s DF = %v, want positive", n.Tenor, n.DF)
		}
		if math.Abs(n.RepricingErrorBP) > 0.005 {
			t.Fatalf("%s repricing error = %.12f bp, want near zero", n.Tenor, n.RepricingErrorBP)
		}
	}
}

func estrRequest() Request {
	return Request{
		CurveDate: "2026-05-20",
		Currency:  "EUR",
		Index:     "ESTR",
		CurveQuotes: map[string]float64{
			"1W":  1.9325,
			"2W":  1.9324,
			"1M":  1.9682,
			"2M":  2.0594,
			"3M":  2.1307,
			"4M":  2.1782,
			"5M":  2.2369,
			"6M":  2.2884,
			"7M":  2.3276,
			"8M":  2.3729,
			"9M":  2.4111,
			"10M": 2.4422,
			"11M": 2.4746,
			"1Y":  2.5049,
			"18M": 2.5932,
			"2Y":  2.6302,
			"3Y":  2.66225,
			"4Y":  2.6975,
			"5Y":  2.74035,
			"6Y":  2.7859,
			"7Y":  2.8357,
			"8Y":  2.88625,
			"9Y":  2.93595,
		},
	}
}
