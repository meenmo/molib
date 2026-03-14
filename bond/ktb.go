package bond

import (
	"fmt"
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
)

const ktbFace = 10000.0

// KTBBond describes a deliverable bond in a KTB futures basket.
type KTBBond struct {
	ISIN         string
	IssueDate    time.Time
	MaturityDate time.Time
	CouponRate   float64 // annual coupon in percent (e.g., 2.875)
	MarketYield  float64 // 민평3사 yield in percent (e.g., 3.03)
}

// KTBFairValueInput holds the parameters for computing KTB futures fair values.
type KTBFairValueInput struct {
	Date    time.Time
	CD91    float64 // CD 91-day rate in percent (e.g., 2.83)
	Baskets []KTBBasket
}

// KTBBasket is a deliverable basket for one futures tenor.
type KTBBasket struct {
	Tenor int // 3, 5, 10, or 30
	Bonds []KTBBond
}

// KTBFairValueResult is the output per tenor.
type KTBFairValueResult struct {
	Tenor         int
	FairValue     float64
	FuturesExpiry time.Time
}

// ComputeKTBFairValues computes theoretical fair values for all baskets.
func ComputeKTBFairValues(in KTBFairValueInput) ([]KTBFairValueResult, error) {
	cd91 := in.CD91 / 100.0
	expiry := calendar.KTBFuturesExpiry(in.Date)

	results := make([]KTBFairValueResult, 0, len(in.Baskets))
	for _, basket := range in.Baskets {
		fwdYields := make([]float64, 0, len(basket.Bonds))
		for _, bond := range basket.Bonds {
			fy, err := ktbForwardYield(in.Date, expiry, cd91, bond)
			if err != nil {
				return nil, fmt.Errorf("tenor %dY bond %s: %w", basket.Tenor, bond.ISIN, err)
			}
			fwdYields = append(fwdYields, fy)
		}

		if len(fwdYields) == 0 {
			return nil, fmt.Errorf("tenor %dY: no bonds", basket.Tenor)
		}

		sum := 0.0
		for _, y := range fwdYields {
			sum += y
		}
		avgYield := sum / float64(len(fwdYields))

		results = append(results, KTBFairValueResult{
			Tenor:         basket.Tenor,
			FairValue:     ktbFairValue(basket.Tenor, avgYield),
			FuturesExpiry: expiry,
		})
	}
	return results, nil
}

// ktbForwardYield implements Steps 1-5 of the KTB futures fair value algorithm.
func ktbForwardYield(today, expiry time.Time, cd91 float64, b KTBBond) (float64, error) {
	flows := KTBCashflows(b.IssueDate, b.MaturityDate, b.CouponRate)
	y := b.MarketYield / 100.0

	// Step 1: Spot dirty price
	prev, next := KTBAdjacentPaymentDates(today, flows, b.IssueDate)
	remainingToday := 0
	for _, cf := range flows {
		if cf.Date.After(today) { // strict: dt > today
			remainingToday++
		}
	}
	spotDirty := KTBMarketPrice(y, b.CouponRate, prev, next, today, remainingToday)

	// Step 2: Coupon before futures expiry — sum coupons in (today, expiry]
	couponBeforeExpiry := 0.0
	couponAmt := ktbFace * (b.CouponRate / 2.0) / 100.0
	for _, cf := range flows {
		if cf.Date.After(today) && !cf.Date.After(expiry) {
			couponBeforeExpiry += couponAmt
		}
	}

	// Step 3: Clean price
	prevExp, nextExp := KTBAdjacentPaymentDates(expiry, flows, b.IssueDate)
	cleanPrice := spotDirty - couponBeforeExpiry
	if couponBeforeExpiry > 0 {
		daysUntilPymt := float64(prevExp.Sub(today).Hours() / 24)
		couponPV := couponBeforeExpiry / (1.0 + cd91*daysUntilPymt/365.0)
		cleanPrice = spotDirty - couponPV
	}

	// Step 4: Forward dirty price
	daysToExpiry := float64(expiry.Sub(today).Hours() / 24)
	fwdDirty := cleanPrice * (1.0 + cd91*daysToExpiry/365.0)

	// Step 5: Solve implied yield at expiry
	numAtExpiry := 0
	for _, cf := range flows {
		if !cf.Date.Before(expiry) { // inclusive: dt >= expiry
			numAtExpiry++
		}
	}

	impliedYield, _, err := KTBSolveImpliedYield(fwdDirty, b.CouponRate, prevExp, nextExp, expiry, numAtExpiry)
	if err == nil {
		return impliedYield, nil
	}

	fallback, fbErr := ktbSolveImpliedYieldBisection(fwdDirty, b.CouponRate, prevExp, nextExp, expiry, numAtExpiry)
	if fbErr != nil {
		return 0, fmt.Errorf("solveImpliedYield for %s: %w", b.ISIN, err)
	}
	return fallback, nil
}

