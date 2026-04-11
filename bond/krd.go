package bond

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"strings"
	"time"
)

const krdFace = 10000.0

type KRDInput struct {
	ValuationDate string       `json:"valuation_date"`
	BumpBP        float64      `json:"bump_bp"`
	Curve         []CurvePoint `json:"curve"`
	Bonds         []BondInput  `json:"bonds"`
}

type CurvePoint struct {
	Tenor    float64 `json:"tenor"`
	ParYield float64 `json:"par_yield"`
}

type BondInput struct {
	ISIN       string    `json:"isin"`
	DirtyPrice float64   `json:"dirty_price"`
	Cashflows  []CFInput `json:"cashflows"`
}

type CFInput struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

type KRDOutput struct {
	ValuationDate string       `json:"valuation_date"`
	BumpBP        float64      `json:"bump_bp"`
	Results       []BondResult `json:"results"`
}

type BondResult struct {
	ISIN              string         `json:"isin"`
	DirtyPrice        float64        `json:"dirty_price"`
	BasePrice         float64        `json:"base_price"`
	EffectiveDuration float64        `json:"effective_duration"`
	KeyRateDeltas     []KeyRateDelta `json:"key_rate_deltas"`
}

type KeyRateDelta struct {
	Tenor        float64 `json:"tenor"`
	PriceDown    float64 `json:"price_down"`
	PriceUp      float64 `json:"price_up"`
	Delta1Sided  float64 `json:"delta_1sided"`
	DeltaCentral float64 `json:"delta_central"`
	KRD          float64 `json:"krd"`
}

type zeroCurve struct {
	points     []CurvePoint
	grid       []float64
	discounts  []float64
	shiftIndex int
	bumpPct    float64
}

type krdCashflow struct {
	amount float64
	tenor  float64
}

type shiftedCurve struct {
	keyIndex  int
	direction int
	curve     *zeroCurve
	err       error
}

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

	baseCurve, err := bootstrapZeroCurve(points, -1, 0)
	if err != nil {
		return KRDOutput{}, err
	}

	bumpPct := in.BumpBP / 100.0
	shiftedCurves, err := buildShiftedCurves(points, bumpPct)
	if err != nil {
		return KRDOutput{}, err
	}

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

func buildShiftedCurves(points []CurvePoint, bumpPct float64) (map[int]map[int]*zeroCurve, error) {
	results := make(chan shiftedCurve, len(points)*2)
	var wg sync.WaitGroup
	for idx := range points {
		for _, dir := range []int{-1, 1} {
			idx := idx
			dir := dir
			wg.Add(1)
			go func() {
				defer wg.Done()
				curve, err := bootstrapZeroCurve(points, idx, float64(dir)*bumpPct)
				results <- shiftedCurve{keyIndex: idx, direction: dir, curve: curve, err: err}
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

func bootstrapZeroCurve(points []CurvePoint, shiftIndex int, bumpPct float64) (*zeroCurve, error) {
	maxTenor := points[len(points)-1].Tenor
	steps := int(math.Round(maxTenor * 2.0))
	if steps < 1 {
		return nil, fmt.Errorf("ComputeKRD: invalid max tenor %.6f", maxTenor)
	}

	grid := make([]float64, 0, steps)
	discounts := make([]float64, 0, steps)
	sumPrev := 0.0
	for i := 1; i <= steps; i++ {
		tenor := float64(i) / 2.0
		parPct := parYieldAt(points, tenor) + waveShift(points, shiftIndex, tenor, bumpPct)
		if parPct <= -100.0 {
			return nil, fmt.Errorf("ComputeKRD: shifted par yield too low at tenor %.2f", tenor)
		}
		rate := parPct / 100.0
		couponPerPeriod := rate / 2.0
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
	}, nil
}

func parseBondCashflows(valuationDate time.Time, bondIn BondInput) ([]krdCashflow, error) {
	out := make([]krdCashflow, 0, len(bondIn.Cashflows))
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
		out = append(out, krdCashflow{amount: cf.Amount, tenor: tenor})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ComputeKRD: bond %s has no future cashflows", bondIn.ISIN)
	}
	return out, nil
}

func priceCashflows(curve *zeroCurve, cashflows []krdCashflow) float64 {
	price := 0.0
	for _, cf := range cashflows {
		price += cf.amount * curve.discountAt(cf.tenor)
	}
	return price
}

func (z *zeroCurve) discountAt(tenor float64) float64 {
	if tenor <= 0 {
		return 1.0
	}
	if tenor < 0.5 {
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
