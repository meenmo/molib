package basis

import (
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

type Curve struct {
	settlement      time.Time
	parQuotes       map[float64]float64 // tenor (years) -> percent
	paymentDates    []time.Time
	parRates        map[time.Time]float64
	discountFactors map[time.Time]float64
	zeros           map[time.Time]float64 // percent
	cal             calendar.CalendarID
	freqMonths      int
}

// BuildCurve creates a par/zero curve using KRX-like bootstrap with 3M spacing.
func BuildCurve(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range quotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement: settlement,
		parQuotes:  parsed,
		cal:        cal,
		freqMonths: freqMonths,
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDiscountFactors()
	c.zeros = c.buildZero()
	return c
}

func (c *Curve) generatePaymentDates() []time.Time {
	dates := make([]time.Time, 0, 600)
	for i := 0; i <= 600; i++ { // up to 50Y for monthly; adjust freq
		t := c.settlement.AddDate(0, c.freqMonths*i, 0)
		dates = append(dates, calendar.Adjust(c.cal, t))
	}
	return dates
}

func (c *Curve) buildParCurve() map[time.Time]float64 {
	par := make(map[time.Time]float64)
	dateToTenor := c.paymentDatesToTenor()
	for _, d := range c.paymentDates {
		tenor := dateToTenor[d]
		if rate, ok := c.parQuotes[tenor]; ok {
			par[d] = rate / 100.0
		} else {
			d1, d2 := c.adjacentQuotedDates(d, dateToTenor)
			r1 := c.parQuotes[dateToTenor[d1]]
			r2 := c.parQuotes[dateToTenor[d2]]
			par[d] = (r1 + (r2-r1)*utils.Days(d1, d)/utils.Days(d1, d2)) / 100.0
		}
	}
	return par
}

func (c *Curve) bootstrapDiscountFactors() map[time.Time]float64 {
	df := make(map[time.Time]float64)
	dates := c.paymentDates

	// First pillar at settlement has DF = 1.0
	df[dates[0]] = 1.0

	// Only bootstrap dates that have explicit par quotes (quoted tenors)
	dateToTenor := c.paymentDatesToTenor()
	quotedDates := []time.Time{dates[0]}
	for _, d := range dates[1:] {
		tenor := dateToTenor[d]
		if _, ok := c.parQuotes[tenor]; ok {
			quotedDates = append(quotedDates, d)
		}
	}

	// Bootstrap each quoted pillar sequentially
	// For OIS: par swap equation is: 1 = sum(DF_i * alpha_i * r) + DF_n
	// Rearranged: DF_n = (1 - sum(DF_i * alpha_i * r)) / (1 + r * alpha_n)

	for i := 1; i < len(quotedDates); i++ {
		maturity := quotedDates[i]
		parRate := c.parRates[maturity]

		// Sum of previous coupon PVs: sum(DF_i * alpha_i * r)
		sumCouponPV := 0.0
		prev := quotedDates[0]
		for j := 1; j < i; j++ {
			curr := quotedDates[j]
			accrual := utils.Days(prev, curr) / 365.0
			sumCouponPV += df[curr] * accrual * parRate
			prev = curr
		}

		// Last period accrual
		lastAccrual := utils.Days(quotedDates[i-1], maturity) / 365.0

		// Solve: 1 = sumCouponPV + DF_n * (1 + r * alpha_n)
		// DF_n = (1 - sumCouponPV) / (1 + r * alpha_n)
		numerator := 1.0 - sumCouponPV
		denominator := 1.0 + parRate * lastAccrual
		df[maturity] = utils.RoundTo(numerator / denominator, 12)
	}

	// Interpolate DFs for all other payment dates using step-forward (log-linear)
	for _, d := range dates {
		if _, ok := df[d]; !ok {
			// Find adjacent quoted dates
			var d1, d2 time.Time
			for j := 0; j < len(quotedDates)-1; j++ {
				if quotedDates[j].Before(d) && (d.Before(quotedDates[j+1]) || d.Equal(quotedDates[j+1])) {
					d1 = quotedDates[j]
					d2 = quotedDates[j+1]
					break
				}
			}

			// Handle dates beyond the last quoted date - use flat extrapolation
			if d1.IsZero() && !d.Before(quotedDates[len(quotedDates)-1]) {
				lastQuoted := quotedDates[len(quotedDates)-1]
				df[d] = df[lastQuoted]
				continue
			}

			if !d1.IsZero() && !d2.IsZero() {
				df1 := df[d1]
				df2 := df[d2]
				t1 := utils.Days(c.settlement, d1) / 365.0
				t2 := utils.Days(c.settlement, d2) / 365.0
				tTarget := utils.Days(c.settlement, d) / 365.0
				forwardRate := math.Log(df1/df2) / (t2 - t1)
				df[d] = utils.RoundTo(df1 * math.Exp(-forwardRate*(tTarget-t1)), 12)
			}
		}
	}

	return df
}

// BuildDualCurve creates an IBOR projection curve using dual-curve bootstrap.
// It uses OIS discount factors for discounting while solving for IBOR forward rates
// that match market IBOR swap quotes.
func BuildDualCurve(settlement time.Time, iborQuotes map[string]float64, oisCurve *Curve, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range iborQuotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement: settlement,
		parQuotes:  parsed,
		cal:        cal,
		freqMonths: freqMonths,
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDualCurveDiscountFactors(oisCurve)
	c.zeros = c.buildZero()
	return c
}

// bootstrapDualCurveDiscountFactors bootstraps IBOR pseudo-discount factors using OIS discounting.
// For each quoted tenor, it solves for the pseudo-DF that makes the IBOR swap NPV = 0
// when discounted at OIS rates.
func (c *Curve) bootstrapDualCurveDiscountFactors(oisCurve *Curve) map[time.Time]float64 {
	pseudoDF := make(map[time.Time]float64)
	dates := c.paymentDates

	// First pillar at settlement has pseudo-DF = 1.0
	pseudoDF[dates[0]] = 1.0

	// Get quoted dates
	dateToTenor := c.paymentDatesToTenor()
	quotedDates := []time.Time{dates[0]}
	for _, d := range dates[1:] {
		tenor := dateToTenor[d]
		if _, ok := c.parQuotes[tenor]; ok {
			quotedDates = append(quotedDates, d)
		}
	}

	// Bootstrap each quoted pillar sequentially
	// For IBOR swap at quoted tenor: NPV = 0
	// NPV = sum(DF_ois_i * alpha_i * forward_i) - sum(DF_ois_i * alpha_i * r_fixed)
	// Where forward_i = (pseudo_DF_start / pseudo_DF_end - 1) / alpha

	for i := 1; i < len(quotedDates); i++ {
		maturity := quotedDates[i]
		parRate := c.parRates[maturity]

		// Solve for pseudo-DF at this maturity using Newton-Raphson
		px := c.solvePseudoDiscountFactor(quotedDates[:i+1], pseudoDF, oisCurve, parRate)


		pseudoDF[maturity] = px
	}

	// Interpolate pseudo-DFs for all other payment dates using log-linear
	for _, d := range dates {
		if _, ok := pseudoDF[d]; !ok {
			// Find adjacent quoted dates
			var d1, d2 time.Time
			for j := 0; j < len(quotedDates)-1; j++ {
				if quotedDates[j].Before(d) && (d.Before(quotedDates[j+1]) || d.Equal(quotedDates[j+1])) {
					d1 = quotedDates[j]
					d2 = quotedDates[j+1]
					break
				}
			}

			// Handle dates beyond the last quoted date - use flat extrapolation
			if d1.IsZero() && !d.Before(quotedDates[len(quotedDates)-1]) {
				lastQuoted := quotedDates[len(quotedDates)-1]
				pseudoDF[d] = pseudoDF[lastQuoted]
				continue
			}

			if !d1.IsZero() && !d2.IsZero() {
				df1 := pseudoDF[d1]
				df2 := pseudoDF[d2]
				t1 := utils.Days(c.settlement, d1) / 365.0
				t2 := utils.Days(c.settlement, d2) / 365.0
				tTarget := utils.Days(c.settlement, d) / 365.0
				forwardRate := math.Log(df1/df2) / (t2 - t1)
				pseudoDF[d] = utils.RoundTo(df1*math.Exp(-forwardRate*(tTarget-t1)), 12)
			}
		}
	}

	return pseudoDF
}

// solvePseudoDiscountFactor solves for the IBOR pseudo-DF at a maturity that makes swap NPV = 0.
func (c *Curve) solvePseudoDiscountFactor(quotedDates []time.Time, pseudoDF map[time.Time]float64, oisCurve *Curve, parRate float64) float64 {
	maturity := quotedDates[len(quotedDates)-1]

	// Initial guess: use OIS DF as starting point
	guess := oisCurve.DF(maturity)

	// Newton-Raphson solver
	tolerance := 1e-12
	maxIter := 100

	for iter := 0; iter < maxIter; iter++ {
		// Calculate NPV and derivative
		npv, derivative := c.evalIBORSwapNPV(quotedDates, pseudoDF, oisCurve, parRate, guess)

		if math.Abs(npv) < tolerance {
			return guess
		}

		// Newton step
		if math.Abs(derivative) < 1e-15 {
			break
		}
		guess = guess - npv/derivative
	}

	return guess
}

// evalIBORSwapNPV evaluates the NPV of an IBOR swap and its derivative w.r.t. the unknown pseudo-DF.
// Uses dual-curve: floating leg uses pseudo-DFs for forwards, OIS DFs for discounting
func (c *Curve) evalIBORSwapNPV(quotedDates []time.Time, pseudoDF map[time.Time]float64, oisCurve *Curve, parRate float64, unknownPseudoDF float64) (float64, float64) {
	start := quotedDates[0]
	maturity := quotedDates[len(quotedDates)-1]

	// Floating leg daycount: match currency conventions used in valuation.
	floatDayCount := "ACT/365F"
	if c.cal == calendar.TARGET {
		floatDayCount = "ACT/360"
	}

	// Create temporary pseudo-DF map including the unknown value
	tempPseudoDF := make(map[time.Time]float64)
	for k, v := range pseudoDF {
		tempPseudoDF[k] = v
	}
	tempPseudoDF[maturity] = unknownPseudoDF

	// Calculate floating leg PV: sum of IBOR forward payments discounted at OIS
	// For each floating period (at the IBOR tenor frequency):
	// forward = (pseudoDF_start / pseudoDF_end - 1) / accrual
	// pv = forward * accrual * oisDF
	floatPV := 0.0
	floatDerivative := 0.0

	// Generate floating periods (use freqMonths from curve)
	floatingDates := []time.Time{start}
	for d := start.AddDate(0, c.freqMonths, 0); !d.After(maturity); d = d.AddDate(0, c.freqMonths, 0) {
		floatingDates = append(floatingDates, d)
	}
	// Ensure maturity is included
	if floatingDates[len(floatingDates)-1].Before(maturity) {
		floatingDates = append(floatingDates, maturity)
	}

	for i := 1; i < len(floatingDates); i++ {
		periodStart := floatingDates[i-1]
		periodEnd := floatingDates[i]
		accrual := utils.YearFraction(periodStart, periodEnd, floatDayCount)

		// Get pseudo-DFs at period boundaries (interpolate if needed)
		pxStart := c.interpolatePseudoDiscountFactor(periodStart, tempPseudoDF, quotedDates)
		pxEnd := c.interpolatePseudoDiscountFactor(periodEnd, tempPseudoDF, quotedDates)

		// Forward rate
		forward := (pxStart/pxEnd - 1.0) / accrual

		// Discount at OIS
		oisDF := oisCurve.DF(periodEnd)

		// Cashflow PV
		cf := forward * accrual * oisDF
		floatPV += cf

		// Derivative: only the last period's pxEnd = unknownPseudoDF contributes
		if periodEnd.Equal(maturity) {
			// d(forward)/d(pxEnd) = -pxStart / (pxEnd^2 * accrual)
			dForward := -pxStart / (pxEnd * pxEnd * accrual)
			floatDerivative += accrual * oisDF * dForward
		}
	}

	// Calculate fixed leg PV (pay fixed at parRate, discounted at OIS)
	// Fixed leg has ANNUAL payments. Use currency-specific daycount:
	// - TARGET (EUR): ACT/360
	// - JPN (JPY): ACT/365F
	fixedDayCount := "ACT/365F"
	if c.cal == calendar.TARGET {
		fixedDayCount = "ACT/360"
	}

	years := utils.Days(start, maturity) / 365.0
	numFixedPeriods := int(math.Ceil(years))

	fixedPV := 0.0
	prevDate := start
	for i := 1; i <= numFixedPeriods; i++ {
		// Payment date is i years from start
		paymentDate := start.AddDate(i, 0, 0)
		// Adjust if beyond maturity
		if paymentDate.After(maturity) {
			paymentDate = maturity
		}

		// Daycount-consistent accrual for fixed leg
		accrual := utils.YearFraction(prevDate, paymentDate, fixedDayCount)
		prevDate = paymentDate

		oisDF := oisCurve.DF(paymentDate)
		fixedPV += oisDF * accrual * parRate
	}

	// NPV = floatPV - fixedPV (receive float, pay fixed)
	npv := floatPV - fixedPV

	return npv, floatDerivative
}

// interpolatePseudoDiscountFactor interpolates pseudo-DF at target date using log-linear interpolation
func (c *Curve) interpolatePseudoDiscountFactor(target time.Time, pseudoDF map[time.Time]float64, quotedDates []time.Time) float64 {
	// If exact match, return it
	if px, ok := pseudoDF[target]; ok {
		return px
	}

	// Find bracketing dates
	var d1, d2 time.Time
	for i := 0; i < len(quotedDates)-1; i++ {
		if (quotedDates[i].Before(target) || quotedDates[i].Equal(target)) &&
			(target.Before(quotedDates[i+1]) || target.Equal(quotedDates[i+1])) {
			d1 = quotedDates[i]
			d2 = quotedDates[i+1]
			break
		}
	}

	// If no bracketing dates found, use flat extrapolation from last known
	if d1.IsZero() {
		// Use last known value
		lastDate := quotedDates[0]
		for _, d := range quotedDates {
			if d.After(lastDate) {
				lastDate = d
			}
		}
		return pseudoDF[lastDate]
	}

	// Log-linear interpolation
	px1 := pseudoDF[d1]
	px2 := pseudoDF[d2]
	t1 := utils.Days(c.settlement, d1) / 365.0
	t2 := utils.Days(c.settlement, d2) / 365.0
	tTarget := utils.Days(c.settlement, target) / 365.0

	forwardRate := math.Log(px1/px2) / (t2 - t1)
	return px1 * math.Exp(-forwardRate*(tTarget-t1))
}

func (c *Curve) buildZero() map[time.Time]float64 {
	zc := make(map[time.Time]float64)

	for i, d := range c.paymentDates {
		if i == 0 {
			zc[d] = utils.RoundTo(c.parRates[d]*100, 12)
		} else {
			df := c.discountFactors[d]
			yearFrac := utils.Days(c.settlement, d) / 365.0
			zc[d] = utils.RoundTo(-math.Log(df)/yearFrac*100, 12)

		}
	}
	return zc
}

func (c *Curve) paymentDatesToTenor() map[time.Time]float64 {
	m := make(map[time.Time]float64)
	for i, d := range c.paymentDates {
		// Calculate tenor directly from index to avoid floating point accumulation errors
		months := i * c.freqMonths
		tenor := float64(months) / 12.0
		m[d] = tenor
	}
	return m
}

func (c *Curve) adjacentQuotedDates(target time.Time, dateToTenor map[time.Time]float64) (time.Time, time.Time) {
	d1 := c.paymentDates[0]
	d2 := c.paymentDates[1]
	for _, d := range c.paymentDates[2:] {
		if d1.Before(target) && target.Before(d2) {
			return d1, d2
		}
		tenor := dateToTenor[d]
		if _, ok := c.parQuotes[tenor]; ok {
			d1 = d2
			d2 = d
		}
	}
	return d1, d2
}

func (c *Curve) ZeroRateAt(t time.Time) float64 {
	if z, ok := c.zeros[t]; ok {
		return z
	}
	d1, d2 := adjacentDates(t, c.paymentDates)
	r1 := c.zeros[d1]
	r2 := c.zeros[d2]
	return utils.RoundTo(r1+(r2-r1)*utils.Days(d1, t)/utils.Days(d1, d2), 12)
}

func (c *Curve) DF(t time.Time) float64 {
	z := c.ZeroRateAt(t)
	return math.Exp(-(utils.Days(c.settlement, t) / 365.0) * (z / 100.0))
}
