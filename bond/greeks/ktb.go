package greeks

import (
	"fmt"
	"sort"
	"time"

	"github.com/meenmo/molib/bond/ktb"
	"github.com/meenmo/molib/calendar"
)

// OnTheRunBond describes an on-the-run KTB used to build the indicator curve
// for the on-off spread calculation.
type OnTheRunBond struct {
	ISIN         string
	MaturityDate time.Time
	Yield        float64 // percent
}

// KTBGreeksInput bundles all inputs required to compute KTB futures greeks for
// a single contract (one basket, one valuation date).
type KTBGreeksInput struct {
	Date             time.Time
	NextBusinessDate time.Time
	CD91             float64 // percent
	FuturesCode      string
	IsNearMonth      bool
	Tenor            int
	MarketPrice      *float64 // nil => basis is nil
	Bonds            []ktb.KTBBond
	KTBCurve         []CurvePoint   // 20-node par curve
	OnTheRunKTB      []OnTheRunBond // nil/empty => onoff_spread is nil
}

// KTBKeyRateDelta is one bucket of the 20-node KTB futures KRD output.
type KTBKeyRateDelta struct {
	Tenor float64 `json:"tenor"`
	Delta float64 `json:"delta"`
}

// KTBGreeksResult is the output of ComputeKTBGreeks for one contract.
type KTBGreeksResult struct {
	FuturesCode   string
	IsNearMonth   bool
	Tenor         int
	FuturesExpiry time.Time
	FairValue     float64
	MarketPrice   *float64
	Theta         float64
	Basis         *float64 // nil when MarketPrice == nil
	OnOffSpread   *float64 // nil when OnTheRunKTB is empty
	KRD           []KTBKeyRateDelta
}

// ComputeKTBGreeks computes fair value, theta, 20-node KRD, basis, and on-off
// spread for one KTB futures contract. All repricing flows through
// bond.ComputeKTBFairValues to preserve exact VBA parity.
func ComputeKTBGreeks(in KTBGreeksInput) (KTBGreeksResult, error) {
	if len(in.Bonds) == 0 {
		return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: bonds are required")
	}
	if len(in.KTBCurve) == 0 {
		return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: ktb_curve is required")
	}

	curve := append([]CurvePoint(nil), in.KTBCurve...)
	sort.Slice(curve, func(i, j int) bool { return curve[i].Tenor < curve[j].Tenor })

	// Determine futures expiry based on near/far month flag.
	var expiry time.Time
	if in.IsNearMonth {
		expiry = calendar.KTBFuturesExpiry(in.Date)
	} else {
		expiry = calendar.KTBFuturesExpiryFarMonth(in.Date)
	}

	// --- Fair value (base) ---
	basePrice, err := priceKTB(in.Date, expiry, in.CD91, in.Tenor, in.Bonds)
	if err != nil {
		return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: base price: %w", err)
	}

	// --- Theta ---
	// Same basket/yields/CD91, advance valuation date by one business day.
	// The futures contract identity (expiry) stays fixed for the theta step.
	pNext, err := priceKTB(in.NextBusinessDate, expiry, in.CD91, in.Tenor, in.Bonds)
	if err != nil {
		return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: theta step: %w", err)
	}
	theta := (pNext - basePrice) / 100.0

	// --- KRD ---
	// 1 bp triangular bump on each of the 20 par-curve nodes. Bloomberg-Wave
	// triangle centered at tau_i, linearly down to 0 at tau_{i-1} and
	// tau_{i+1}; half-triangle at the two boundary nodes. CD91 is only
	// bumped inside the 0.25Y bucket.
	const bumpBP = 1.0
	bumpPct := bumpBP / 100.0
	krdOut := make([]KTBKeyRateDelta, len(curve))
	for i := range curve {
		pDown, pUp, err := ktbKRDBucket(in, expiry, curve, i, bumpPct)
		if err != nil {
			return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: krd bucket %.2f: %w", curve[i].Tenor, err)
		}
		// Central difference: (P_down - P_up) / (2 * bump_bp * 100).
		delta := (pDown - pUp) / (2.0 * bumpBP * 100.0)
		krdOut[i] = KTBKeyRateDelta{Tenor: curve[i].Tenor, Delta: delta}
	}

	// --- Basis ---
	var basis *float64
	if in.MarketPrice != nil {
		b := *in.MarketPrice - basePrice
		basis = &b
	}

	// --- On-off spread ---
	var onoff *float64
	if len(in.OnTheRunKTB) > 0 {
		indicator := buildIndicatorCurve(in.Date, in.OnTheRunKTB)
		indBonds := make([]ktb.KTBBond, len(in.Bonds))
		for j, b := range in.Bonds {
			residual := yearsBetween(in.Date, b.MaturityDate)
			indBonds[j] = b
			indBonds[j].MarketYield = interpFlat(indicator, residual)
		}
		pInd, err := priceKTB(in.Date, expiry, in.CD91, in.Tenor, indBonds)
		if err != nil {
			return KTBGreeksResult{}, fmt.Errorf("ComputeKTBGreeks: onoff price: %w", err)
		}
		s := basePrice - pInd
		onoff = &s
	}

	return KTBGreeksResult{
		FuturesCode:   in.FuturesCode,
		IsNearMonth:   in.IsNearMonth,
		Tenor:         in.Tenor,
		FuturesExpiry: expiry,
		FairValue:     basePrice,
		MarketPrice:   in.MarketPrice,
		Theta:         theta,
		Basis:         basis,
		OnOffSpread:   onoff,
		KRD:           krdOut,
	}, nil
}

