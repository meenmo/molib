package greeks

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// normalizeCurvePoints sorts and validates curve points.
func normalizeCurvePoints(points []CurvePoint) ([]CurvePoint, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("ComputeKRD: curve is required")
	}
	normalized := append([]CurvePoint(nil), points...)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Tenor < normalized[j].Tenor })
	for i, pt := range normalized {
		if pt.Tenor <= 0 {
			return nil, fmt.Errorf("ComputeKRD: curve tenor must be positive")
		}
		if i > 0 && math.Abs(pt.Tenor-normalized[i-1].Tenor) < 1e-12 {
			return nil, fmt.Errorf("ComputeKRD: duplicate curve tenor %.6f", pt.Tenor)
		}
	}
	return normalized, nil
}

// buildShiftedCurves builds 2*N shifted curves in parallel.
func buildShiftedCurves(points []CurvePoint, bumpPct float64, freq int) (map[int]map[int]*zeroCurve, error) {
	results := make(chan shiftedCurveResult, len(points)*2)
	var wg sync.WaitGroup
	for idx := range points {
		for _, dir := range []int{-1, 1} {
			idx := idx
			dir := dir
			wg.Add(1)
			go func() {
				defer wg.Done()
				curve, err := bootstrapZeroCurve(points, idx, float64(dir)*bumpPct, freq)
				results <- shiftedCurveResult{keyIndex: idx, direction: dir, curve: curve, err: err}
			}()
		}
	}
	wg.Wait()
	close(results)

	shifted := make(map[int]map[int]*zeroCurve, len(points))
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		if shifted[result.keyIndex] == nil {
			shifted[result.keyIndex] = make(map[int]*zeroCurve, 2)
		}
		shifted[result.keyIndex][result.direction] = result.curve
	}
	return shifted, nil
}

// bootstrapZeroCurve converts par yields to discount factors on a 1/freq step grid.
func bootstrapZeroCurve(points []CurvePoint, shiftIndex int, bumpPct float64, freq int) (*zeroCurve, error) {
	step := 1.0 / float64(freq)
	maxTenor := points[len(points)-1].Tenor
	nSteps := int(math.Round(maxTenor / step))
	if nSteps < 1 {
		return nil, fmt.Errorf("ComputeKRD: invalid max tenor %.6f", maxTenor)
	}

	grid := make([]float64, 0, nSteps)
	discounts := make([]float64, 0, nSteps)
	sumPrev := 0.0
	for i := 1; i <= nSteps; i++ {
		tenor := float64(i) * step
		parPct := parYieldAt(points, tenor) + waveShift(points, shiftIndex, tenor, bumpPct)
		if parPct <= -100.0 {
			return nil, fmt.Errorf("ComputeKRD: shifted par yield too low at tenor %.2f", tenor)
		}
		rate := parPct / 100.0
		couponPerPeriod := rate / float64(freq)
		df := (1.0 - couponPerPeriod*sumPrev) / (1.0 + couponPerPeriod)
		if df <= 0 || math.IsNaN(df) || math.IsInf(df, 0) {
			return nil, fmt.Errorf("ComputeKRD: invalid discount factor at tenor %.2f", tenor)
		}
		grid = append(grid, tenor)
		discounts = append(discounts, df)
		sumPrev += df
	}

	return &zeroCurve{
		points:     append([]CurvePoint(nil), points...),
		grid:       grid,
		discounts:  discounts,
		shiftIndex: shiftIndex,
		bumpPct:    bumpPct,
		freq:       freq,
		step:       step,
	}, nil
}

// discountAt returns the interpolated discount factor at a given tenor.
func (z *zeroCurve) discountAt(tenor float64) float64 {
	if tenor <= 0 {
		return 1.0
	}
	if tenor < z.step {
		parPct := parYieldAt(z.points, tenor) + waveShift(z.points, z.shiftIndex, tenor, z.bumpPct)
		return math.Exp(-(parPct / 100.0) * tenor)
	}
	if tenor <= z.grid[0] {
		return z.discounts[0]
	}
	lastIdx := len(z.grid) - 1
	if tenor >= z.grid[lastIdx] {
		lastTenor := z.grid[lastIdx]
		lastDF := z.discounts[lastIdx]
		zeroRate := -math.Log(lastDF) / lastTenor
		return math.Exp(-zeroRate * tenor)
	}
	idx := sort.Search(len(z.grid), func(i int) bool { return z.grid[i] >= tenor })
	if idx < len(z.grid) && math.Abs(z.grid[idx]-tenor) < 1e-12 {
		return z.discounts[idx]
	}
	leftTenor := z.grid[idx-1]
	rightTenor := z.grid[idx]
	leftLog := math.Log(z.discounts[idx-1])
	rightLog := math.Log(z.discounts[idx])
	weight := (tenor - leftTenor) / (rightTenor - leftTenor)
	return math.Exp(leftLog + weight*(rightLog-leftLog))
}

// parYieldAt linearly interpolates par yield at a given tenor.
func parYieldAt(points []CurvePoint, tenor float64) float64 {
	if tenor <= points[0].Tenor {
		return points[0].ParYield
	}
	last := len(points) - 1
	if tenor >= points[last].Tenor {
		return points[last].ParYield
	}
	idx := sort.Search(len(points), func(i int) bool { return points[i].Tenor >= tenor })
	if idx < len(points) && math.Abs(points[idx].Tenor-tenor) < 1e-12 {
		return points[idx].ParYield
	}
	left := points[idx-1]
	right := points[idx]
	weight := (tenor - left.Tenor) / (right.Tenor - left.Tenor)
	return left.ParYield + weight*(right.ParYield-left.ParYield)
}

// waveShift computes the Bloomberg triangle wave shift at a given tenor.
func waveShift(points []CurvePoint, shiftIndex int, tenor, bumpPct float64) float64 {
	if shiftIndex < 0 || shiftIndex >= len(points) || bumpPct == 0 {
		return 0
	}
	current := points[shiftIndex].Tenor
	if shiftIndex == 0 {
		next := points[1].Tenor
		switch {
		case tenor <= current:
			if current == 0 {
				return 0
			}
			return bumpPct * tenor / current
		case tenor <= next:
			return bumpPct * (next - tenor) / (next - current)
		default:
			return 0
		}
	}
	if shiftIndex == len(points)-1 {
		prev := points[shiftIndex-1].Tenor
		if tenor < prev {
			return 0
		}
		if tenor < current {
			return bumpPct * (tenor - prev) / (current - prev)
		}
		return bumpPct
	}
	prev := points[shiftIndex-1].Tenor
	next := points[shiftIndex+1].Tenor
	if tenor <= prev || tenor >= next {
		return 0
	}
	if tenor <= current {
		return bumpPct * (tenor - prev) / (current - prev)
	}
	return bumpPct * (next - tenor) / (next - current)
}

// priceCashflows computes the present value of cashflows on a given curve.
func priceCashflows(curve *zeroCurve, cashflows []cashflow) float64 {
	price := 0.0
	for _, cf := range cashflows {
		price += cf.amount * curve.discountAt(cf.tenor)
	}
	return price
}