// ktbFairValue computes the standard KTB futures theoretical price (5% coupon).
func ktbFairValue(tenor int, avgYield float64) float64 {
	n := 2 * tenor
	half := 1.0 + avgYield/2.0
	pv := 0.0
	for i := 1; i <= n; i++ {
		pv += 2.5 / math.Pow(half, float64(i))
	}
	pv += 100.0 / math.Pow(half, float64(n))
	return pv
}

// KTBCashflows generates semiannual cashflows for a KTB bond (face=10000).
func KTBCashflows(issue, maturity time.Time, couponRatePct float64) []Cashflow {
	couponAmt := ktbFace * couponRatePct / 200.0

	var dates []time.Time
	cur := issue.AddDate(0, 6, 0)
	for !cur.After(maturity) {
		dates = append(dates, cur)
		cur = cur.AddDate(0, 6, 0)
	}
	if len(dates) == 0 {
		dates = []time.Time{maturity}
	}

	flows := make([]Cashflow, len(dates))
	for i, d := range dates {
		if i < len(dates)-1 {
			flows[i] = Cashflow{Date: d, Coupon: couponAmt, Principal: 0}
		} else {
			flows[i] = Cashflow{Date: d, Coupon: couponAmt, Principal: ktbFace}
		}
	}
	return flows
}

// KTBAdjacentPaymentDates finds the previous and next coupon dates around asOf.
func KTBAdjacentPaymentDates(asOf time.Time, flows []Cashflow, issue time.Time) (time.Time, time.Time) {
	if asOf.Before(flows[0].Date) {
		return issue, flows[0].Date
	}
	prev := flows[0].Date
	for _, cf := range flows {
		if !cf.Date.After(asOf) { // dt <= asOf
			prev = cf.Date
		} else {
			return prev, cf.Date
		}
	}
	return flows[len(flows)-1].Date, flows[len(flows)-1].Date
}

// KTBMarketPrice computes the KTB bond dirty price given yield.
// Face = 10000, semiannual coupons.
func KTBMarketPrice(y, couponRatePct float64, prev, next, pricingDate time.Time, numPayments int) float64 {
	couponAmt := ktbFace * (couponRatePct / 2.0) / 100.0

	price := 0.0
	for k := 0; k < numPayments; k++ {
		price += couponAmt / math.Pow(1.0+y/2.0, float64(k))
	}
	lastIdx := numPayments - 1
	if lastIdx < 0 {
		lastIdx = 0
	}
	price += ktbFace / math.Pow(1.0+y/2.0, float64(lastIdx))

	d := float64(next.Sub(pricingDate).Hours() / 24)
	t := float64(next.Sub(prev).Hours() / 24)
	if t < 1 {
		t = 1
	}
	return price / (1.0 + (d/t)*(y/2.0))
}

// KTBMarketPriceAndDeriv returns (price, dPrice/dy) using the analytic quotient-rule derivative.
func KTBMarketPriceAndDeriv(y, couponRatePct float64, prev, next, pricingDate time.Time, numPayments int) (float64, float64) {
	couponAmt := ktbFace * (couponRatePct / 2.0) / 100.0

	d := float64(next.Sub(pricingDate).Hours() / 24)
	t := float64(next.Sub(prev).Hours() / 24)
	if t < 1 {
		t = 1
	}

	half := 1.0 + y/2.0

	A, dAdy := 0.0, 0.0
	for k := 0; k < numPayments; k++ {
		pk := math.Pow(half, float64(k))
		A += couponAmt / pk
		dAdy += -float64(k) * (couponAmt / 2.0) / math.Pow(half, float64(k)+1)
	}
	lastIdx := numPayments - 1
	if lastIdx < 0 {
		lastIdx = 0
	}
	pLast := math.Pow(half, float64(lastIdx))
	A += ktbFace / pLast
	dAdy += -float64(lastIdx) * (ktbFace / 2.0) / math.Pow(half, float64(lastIdx)+1)

	B := 1.0 + (d/t)*(y/2.0)
	dBdy := d / (2.0 * t)

	P := A / B
	dP := (dAdy*B - A*dBdy) / (B * B)
	return P, dP
}

