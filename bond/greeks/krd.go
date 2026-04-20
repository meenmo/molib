// Package greeks provides bond risk measures including Key Rate Duration (KRD)
// using the Bloomberg Wave methodology.
package greeks

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ComputeKRD calculates Key Rate Durations for a set of bonds.
func ComputeKRD(in KRDInput) (KRDOutput, error) {
	if strings.TrimSpace(in.ValuationDate) == "" {
		return KRDOutput{}, fmt.Errorf("ComputeKRD: valuation_date is required")
	}
	valuationDate, err := time.Parse("2006-01-02", in.ValuationDate)
	if err != nil {
		return KRDOutput{}, fmt.Errorf("ComputeKRD: parse valuation_date: %w", err)
	}
	if in.BumpBP <= 0 {
		return KRDOutput{}, fmt.Errorf("ComputeKRD: bump_bp must be positive")
	}
	points, err := normalizeCurvePoints(in.Curve)
	if err != nil {
		return KRDOutput{}, err
	}
	if len(in.Bonds) == 0 {
		return KRDOutput{}, fmt.Errorf("ComputeKRD: bonds are required")
	}

	// Default to semi-annual if not specified.
	freq := in.CouponFrequency
	if freq <= 0 {
		freq = 2
	}

	// Build base zero curve (no shift).
	dayCount := in.DayCount
	if dayCount == "" {
		dayCount = "ACT/ACT"
	}

	baseCurve, err := bootstrapZeroCurve(points, -1, 0, freq, dayCount)
	if err != nil {
		return KRDOutput{}, err
	}

	// Phase 1: Build 2N shifted curves in parallel.
	bumpPct := in.BumpBP / 100.0
	shiftedCurves, err := buildShiftedCurves(points, bumpPct, freq, dayCount)
	if err != nil {
		return KRDOutput{}, err
	}

	// Phase 2: Reprice all bonds in parallel.
	results := make([]BondResult, len(in.Bonds))
	jobs := make(chan int)
	errCh := make(chan error, len(in.Bonds))
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				result, err := computeBondKRD(valuationDate, in.Bonds[idx], baseCurve, shiftedCurves, in.BumpBP)
				if err != nil {
					errCh <- err
					continue
				}
				results[idx] = result
			}
		}()
	}

	for i := range in.Bonds {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(errCh)
	if err, ok := <-errCh; ok {
		return KRDOutput{}, err
	}

	return KRDOutput{
		ValuationDate: in.ValuationDate,
		BumpBP:        in.BumpBP,
		Results:       results,
	}, nil
}

// computeBondKRD computes KRD for a single bond against base and shifted curves.
func computeBondKRD(
	valuationDate time.Time,
	bondIn BondInput,
	baseCurve *zeroCurve,
	shifted map[int]map[int]*zeroCurve,
	bumpBP float64,
) (BondResult, error) {
	if strings.TrimSpace(bondIn.ISIN) == "" {
		return BondResult{}, fmt.Errorf("ComputeKRD: bond isin is required")
	}
	if bondIn.DirtyPrice <= 0 {
		return BondResult{}, fmt.Errorf("ComputeKRD: bond %s dirty_price must be positive", bondIn.ISIN)
	}
	if len(bondIn.Cashflows) == 0 {
		return BondResult{}, fmt.Errorf("ComputeKRD: bond %s cashflows are required", bondIn.ISIN)
	}

	parsed, err := parseBondCashflows(valuationDate, bondIn)
	if err != nil {
		return BondResult{}, err
	}
	basePrice := priceCashflows(baseCurve, parsed)

	keyResults := make([]KeyRateDelta, 0, len(baseCurve.points))
	denominator := 2.0 * (bumpBP / 100.0) * bondIn.DirtyPrice
	if denominator == 0 {
		return BondResult{}, fmt.Errorf("ComputeKRD: bond %s invalid denominator", bondIn.ISIN)
	}
	effectiveDuration := 0.0

	for i, pt := range baseCurve.points {
		curveDown := shifted[i][-1]
		curveUp := shifted[i][1]
		priceDown := priceCashflows(curveDown, parsed)
		priceUp := priceCashflows(curveUp, parsed)
		deltaCentral := priceDown - priceUp
		krd := 100.0 * deltaCentral / denominator
		effectiveDuration += krd
		keyResults = append(keyResults, KeyRateDelta{
			Tenor:        pt.Tenor,
			PriceDown:    priceDown,
			PriceUp:      priceUp,
			Delta1Sided:  priceDown - basePrice,
			DeltaCentral: deltaCentral,
			KRD:          krd,
		})
	}

	return BondResult{
		ISIN:              bondIn.ISIN,
		DirtyPrice:        bondIn.DirtyPrice,
		BasePrice:         basePrice,
		EffectiveDuration: effectiveDuration,
		KeyRateDeltas:     keyResults,
	}, nil
}

// parseBondCashflows converts input cashflows to internal format with pre-computed tenors.
func parseBondCashflows(valuationDate time.Time, bondIn BondInput) ([]cashflow, error) {
	out := make([]cashflow, 0, len(bondIn.Cashflows))
	for _, cf := range bondIn.Cashflows {
		if cf.Amount == 0 {
			continue
		}
		cfDate, err := time.Parse("2006-01-02", cf.Date)
		if err != nil {
			return nil, fmt.Errorf("ComputeKRD: bond %s parse cashflow date %q: %w", bondIn.ISIN, cf.Date, err)
		}
		tenor := cfDate.Sub(valuationDate).Hours() / 24.0 / 365.0
		if tenor <= 0 {
			continue
		}
		out = append(out, cashflow{amount: cf.Amount, tenor: tenor})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ComputeKRD: bond %s has no future cashflows", bondIn.ISIN)
	}
	return out, nil
}
