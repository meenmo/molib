package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// tenorNearFairValue collects the near-month fair value per tenor during
// TestAllFixturesParity so we can assert near != far for same-basket tenors.
var tenorNearFairValue = map[int]float64{}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// expectedNearMonthFairValue is the hard-coded pricing.ktb_futures fair value
// per tenor for the near-month contract on 2026-04-13. Tests use these as the
// reference; far-month contracts fall within 1.0 price point of the near-month
// value of the same tenor.
var expectedNearMonthFairValue = map[int]float64{
	3:  104.36573469,
	5:  106.41362070,
	10: 110.50301820,
	30: 125.30515444,
}

func loadFixture(t *testing.T, futuresCode string) ktbInput {
	t.Helper()
	path := filepath.Join("testdata", fmt.Sprintf("input_20260413_%s.json", futuresCode))
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

func TestAllFixturesParity(t *testing.T) {
	fixtures := []struct {
		code   string
		tenor  int
		isNear bool
	}{
		{"A6566000", 3, true},
		{"A6569000", 3, false},
		{"A6666000", 5, true},
		{"A6669000", 5, false},
		{"A6766000", 10, true},
		{"A6769000", 10, false},
		{"A7066000", 30, true},
		{"A7069000", 30, false},
	}

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.code, func(t *testing.T) {
			in := loadFixture(t, fx.code)
			if in.Tenor != fx.tenor {
				t.Fatalf("fixture %s: tenor=%d, want %d", fx.code, in.Tenor, fx.tenor)
			}
			if in.IsNearMonth != fx.isNear {
				t.Fatalf("fixture %s: is_near_month=%v, want %v", fx.code, in.IsNearMonth, fx.isNear)
			}
			if in.MarketPrice == nil {
				t.Fatalf("fixture %s: market_price is nil, expected populated", fx.code)
			}
			if len(in.OnTheRunKTB) == 0 {
				t.Fatalf("fixture %s: on_the_run_ktb is empty, expected populated", fx.code)
			}
			if len(in.KTBCurve) != 20 {
				t.Fatalf("fixture %s: ktb_curve has %d nodes, expected 20", fx.code, len(in.KTBCurve))
			}

			out, err := process(in)
			if err != nil {
				t.Fatalf("process %s: %v", fx.code, err)
			}

			// fair_value check
			wantNear, ok := expectedNearMonthFairValue[fx.tenor]
			if !ok {
				t.Fatalf("missing expected fair value for tenor %d", fx.tenor)
			}
			if fx.isNear {
				// Tolerance 5e-4 absorbs rounding in the hard-coded reference
				// values (pricing.ktb_futures stored at ~1e-4 precision).
				diff := absFloat(out.FairValue - wantNear)
				if diff > 5e-4 {
					t.Errorf("%s near-month: fair_value=%.8f want=%.8f diff=%.2e",
						fx.code, out.FairValue, wantNear, diff)
				}
				tenorNearFairValue[fx.tenor] = out.FairValue
			} else {
				// Far-month: sanity range only — do not hard-code expected values.
				if math.IsNaN(out.FairValue) || math.IsInf(out.FairValue, 0) {
					t.Errorf("%s far-month: fair_value=%v is not finite", fx.code, out.FairValue)
				}
				diff := absFloat(out.FairValue - wantNear)
				if diff > 1.0 {
					t.Errorf("%s far-month: fair_value=%.8f out of ±1.0 of near %.8f (diff=%.4f)",
						fx.code, out.FairValue, wantNear, diff)
				}
			}

			// KRD: exactly 20 entries ascending by tenor
			if len(out.KRD) != 20 {
				t.Fatalf("%s: KRD has %d entries, want 20", fx.code, len(out.KRD))
			}
			for i := 1; i < len(out.KRD); i++ {
				if !(out.KRD[i].Tenor > out.KRD[i-1].Tenor) {
					t.Errorf("%s: KRD not strictly ascending at index %d (%.4f <= %.4f)",
						fx.code, i, out.KRD[i].Tenor, out.KRD[i-1].Tenor)
				}
			}

			// basis finite when market_price present
			if out.Basis == nil {
				t.Errorf("%s: basis is nil despite market_price being populated", fx.code)
			} else if math.IsNaN(*out.Basis) || math.IsInf(*out.Basis, 0) {
				t.Errorf("%s: basis=%v is not finite", fx.code, *out.Basis)
			} else {
				// basis must equal market_price - fair_value exactly (simple subtraction).
				want := *in.MarketPrice - out.FairValue
				if absFloat(*out.Basis-want) > 1e-9 {
					t.Errorf("%s: basis=%.12f want %.12f", fx.code, *out.Basis, want)
				}
			}

			// onoff_spread finite when on_the_run_ktb present
			if out.OnOffSpread == nil {
				t.Errorf("%s: onoff_spread is nil despite on_the_run_ktb populated", fx.code)
			} else if math.IsNaN(*out.OnOffSpread) || math.IsInf(*out.OnOffSpread, 0) {
				t.Errorf("%s: onoff_spread=%v is not finite", fx.code, *out.OnOffSpread)
			}

			// theta finite
			if math.IsNaN(out.Theta) || math.IsInf(out.Theta, 0) {
				t.Errorf("%s: theta=%v is not finite", fx.code, out.Theta)
			}

			t.Logf("%s tenor=%dY near=%v fair=%.6f basis=%+v onoff=%+v theta=%.3e",
				fx.code, fx.tenor, fx.isNear, out.FairValue, fmtPtr(out.Basis), fmtPtr(out.OnOffSpread), out.Theta)
		})
	}

	// 10Y near vs far must differ: they share the same basket so the only
	// source of difference is the expiry date. Require at least 1e-3 apart.
	t.Run("10Y_near_ne_far", func(t *testing.T) {
		inNear := loadFixture(t, "A6766000")
		inFar := loadFixture(t, "A6769000")
		outNear, err := process(inNear)
		if err != nil {
			t.Fatalf("process near: %v", err)
		}
		outFar, err := process(inFar)
		if err != nil {
			t.Fatalf("process far: %v", err)
		}
		diff := math.Abs(outNear.FairValue - outFar.FairValue)
		if diff <= 1e-3 {
			t.Errorf("10Y near fair=%.8f and far fair=%.8f are too close (diff=%.2e); expiry not being distinguished",
				outNear.FairValue, outFar.FairValue, diff)
		}
		t.Logf("10Y near=%.8f far=%.8f diff=%.6f", outNear.FairValue, outFar.FairValue, diff)
	})
}

func fmtPtr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%.6f", *p)
}

// TestOptionalFieldsNil verifies that omitting market_price produces basis=nil
// (JSON null) and omitting on_the_run_ktb produces onoff_spread=nil.
func TestOptionalFieldsNil(t *testing.T) {
	in := loadFixture(t, "A6566000")
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

	// Verify JSON null serialization for nil pointer fields.
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, needle := range []string{`"basis":null`, `"onoff_spread":null`, `"market_price":null`} {
		if !contains(s, needle) {
			t.Errorf("JSON missing %q; got %s", needle, s)
		}
	}
}

func contains(hay, needle string) bool {
	return len(needle) == 0 || (len(hay) >= len(needle) && indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	n := len(hay) - len(needle)
	for i := 0; i <= n; i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
