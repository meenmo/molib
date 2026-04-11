package bond

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func loadKRDTestInput(t *testing.T) KRDInput {
	t.Helper()

	raw, err := os.ReadFile("../cmd/krwbondkrd/testdata/input.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	var in KRDInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("parse testdata: %v", err)
	}
	return in
}

func TestKRD_LoadAndRun(t *testing.T) {
	input := loadKRDTestInput(t)
	out, err := ComputeKRD(input)
	if err != nil {
		t.Fatalf("ComputeKRD: %v", err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results returned")
	}

	for _, r := range out.Results {
		t.Logf("Bond %s: base_price=%.4f, eff_dur=%.6f, num_krds=%d",
			r.ISIN, r.BasePrice, r.EffectiveDuration, len(r.KeyRateDeltas))
		for _, d := range r.KeyRateDeltas {
			t.Logf("  tenor=%.2f  krd=%.6f  delta_1s=%.4f  delta_c=%.4f  P_down=%.4f  P_up=%.4f",
				d.Tenor, d.KRD, d.Delta1Sided, d.DeltaCentral, d.PriceDown, d.PriceUp)
		}
	}
}

func TestKRD_SumEqualsEffectiveDuration(t *testing.T) {
	input := loadKRDTestInput(t)
	out, err := ComputeKRD(input)
	if err != nil {
		t.Fatalf("ComputeKRD: %v", err)
	}

	for _, r := range out.Results {
		t.Run(r.ISIN, func(t *testing.T) {
			sumKRD := 0.0
			for _, d := range r.KeyRateDeltas {
				sumKRD += d.KRD
			}
			diff := math.Abs(sumKRD - r.EffectiveDuration)
			if diff > 1e-6 {
				t.Errorf("sum(KRDs)=%.8f != eff_dur=%.8f (diff=%.2e)",
					sumKRD, r.EffectiveDuration, diff)
			}
			t.Logf("sum(KRDs)=%.8f, eff_dur=%.8f, diff=%.2e",
				sumKRD, r.EffectiveDuration, diff)
		})
	}
}

func TestKRD_BootstrapRepricesPar(t *testing.T) {
	input := loadKRDTestInput(t)

	// Use the actual curve from testdata
	for _, cp := range input.Curve {
		if cp.Tenor < 0.5 || cp.Tenor == 0.75 {
			continue
		}
		t.Run(fmt.Sprintf("%.1fY", cp.Tenor), func(t *testing.T) {
			coupon := 10000.0 * (cp.ParYield / 100.0) / 2.0
			n := int(cp.Tenor * 2)
			valDate, _ := time.Parse("2006-01-02", input.ValuationDate)
			cfs := make([]CFInput, n)
			for i := 0; i < n; i++ {
				days := int(float64(i+1) * 365.0 / 2.0)
				cfDate := valDate.AddDate(0, 0, days)
				amt := coupon
				if i == n-1 {
					amt += 10000.0
				}
				cfs[i] = CFInput{Date: cfDate.Format("2006-01-02"), Amount: amt}
			}

			parInput := KRDInput{
				ValuationDate: input.ValuationDate,
				BumpBP:        1,
				Curve:         input.Curve,
				Bonds: []BondInput{
					{ISIN: "PAR_TEST", DirtyPrice: 10000.0, Cashflows: cfs},
				},
			}

			out, err := ComputeKRD(parInput)
			if err != nil {
				t.Fatalf("ComputeKRD: %v", err)
			}

			bp := out.Results[0].BasePrice
			diff := math.Abs(bp - 10000.0)
			t.Logf("par bond %.1fY: base_price=%.6f (diff from par: %.6f)", cp.Tenor, bp, diff)
			if diff > 1.0 {
				t.Errorf("base_price=%.6f, expected ~10000 (diff=%.6f)", bp, diff)
			}
		})
	}
}

func TestKRD_ShiftedCurveLocalEffect(t *testing.T) {
	input := loadKRDTestInput(t)
	out, err := ComputeKRD(input)
	if err != nil {
		t.Fatalf("ComputeKRD: %v", err)
	}

	for _, r := range out.Results {
		t.Run(r.ISIN, func(t *testing.T) {
			var bondIn BondInput
			for _, b := range input.Bonds {
				if b.ISIN == r.ISIN {
					bondIn = b
					break
				}
			}
			valDate, _ := time.Parse("2006-01-02", input.ValuationDate)
			lastCF, _ := time.Parse("2006-01-02", bondIn.Cashflows[len(bondIn.Cashflows)-1].Date)
			maturityTenor := lastCF.Sub(valDate).Hours() / (24 * 365)

			for _, d := range r.KeyRateDeltas {
				if d.Tenor > maturityTenor+3.0 {
					if math.Abs(d.KRD) > 0.001 {
						t.Errorf("tenor=%.1f (beyond maturity %.1f): KRD=%.6f should be ~0",
							d.Tenor, maturityTenor, d.KRD)
					}
				}
			}
			t.Logf("maturity_tenor=%.2f", maturityTenor)
		})
	}
}

var _ = fmt.Sprintf
