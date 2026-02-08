package bond

import (
	"fmt"
	"math"
	"time"
)

// ForwardYieldInput holds the parameters needed to compute the forward yield
// of a bond delivered via a futures contract.
type ForwardYieldInput struct {
	// SettlementDate is the futures delivery date (e.g. 2026-03-10 for Eurex).
	SettlementDate time.Time
	// FuturesPrice is the clean futures price (e.g. 128.20).
	FuturesPrice float64
	// ConversionFactor maps the futures price to the CTD bond's invoice price.
	ConversionFactor float64
	// CouponRate is the annual coupon in percent (e.g. 2.5 for 2.5%).
	CouponRate float64
	// CouponFrequency is coupons per year (1 = annual, 2 = semi-annual).
	CouponFrequency int
	// Cashflows are the remaining cash flows *after* settlement, in per-100
	// terms. Callers using DB-format cents should divide by 10 000 first.
	Cashflows []Cashflow
}

// ForwardYieldResult is the output of ComputeForwardYield.
type ForwardYieldResult struct {
	// ForwardYield is the annualised yield in percent (e.g. 2.83).
	ForwardYield float64
	// InvoicePrice is futures_price × conversion_factor + accrued_interest (per-100).
	InvoicePrice float64
	// AccruedInterest is the accrued coupon at settlement (per-100).
	AccruedInterest float64
	// Iterations is the number of Newton-Raphson steps taken.
	Iterations int
}

// ComputeForwardYield solves for the yield y such that the dirty-price
// function (ACT/ACT ICMA discounting) equals the invoice price of the
// futures delivery.
//
// The solver uses Newton-Raphson with analytic first derivative.
func ComputeForwardYield(in ForwardYieldInput) (ForwardYieldResult, error) {
	if in.SettlementDate.IsZero() {
		return ForwardYieldResult{}, fmt.Errorf("ComputeForwardYield: SettlementDate is required")
	}
	if len(in.Cashflows) == 0 {
		return ForwardYieldResult{}, fmt.Errorf("ComputeForwardYield: Cashflows are required")
	}
	if in.CouponFrequency <= 0 {
		return ForwardYieldResult{}, fmt.Errorf("ComputeForwardYield: CouponFrequency must be positive")
	}

	// Derive previous coupon date: first cashflow minus one coupon period.
	monthsPerPeriod := 12 / in.CouponFrequency
	prevCoupon := in.Cashflows[0].Date.AddDate(0, -monthsPerPeriod, 0)

	// Accrued interest: coupon × (days from last coupon to settlement) / (days in period).
	daysAccrued := daysBetween(prevCoupon, in.SettlementDate)
	daysPeriod := daysBetween(prevCoupon, in.Cashflows[0].Date)
	accruedInterest := in.CouponRate * float64(daysAccrued) / float64(daysPeriod)

	// Invoice price: futures × CF + AI.
	invoicePrice := in.FuturesPrice*in.ConversionFactor + accruedInterest

	// Newton-Raphson: find y s.t. dirtyPrice(y) = invoicePrice.
	yield, iterations, err := solveYield(invoicePrice, in.SettlementDate, prevCoupon, in.Cashflows)
	if err != nil {
		return ForwardYieldResult{}, err
	}

	return ForwardYieldResult{
		ForwardYield:    yield * 100.0, // decimal → percent
		InvoicePrice:    invoicePrice,
		AccruedInterest: accruedInterest,
		Iterations:      iterations,
	}, nil
}

// ---------------------------------------------------------------------------
// Newton-Raphson solver (unexported)
// ---------------------------------------------------------------------------

const (
	yieldTolerance = 1e-12
	yieldMaxIter   = 100
	yieldFloor     = -0.05
	yieldCeiling   = 0.50
)

// solveYield finds y such that dirtyPrice(y) == target via Newton-Raphson.
func solveYield(target float64, settlement, prevCoupon time.Time, cfs []Cashflow) (float64, int, error) {
	// Initial guess: mid-range (2.5 %).
	y := 0.025
	y = clamp(y, yieldFloor, yieldCeiling)

	for iter := 0; iter < yieldMaxIter; iter++ {
		price, dPdy := dirtyPriceAndDeriv(y, settlement, prevCoupon, cfs)
		f := price - target

		if math.Abs(f) < yieldTolerance {
			return y, iter + 1, nil
		}
		if math.Abs(dPdy) < 1e-15 {
			return y, iter + 1, fmt.Errorf("ComputeForwardYield: derivative too small at iter %d", iter)
		}

		y = clamp(y-f/dPdy, yieldFloor, yieldCeiling)
	}

	return y, yieldMaxIter, fmt.Errorf("ComputeForwardYield: did not converge after %d iterations", yieldMaxIter)
}

// dirtyPriceAndDeriv returns (price, dPrice/dy) using ACT/ACT ICMA.
//
//	t_1  = days(settlement, cf[0]) / days(prevCoupon, cf[0])   (fractional first period)
//	t_k  = t_1 + (k − 1)                                       (annual coupon steps)
//	price = Σ CF_k / (1+y)^t_k
//	dP/dy = Σ −t_k · CF_k / (1+y)^(t_k+1)
func dirtyPriceAndDeriv(y float64, settlement, prevCoupon time.Time, cfs []Cashflow) (float64, float64) {
	if len(cfs) == 0 {
		return 0, 0
	}

	t1 := float64(daysBetween(settlement, cfs[0].Date)) / float64(daysBetween(prevCoupon, cfs[0].Date))

	var price, deriv float64
	for i, cf := range cfs {
		t := t1 + float64(i)
		amt := cf.Amount()
		disc := math.Pow(1.0+y, t)
		price += amt / disc
		deriv += -t * amt / math.Pow(1.0+y, t+1)
	}

	return price, deriv
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// daysBetween returns the number of calendar days from start to end (ACT).
func daysBetween(start, end time.Time) int {
	return int(end.Sub(start).Hours() / 24)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
