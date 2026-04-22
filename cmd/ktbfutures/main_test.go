package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadCanonicalInput(t *testing.T) ktbInput {
	t.Helper()
	path := filepath.Join("testdata", "input.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var in ktbInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return in
}

// TestCanonicalInput validates the canonical testdata/input.json (3Y near-month
// A6566000 on 2026-04-13) end-to-end through the CLI's process function.
func TestCanonicalInput(t *testing.T) {
	in := loadCanonicalInput(t)

	// Schema sanity
	if in.Tenor != 3 {
		t.Fatalf("tenor=%d, want 3", in.Tenor)
	}
	if in.FuturesCode != "A6566000" {
		t.Fatalf("futures_code=%q, want A6566000", in.FuturesCode)
	}
	if !in.IsNearMonth {
		t.Fatalf("is_near_month=%v, want true", in.IsNearMonth)
	}
	if in.MarketPrice == nil {
		t.Fatalf("market_price is nil, expected populated")
	}
	if len(in.OnTheRunKTB) == 0 {
		t.Fatalf("on_the_run_ktb is empty, expected populated")
	}
	if len(in.KTBCurve) != 20 {
		t.Fatalf("ktb_curve has %d nodes, want 20", len(in.KTBCurve))
	}
	if len(in.Basket) != 3 {
		t.Fatalf("basket has %d bonds, want 3 for 3Y", len(in.Basket))
	}

	out, err := process(in)
	if err != nil {
		t.Fatalf("process: %v", err)
	}

	// Fair value must match pricing.ktb_futures reference for 3Y on 2026-04-13.
	const wantFair = 104.36573469
	if math.Abs(out.FairValue-wantFair) > 5e-4 {
		t.Errorf("fair_value=%.8f want=%.8f (diff=%.2e)",
			out.FairValue, wantFair, math.Abs(out.FairValue-wantFair))
	}

	// KRD: exactly 20 entries ascending by tenor.
	if len(out.KRD) != 20 {
		t.Fatalf("KRD has %d entries, want 20", len(out.KRD))
	}
	for i := 1; i < len(out.KRD); i++ {
		if out.KRD[i].Tenor <= out.KRD[i-1].Tenor {
			t.Errorf("KRD not strictly ascending at index %d (%.4f <= %.4f)",
				i, out.KRD[i].Tenor, out.KRD[i-1].Tenor)
		}
	}

	// Basis = market_price - fair_value (exact).
	if out.Basis == nil {
		t.Errorf("basis is nil despite market_price populated")
	} else {
		want := *in.MarketPrice - out.FairValue
		if math.Abs(*out.Basis-want) > 1e-9 {
			t.Errorf("basis=%.12f want %.12f", *out.Basis, want)
		}
	}

	// On-off finite when on_the_run_ktb populated.
	if out.OnOffSpread == nil {
		t.Errorf("onoff_spread is nil despite on_the_run_ktb populated")
	} else if math.IsNaN(*out.OnOffSpread) || math.IsInf(*out.OnOffSpread, 0) {
		t.Errorf("onoff_spread=%v is not finite", *out.OnOffSpread)
	}

	// Theta finite.
	if math.IsNaN(out.Theta) || math.IsInf(out.Theta, 0) {
		t.Errorf("theta=%v is not finite", out.Theta)
	}

	t.Logf("fair=%.8f basis=%.6f onoff=%.6f theta=%.3e",
		out.FairValue, *out.Basis, *out.OnOffSpread, out.Theta)
}

// TestOptionalFieldsNil verifies that omitting market_price produces basis=nil
// (JSON null) and omitting on_the_run_ktb produces onoff_spread=nil.
func TestOptionalFieldsNil(t *testing.T) {
	in := loadCanonicalInput(t)
	in.MarketPrice = nil
	in.OnTheRunKTB = nil

	out, err := process(in)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if out.Basis != nil {
		t.Errorf("basis = %v, want nil when market_price absent", *out.Basis)
	}
	if out.OnOffSpread != nil {
		t.Errorf("onoff_spread = %v, want nil when on_the_run_ktb absent", *out.OnOffSpread)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, needle := range []string{`"basis":null`, `"onoff_spread":null`, `"market_price":null`} {
		if !strings.Contains(s, needle) {
			t.Errorf("JSON missing %q; got %s", needle, s)
		}
	}
}