// KTBSolveImpliedYield finds the yield such that KTBMarketPrice(y) == targetPrice.
// Uses Newton-Raphson with bracket and fallback seed strategies.
func KTBSolveImpliedYield(targetPrice, couponRatePct float64, prev, next, pricingDate time.Time, numPayments int) (float64, int, error) {
	const (
		tol     = 1e-12
		maxIter = 200
		yFloor  = -0.05
		yCeil   = 0.50
	)

	clamp := func(v float64) float64 {
		if v < yFloor {
			return yFloor
		}
		if v > yCeil {
			return yCeil
		}
		return v
	}

	solve := func(seed float64) (float64, int, bool) {
		y := clamp(seed)
		for i := 0; i < maxIter; i++ {
			p, dp := KTBMarketPriceAndDeriv(y, couponRatePct, prev, next, pricingDate, numPayments)
			f := p - targetPrice
			if math.Abs(f) < tol {
				return y, i + 1, true
			}
			if math.Abs(dp) < 1e-15 {
				return y, i + 1, false
			}
			y = clamp(y - f/dp)
		}
		return y, maxIter, false
	}

	// Strategy 1: single seed
	if y, iters, ok := solve(0.028); ok {
		return y, iters, nil
	}

	// Strategy 2: bracket
	lo, hi := 0.0, 0.15
	pLo := KTBMarketPrice(lo, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
	pHi := KTBMarketPrice(hi, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
	if pLo*pHi < 0 {
		for i := 0; i < 50; i++ {
			mid := (lo + hi) / 2.0
			pMid := KTBMarketPrice(mid, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
			if math.Abs(pMid) < tol {
				return mid, i, nil
			}
			if pLo*pMid < 0 {
				hi = mid
			} else {
				lo = mid
				pLo = pMid
			}
		}
		seed := (lo + hi) / 2.0
		if y, iters, ok := solve(seed); ok {
			return y, iters, nil
		}
	}

	// Strategy 3: fallback seeds
	for _, seed := range []float64{0.005, 0.01, 0.02, 0.03, 0.05, 0.08, 0.12} {
		if y, iters, ok := solve(seed); ok {
			return y, iters, nil
		}
	}

	return 0, maxIter, fmt.Errorf("KTBSolveImpliedYield: did not converge for target=%.6f", targetPrice)
}

// ktbSolveImpliedYieldBisection is a pure bisection fallback solver.
func ktbSolveImpliedYieldBisection(targetPrice, couponRatePct float64, prev, next, pricingDate time.Time, numPayments int) (float64, error) {
	const (
		yLo      = -0.05
		yHi      = 0.50
		tolPrice = 1e-9
		tolY     = 1e-13
		maxIter  = 300
	)

	fLo := KTBMarketPrice(yLo, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
	fHi := KTBMarketPrice(yHi, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
	if fLo == 0 {
		return yLo, nil
	}
	if fHi == 0 {
		return yHi, nil
	}
	if fLo*fHi > 0 {
		return 0, fmt.Errorf("bisection bracket failure for target=%.6f", targetPrice)
	}

	lo, hi := yLo, yHi
	for i := 0; i < maxIter; i++ {
		mid := (lo + hi) / 2.0
		fMid := KTBMarketPrice(mid, couponRatePct, prev, next, pricingDate, numPayments) - targetPrice
		if math.Abs(fMid) <= tolPrice || (hi-lo) <= tolY {
			return mid, nil
		}
		if fLo*fMid <= 0 {
			hi = mid
		} else {
			lo = mid
			fLo = fMid
		}
	}
	return (lo + hi) / 2.0, nil
}