// priceKTB is a thin wrapper that calls ComputeKTBFairValues with a single
// basket and an explicit expiry date, returning the scalar fair value.
func priceKTB(valDate time.Time, expiry time.Time, cd91 float64, tenor int, bonds []ktb.KTBBond) (float64, error) {
	out, err := ktb.ComputeKTBFuturesFairValues(ktb.KTBFuturesFairValueInput{
		Date:       valDate,
		ExpiryDate: expiry,
		CD91:       cd91,
		Baskets:    []ktb.KTBFuturesBasket{{Tenor: tenor, Bonds: bonds}},
	})
	if err != nil {
		return 0, err
	}
	if len(out) != 1 {
		return 0, fmt.Errorf("unexpected result count %d", len(out))
	}
	return out[0].FairValue, nil
}

// ktbKRDBucket computes P_down and P_up for one triangular bucket shift.
// CD91 is bumped only when the bucket tenor is 0.25 (the short-rate bucket).
func ktbKRDBucket(in KTBGreeksInput, expiry time.Time, curve []CurvePoint, i int, bumpPct float64) (float64, float64, error) {
	// Build per-bond yield shift = triangle(T_j) at +bumpPct.
	shiftPos := make([]float64, len(in.Bonds))
	for j, b := range in.Bonds {
		tj := yearsBetween(in.Date, b.MaturityDate)
		shiftPos[j] = ktbTriangle(curve, i, tj, bumpPct)
	}

	// CD91 shift: sample the same triangle at tenor 0.25Y. Because the
	// triangle is zero outside [tau_{i-1}, tau_{i+1}], this is non-zero
	// only for the 0.25Y bucket (boundary node).
	cdShift := ktbTriangle(curve, i, 0.25, bumpPct)

	bondsUp := make([]ktb.KTBBond, len(in.Bonds))
	bondsDown := make([]ktb.KTBBond, len(in.Bonds))
	for j, b := range in.Bonds {
		bondsUp[j] = b
		bondsUp[j].MarketYield = b.MarketYield + shiftPos[j]
		bondsDown[j] = b
		bondsDown[j].MarketYield = b.MarketYield - shiftPos[j]
	}

	cd91Up := in.CD91 + cdShift
	cd91Down := in.CD91 - cdShift

	pUp, err := priceKTB(in.Date, expiry, cd91Up, in.Tenor, bondsUp)
	if err != nil {
		return 0, 0, err
	}
	pDown, err := priceKTB(in.Date, expiry, cd91Down, in.Tenor, bondsDown)
	if err != nil {
		return 0, 0, err
	}
	return pDown, pUp, nil
}

// ktbTriangle evaluates the Bloomberg-Wave triangle centered on curve[i] at
// the given tenor. Value = bumpPct at curve[i].Tenor, linear ramp to 0 at
// curve[i-1].Tenor and curve[i+1].Tenor. Half-triangle at the boundaries.
// Standalone helper (the existing greeks/curve.go waveShift is specialized to
// bond pricing via a zero curve; this one is the minimal triangle evaluator).
func ktbTriangle(curve []CurvePoint, i int, tenor, bumpPct float64) float64 {
	if i < 0 || i >= len(curve) || bumpPct == 0 {
		return 0
	}
	current := curve[i].Tenor

	// Left boundary node: half-triangle (left side flat up to current).
	if i == 0 {
		next := curve[1].Tenor
		switch {
		case tenor <= current:
			return bumpPct
		case tenor < next:
			return bumpPct * (next - tenor) / (next - current)
		default:
			return 0
		}
	}

	// Right boundary node: half-triangle (right side flat beyond current).
	if i == len(curve)-1 {
		prev := curve[i-1].Tenor
		switch {
		case tenor <= prev:
			return 0
		case tenor < current:
			return bumpPct * (tenor - prev) / (current - prev)
		default:
			return bumpPct
		}
	}

	prev := curve[i-1].Tenor
	next := curve[i+1].Tenor
	switch {
	case tenor <= prev || tenor >= next:
		return 0
	case tenor <= current:
		return bumpPct * (tenor - prev) / (current - prev)
	default:
		return bumpPct * (next - tenor) / (next - current)
	}
}

// indicatorPoint is one (residual_years, yield) node on the on-the-run curve.
type indicatorPoint struct {
	Tenor float64
	Yield float64
}

// buildIndicatorCurve builds a sorted (residual_years, yield) curve from the
// on-the-run bond set, using (maturity - date) / 365 for residual maturity.
func buildIndicatorCurve(valDate time.Time, bonds []OnTheRunBond) []indicatorPoint {
	out := make([]indicatorPoint, len(bonds))
	for i, b := range bonds {
		out[i] = indicatorPoint{
			Tenor: yearsBetween(valDate, b.MaturityDate),
			Yield: b.Yield,
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tenor < out[j].Tenor })
	return out
}

// interpFlat linearly interpolates yields on the indicator curve, holding
// flat at both ends (matches VBA LinearInterpolation).
func interpFlat(pts []indicatorPoint, tenor float64) float64 {
	if len(pts) == 0 {
		return 0
	}
	if tenor <= pts[0].Tenor {
		return pts[0].Yield
	}
	last := len(pts) - 1
	if tenor >= pts[last].Tenor {
		return pts[last].Yield
	}
	for i := 0; i < last; i++ {
		a, b := pts[i], pts[i+1]
		if tenor >= a.Tenor && tenor <= b.Tenor {
			if b.Tenor == a.Tenor {
				return a.Yield
			}
			w := (tenor - a.Tenor) / (b.Tenor - a.Tenor)
			return a.Yield + w*(b.Yield-a.Yield)
		}
	}
	return pts[last].Yield
}

// yearsBetween returns (to - from) in years using a 365-day denominator,
// matching the spec's residual-maturity convention.
func yearsBetween(from, to time.Time) float64 {
	return to.Sub(from).Hours() / 24.0 / 365.0
}
