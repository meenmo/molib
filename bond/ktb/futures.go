package ktb

import (
	"fmt"
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
)

// KTBFuturesBasket is a deliverable basket for one futures tenor.
type KTBFuturesBasket struct {
	Tenor int // 3, 5, 10, or 30
	Bonds []KTBBond
}

// KTBFuturesFairValueInput holds the parameters for computing KTB futures fair values.
type KTBFuturesFairValueInput struct {
	Date       time.Time
	ExpiryDate time.Time // optional; if zero, derived as near-month via calendar.KTBFuturesExpiry
	CD91       float64   // CD 91-day rate in percent (e.g., 2.83)
	Baskets    []KTBFuturesBasket
}

// KTBFuturesFairValueResult is the output per tenor.
type KTBFuturesFairValueResult struct {
	Tenor         int
	FairValue     float64
	FuturesExpiry time.Time
}

// ComputeKTBFuturesFairValues computes theoretical fair values for all baskets.
func ComputeKTBFuturesFairValues(in KTBFuturesFairValueInput) ([]KTBFuturesFairValueResult, error) {
	cd91 := in.CD91 / 100.0
	expiry := in.ExpiryDate
	if expiry.IsZero() {
		expiry = calendar.KTBFuturesExpiry(in.Date)
	}

	results := make([]KTBFuturesFairValueResult, 0, len(in.Baskets))
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

		results = append(results, KTBFuturesFairValueResult{
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
