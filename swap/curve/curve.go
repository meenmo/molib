package curve

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
	curveDayCount   string
	fixedLegDC      FixedLegDayCount // day count for fixed leg during bootstrap
}

// defaultCurveDayCount returns the time basis for curve construction.
// Following market convention (and QuantLib), the curve time axis uses ACT/365F
// for interpolation and zero rate calculations, regardless of currency.
// Note: Leg-specific day counts (ACT/360 for EUR, etc.) are used separately
// for coupon accrual calculations.
func defaultCurveDayCount(cal calendar.CalendarID) string {
	// Use ACT/365F for curve time axis - this is the standard convention
	// used by QuantLib and Bloomberg for discount curve interpolation.
	return "ACT/365F"
}

// FixedLegDayCount specifies the day count convention for the fixed leg during bootstrap.
type FixedLegDayCount string

const (
	FixedLegDayCountOIS  FixedLegDayCount = "OIS"  // ACT/360 for EUR, ACT/365F for JPY (OIS convention)
	FixedLegDayCountIBOR FixedLegDayCount = "IBOR" // 30/360 for EUR, ACT/365F for JPY (IBOR IRS convention)
)

// BuildCurve creates a par/zero curve using KRX-like bootstrap with 3M spacing.
// Uses OIS conventions (ACT/360 for EUR) for the fixed leg.
func BuildCurve(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range quotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement:    settlement,
		parQuotes:     parsed,
		cal:           cal,
		freqMonths:    freqMonths,
		curveDayCount: defaultCurveDayCount(cal),
		fixedLegDC:    FixedLegDayCountOIS,
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDiscountFactors()
	c.zeros = c.buildZero()
	return c
}

// BuildIBORDiscountCurve creates a discount curve from IBOR swap quotes.
// Uses IBOR IRS conventions (30/360 for EUR fixed leg) instead of OIS conventions.
// This is appropriate for pre-2020 IBOR discounting where swaps were discounted
// at the same IBOR rate (e.g., EURIBOR 6M discounting for EUR swaps).
func BuildIBORDiscountCurve(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range quotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement:    settlement,
		parQuotes:     parsed,
		cal:           cal,
		freqMonths:    freqMonths,
		curveDayCount: defaultCurveDayCount(cal),
		fixedLegDC:    FixedLegDayCountIBOR,
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDiscountFactors()
	c.zeros = c.buildZero()
	return c
}

// NewCurveFromDFs creates a curve from explicitly provided discount factors.
// This is primarily for diagnostics, where we want to isolate valuation from bootstrap
// by injecting exact discount factors from another system (e.g. SWPM/ficclib).
//
// Behaviour:
//   - If freqMonths > 0, the curve is expanded to a regular grid using the same date-generation
//     logic as BuildCurve, and the provided DFs are log-linearly interpolated onto that grid.
//   - If freqMonths <= 0, the curve uses only the provided DF node dates (no grid expansion).
//
// To avoid interpolation affecting results, provide DFs at all cashflow payment dates and
// call with freqMonths <= 0.
func NewCurveFromDFs(settlement time.Time, dfs map[time.Time]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	c := &Curve{
		settlement:      settlement,
		parQuotes:       make(map[float64]float64), // No quotes
		cal:             cal,
		freqMonths:      freqMonths,
		curveDayCount:   defaultCurveDayCount(cal),
		discountFactors: make(map[time.Time]float64, len(dfs)),
	}

	// Copy DFs
	for t, df := range dfs {
		c.discountFactors[t] = df
	}

	// Build the set of dates we want this curve to support.
	var inputDates []time.Time
	for t := range dfs {
		inputDates = append(inputDates, t)
	}
	utils.SortDates(inputDates)

	if freqMonths > 0 {
		// Expand to regular grid and interpolate DFs.
		c.paymentDates = c.generatePaymentDates()
		for _, d := range c.paymentDates {
			if _, ok := c.discountFactors[d]; !ok {
				c.discountFactors[d] = c.interpolateDF(d, inputDates, dfs)
			}
		}
	} else {
		// Use only provided DF node dates.
		c.paymentDates = inputDates
	}

	c.zeros = c.buildZero()
	return c
}

func (c *Curve) interpolateDF(t time.Time, sortedPillars []time.Time, dfs map[time.Time]float64) float64 {
	// Simple log-linear
	if len(sortedPillars) < 2 {
		if len(sortedPillars) == 1 {
			return dfs[sortedPillars[0]]
		}
		return 1.0
	}

	// Find brackets using binary search (handles boundary cases with extrapolation)
	d1, d2 := findBracketOrBoundary(sortedPillars, t)

	df1 := dfs[d1]
	df2 := dfs[d2]

	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)

	if t2 == t1 {
		return df1
	}

	forwardRate := math.Log(df1/df2) / (t2 - t1)
	return df1 * math.Exp(-forwardRate*(tTarget-t1))
}

func (c *Curve) generatePaymentDates() []time.Time {
	// Calculate number of dates based on max tenor from quotes
	maxTenorMonths := c.getMaxTenorMonths()
	numDates := maxTenorMonths/c.freqMonths + 1

	dates := make([]time.Time, 0, numDates)
	for i := 0; i <= numDates; i++ {
		t := c.settlement.AddDate(0, c.freqMonths*i, 0)
		dates = append(dates, calendar.Adjust(c.cal, t))
	}
	return dates
}

// getMaxTenorMonths returns the maximum tenor in months from the par quotes.
func (c *Curve) getMaxTenorMonths() int {
	maxYears := 0.0
	for tenor := range c.parQuotes {
		if tenor > maxYears {
			maxYears = tenor
		}
	}
	// Convert years to months, add buffer for safety
	return int(maxYears*12) + 12
}

func (c *Curve) buildParCurve() map[time.Time]float64 {
	par := make(map[time.Time]float64, len(c.paymentDates))
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
	df := make(map[time.Time]float64, len(c.paymentDates))
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
	for i := 1; i < len(quotedDates); i++ {
		maturity := quotedDates[i]
		parRate := c.parRates[maturity]

		// Build fixed leg schedule for this maturity
		coupons := c.buildOISCoupons(maturity)

		// Solve for DF(maturity)
		df[maturity] = c.solveOISDiscountFactor(quotedDates[:i+1], df, coupons, parRate)
	}

	// Interpolate DFs for all other payment dates using step-forward (log-linear)
	for _, d := range dates {
		if _, ok := df[d]; !ok {
			// Find adjacent quoted dates using binary search
			d1, d2, found := findBracket(quotedDates, d)

			if !found {
				// Handle dates beyond the last quoted date - use flat extrapolation
				if !d.Before(quotedDates[len(quotedDates)-1]) {
					lastQuoted := quotedDates[len(quotedDates)-1]
					df[d] = df[lastQuoted]
				}
				continue
			}

			df1 := df[d1]
			df2 := df[d2]
			t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
			t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
			tTarget := utils.YearFraction(c.settlement, d, c.curveDayCount)
			forwardRate := math.Log(df1/df2) / (t2 - t1)
			df[d] = utils.RoundTo(df1*math.Exp(-forwardRate*(tTarget-t1)), 12)
		}
	}

	return df
}

type oisCoupon struct {
	PaymentDate time.Time
	Accrual     float64
}

// buildOISCoupons generates fixed leg coupons for an OIS from settlement to maturity.
// It assumes annual coupons (common for TONAR/ESTR) and applies currency-specific conventions.
// The day count convention depends on c.fixedLegDC:
//   - FixedLegDayCountOIS: ACT/360 for EUR (OIS convention)
//   - FixedLegDayCountIBOR: 30/360 for EUR (IBOR IRS convention)
func (c *Curve) buildOISCoupons(maturity time.Time) []oisCoupon {
	coupons := []oisCoupon{}

	// Determine conventions based on calendar and fixedLegDC
	payDelay := 0
	accrualDC := "ACT/365F" // Default

	if c.cal == calendar.JP {
		payDelay = 2
		accrualDC = "ACT/365F"
	} else if c.cal == calendar.FD || c.cal == calendar.GT {
		// USD money-market convention for SOFR OIS fixed legs:
		// - ACT/360
		// - payment lag T+2
		if c.fixedLegDC == FixedLegDayCountIBOR {
			// Legacy USD IBOR fixed legs commonly use 30/360.
			accrualDC = "30/360"
		} else {
			accrualDC = "ACT/360"
		}
		payDelay = 2
	} else if c.cal == calendar.TARGET {
		// Use 30/360 for IBOR discounting, ACT/360 for OIS
		if c.fixedLegDC == FixedLegDayCountIBOR {
			accrualDC = "30/360"
			// EUR IBOR IRS fixed legs pay on accrual end date (no payment lag).
			payDelay = 0
		} else {
			// Bloomberg SWPM convention for EUR ESTR OIS fixed legs is 30/360
			// (fixed coupons are computed on a 30/360 basis even though the curve
			// represents an overnight index swap).
			accrualDC = "30/360"
			// EUR OIS fixed legs typically pay T+1 (ESTR convention).
			payDelay = 1
		}
	}

	// Use backward schedule generation (Bloomberg SWPM convention) to avoid date drift
	// from repeated Modified Following adjustments.
	//
	// This is especially important for EUR IBOR/IRS discount curves where annual fixed coupons
	// must align to the swap maturity date.
	months := 12

	// Build unadjusted dates rolling backward from maturity.
	unadjustedDates := []time.Time{}
	current := maturity
	for current.After(c.settlement) {
		unadjustedDates = append([]time.Time{current}, unadjustedDates...)
		current = utils.AddMonth(current, -months)
	}
	unadjustedDates = append([]time.Time{c.settlement}, unadjustedDates...)

	// Build coupons from consecutive date pairs.
	for i := 0; i < len(unadjustedDates)-1; i++ {
		startUnadj := unadjustedDates[i]
		endUnadj := unadjustedDates[i+1]

		accrualStart := calendar.Adjust(c.cal, startUnadj)
		accrualEnd := calendar.Adjust(c.cal, endUnadj)
		payDate := calendar.AddBusinessDays(c.cal, accrualEnd, payDelay)

		alpha := utils.YearFraction(accrualStart, accrualEnd, accrualDC)
		coupons = append(coupons, oisCoupon{PaymentDate: payDate, Accrual: alpha})
	}

	return coupons
}

// solveOISDiscountFactor solves for the discount factor at maturity using Newton-Raphson.
// It handles cases where intermediate coupons fall between pillars.
func (c *Curve) solveOISDiscountFactor(quotedDates []time.Time, df map[time.Time]float64, coupons []oisCoupon, parRate float64) float64 {
	maturity := quotedDates[len(quotedDates)-1]
	prevPillar := quotedDates[len(quotedDates)-2]
	dfPrev := df[prevPillar]

	// Initial guess: assume flat forward from previous pillar
	guess := dfPrev * math.Exp(-0.0*utils.YearFraction(prevPillar, maturity, c.curveDayCount)) // r=0
	// Better guess: existing DF? or 1.0? dfPrev is good.
	guess = dfPrev

	tolerance := 1e-12
	maxIter := 50

	for iter := 0; iter < maxIter; iter++ {
		pvFixed := 0.0
		derivative := 0.0 // d(PV_fixed)/d(DF_maturity)

		for _, cpn := range coupons {
			var d, dPrime float64

			// If coupon payment is on or before previous pillar, DF is known
			if !cpn.PaymentDate.After(prevPillar) {
				d = c.getKnownDF(cpn.PaymentDate, df, quotedDates)
				dPrime = 0.0
			} else {
				// Coupon is in the current unknown interval (prevPillar, maturity]
				// or (rarely) beyond? (Should not happen for standard OIS)
				// Interpolate between prevPillar (known) and maturity (unknown x)
				d, dPrime = c.interpolateUnknownDF(cpn.PaymentDate, prevPillar, dfPrev, maturity, guess)
			}

			pvFixed += d * cpn.Accrual * parRate
			derivative += dPrime * cpn.Accrual * parRate
		}

		// OIS Equation: 1 = PV_fixed + D(maturity)
		// f(x) = PV_fixed + x - 1
		// f'(x) = d(PV_fixed)/dx + 1

		fVal := pvFixed + guess - 1.0
		fPrime := derivative + 1.0

		if math.Abs(fVal) < tolerance {
			return guess
		}

		if math.Abs(fPrime) < 1e-15 {
			break
		}
		guess = guess - fVal/fPrime
	}
	return guess
}

// getKnownDF retrieves or interpolates a DF from already solved pillars.
func (c *Curve) getKnownDF(t time.Time, df map[time.Time]float64, quotedDates []time.Time) float64 {
	if val, ok := df[t]; ok {
		return val
	}

	// Need at least 2 dates for interpolation
	if len(quotedDates) < 2 {
		if len(quotedDates) == 1 {
			return df[quotedDates[0]]
		}
		return 1.0
	}

	// Find bracketing pillars using binary search (handles boundary cases)
	d1, d2 := findBracketOrBoundary(quotedDates, t)

	// Interpolate log-linearly
	df1 := df[d1]
	df2 := df[d2]
	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)

	if t2 == t1 {
		return df1
	}

	forwardRate := math.Log(df1/df2) / (t2 - t1)
	return df1 * math.Exp(-forwardRate*(tTarget-t1))
}

// interpolateUnknownDF interpolates DF at t where endpoint DF(maturity) = x is unknown.
// Returns DF(t) and d(DF(t))/dx.
func (c *Curve) interpolateUnknownDF(t, start time.Time, dfStart float64, end time.Time, x float64) (float64, float64) {
	// Log-linear interpolation:
	// D(t) = D(start) * (D(end)/D(start)) ^ ratio
	// ratio = (t - start) / (end - start) in curve time.
	tStart := utils.YearFraction(c.settlement, start, c.curveDayCount)
	tEnd := utils.YearFraction(c.settlement, end, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)

	if tEnd == tStart {
		return dfStart, 0
	}

	ratio := (tTarget - tStart) / (tEnd - tStart)

	// Safety for x <= 0
	if x <= 1e-9 {
		x = 1e-9
	}

	dfT := math.Pow(dfStart, 1.0-ratio) * math.Pow(x, ratio)
	dDfdx := ratio * dfT / x

	return dfT, dDfdx
}

// bootstrapDualCurve bootstraps IBOR pseudo-discount factors using OIS discounting.
// For each quoted tenor, it solves for the pseudo-DF that makes the IBOR swap NPV = 0
// when discounted at OIS rates. The floatFreqMonths parameter specifies the frequency
// of the floating leg during bootstrap (e.g., 3 for 3M, 6 for 6M).
func (c *Curve) bootstrapDualCurve(oisCurve *Curve, floatFreqMonths int) map[time.Time]float64 {
	dates := c.paymentDates
	pseudoDF := make(map[time.Time]float64, len(dates))

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
	for i := 1; i < len(quotedDates); i++ {
		maturity := quotedDates[i]
		parRate := c.parRates[maturity]

		// Solve for pseudo-DF at this maturity using Newton-Raphson
		px := c.solvePseudoDiscountFactor(quotedDates[:i+1], pseudoDF, oisCurve, parRate, floatFreqMonths)

		pseudoDF[maturity] = px
	}

	// Interpolate pseudo-DFs for all other payment dates using log-linear
	for _, d := range dates {
		if _, ok := pseudoDF[d]; !ok {
			// Find adjacent quoted dates using binary search
			d1, d2, found := findBracket(quotedDates, d)

			if !found {
				// Handle dates beyond the last quoted date - use flat extrapolation
				if !d.Before(quotedDates[len(quotedDates)-1]) {
					lastQuoted := quotedDates[len(quotedDates)-1]
					pseudoDF[d] = pseudoDF[lastQuoted]
				}
				continue
			}

			df1 := pseudoDF[d1]
			df2 := pseudoDF[d2]
			t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
			t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
			tTarget := utils.YearFraction(c.settlement, d, c.curveDayCount)
			forwardRate := math.Log(df1/df2) / (t2 - t1)
			pseudoDF[d] = utils.RoundTo(df1*math.Exp(-forwardRate*(tTarget-t1)), 12)
		}
	}

	return pseudoDF
}

// solvePseudoDiscountFactor solves for the IBOR pseudo-DF using the specified float leg frequency.
func (c *Curve) solvePseudoDiscountFactor(quotedDates []time.Time, pseudoDF map[time.Time]float64, oisCurve *Curve, parRate float64, floatFreqMonths int) float64 {
	maturity := quotedDates[len(quotedDates)-1]
	prevPillar := quotedDates[len(quotedDates)-2]

	// Initial guess: use previous pseudo DF
	guess := pseudoDF[prevPillar]
	if guess == 0 {
		guess = oisCurve.DF(maturity)
	}

	// Newton-Raphson solver
	tolerance := 1e-12
	maxIter := 100

	for iter := 0; iter < maxIter; iter++ {
		// Calculate NPV and derivative using specified float frequency
		npv, derivative := c.evalIBORSwapNPV(quotedDates, pseudoDF, oisCurve, parRate, guess, floatFreqMonths)

		// Robust checks for NaN/Inf
		if math.IsNaN(npv) || math.IsInf(npv, 0) || math.IsNaN(derivative) || math.IsInf(derivative, 0) {
			guess = 0.9 * guess
			if guess < 1e-9 {
				guess = 1e-9
			}
			continue
		}

		if math.Abs(npv) < tolerance {
			return guess
		}

		// Newton step
		if math.Abs(derivative) < 1e-15 {
			break
		}

		delta := npv / derivative

		// Damping
		if math.Abs(delta) > 0.5*guess {
			delta = 0.5 * guess * (delta / math.Abs(delta))
		}

		guess = guess - delta

		// Safety clamp
		if math.IsNaN(guess) || guess <= 1e-9 {
			guess = 1e-9
		}
	}

	return guess
}

// evalIBORSwapNPV evaluates IBOR swap NPV using the specified floating leg frequency.
func (c *Curve) evalIBORSwapNPV(quotedDates []time.Time, pseudoDF map[time.Time]float64, oisCurve *Curve, parRate float64, unknownPseudoDF float64, floatFreqMonths int) (float64, float64) {
	start := quotedDates[0]
	maturity := quotedDates[len(quotedDates)-1]

	// Floating leg daycount: match currency conventions
	floatDayCount := "ACT/365F"
	if c.cal == calendar.TARGET {
		floatDayCount = "ACT/360"
	}

	// Create temporary pseudo-DF map including the unknown value
	tempPseudoDF := make(map[time.Time]float64, len(pseudoDF)+1)
	for k, v := range pseudoDF {
		tempPseudoDF[k] = v
	}
	tempPseudoDF[maturity] = unknownPseudoDF

	// Calculate floating leg PV
	floatPV := 0.0
	floatDerivative := 0.0

	// Generate floating periods using the specified frequency (not c.freqMonths)
	floatingDates := []time.Time{start}
	curr := start
	for {
		nextUnadj := curr.AddDate(0, floatFreqMonths, 0)
		nextAdj := calendar.Adjust(c.cal, nextUnadj)

		if nextAdj.After(maturity) && !nextAdj.Equal(maturity) {
			break
		}

		floatingDates = append(floatingDates, nextAdj)

		if nextAdj.Equal(maturity) {
			break
		}
		curr = nextUnadj
	}

	// Force the last date to be maturity if it wasn't added
	if !floatingDates[len(floatingDates)-1].Equal(maturity) {
		floatingDates = append(floatingDates, maturity)
	}

	// Get previous pillar for interpolation derivative calculation
	prevPillar := quotedDates[len(quotedDates)-2]

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

		// Derivative calculation: include contributions from all periods that depend on unknownPseudoDF
		// For periods between prevPillar and maturity, the interpolated pseudo-DFs depend on the unknown
		if periodEnd.After(prevPillar) {
			// Calculate derivatives of pxStart and pxEnd w.r.t. unknownPseudoDF
			dPxStart := c.interpolatePseudoDiscountFactorDerivative(periodStart, tempPseudoDF, quotedDates, maturity, unknownPseudoDF)
			dPxEnd := c.interpolatePseudoDiscountFactorDerivative(periodEnd, tempPseudoDF, quotedDates, maturity, unknownPseudoDF)

			// d(forward)/d(unknown) = (1/pxEnd * dPxStart - pxStart/pxEnd^2 * dPxEnd) / accrual
			dForward := (dPxStart/pxEnd - pxStart*dPxEnd/(pxEnd*pxEnd)) / accrual
			floatDerivative += accrual * oisDF * dForward
		}
	}

	// Calculate fixed leg PV (pay fixed at parRate, discounted at OIS)
	// - TARGET (EUR): Annual (12M), 30E/360 (matches SWPM/ficclib for IBOR swap quotes)
	// - JPN (JPY): Semi-Annual (6M), ACT/365F
	fixedDayCount := "ACT/365F"
	fixedFreqMonths := 12
	if c.cal == calendar.TARGET {
		fixedDayCount = "30E/360"
		fixedFreqMonths = 12
	} else if c.cal == calendar.JP {
		fixedDayCount = "ACT/365F"
		fixedFreqMonths = 6
	}

	fixedPV := 0.0
	currUnadj := start
	prevAdj := start

	for {
		currUnadj = currUnadj.AddDate(0, fixedFreqMonths, 0)
		paymentDate := calendar.Adjust(c.cal, currUnadj)

		if paymentDate.After(maturity) && !paymentDate.Equal(maturity) {
			break
		}

		accrual := utils.YearFraction(prevAdj, paymentDate, fixedDayCount)
		oisDF := oisCurve.DF(paymentDate)
		fixedPV += oisDF * accrual * parRate

		prevAdj = paymentDate

		if paymentDate.Equal(maturity) {
			break
		}
	}

	// Handle case where we didn't hit maturity
	if !prevAdj.Equal(maturity) {
		accrual := utils.YearFraction(prevAdj, maturity, fixedDayCount)
		oisDF := oisCurve.DF(maturity)
		fixedPV += oisDF * accrual * parRate
	}

	// NPV = floatPV - fixedPV (receive float, pay fixed)
	npv := floatPV - fixedPV

	return npv, floatDerivative
}

// interpolatePseudoDiscountFactorDerivative calculates the derivative of the interpolated pseudo-DF
// at target date with respect to the unknown pseudo-DF at maturity.
// This is needed for Newton-Raphson convergence during bootstrap.
func (c *Curve) interpolatePseudoDiscountFactorDerivative(target time.Time, pseudoDF map[time.Time]float64, quotedDates []time.Time, maturity time.Time, unknownPseudoDF float64) float64 {
	// If target is exactly at maturity, derivative is 1
	if target.Equal(maturity) {
		return 1.0
	}

	// If target is at or before the previous pillar, it doesn't depend on unknownPseudoDF
	prevPillar := quotedDates[len(quotedDates)-2]
	if !target.After(prevPillar) {
		return 0.0
	}

	// Target is between prevPillar and maturity, so it's interpolated and depends on unknownPseudoDF
	// For log-linear interpolation: D(t) = D(prev)^(1-r) * D(mat)^r
	// where r = (t - t_prev) / (t_mat - t_prev)
	// Derivative: dD(t)/dD(mat) = r * D(t) / D(mat)

	t1 := utils.YearFraction(c.settlement, prevPillar, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, maturity, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, target, c.curveDayCount)

	if t2 == t1 {
		return 0.0
	}

	ratio := (tTarget - t1) / (t2 - t1)

	// Get the interpolated pseudo-DF at target
	pxTarget := c.interpolatePseudoDiscountFactor(target, pseudoDF, quotedDates)

	// Safety check for unknownPseudoDF
	if unknownPseudoDF <= 1e-9 {
		return 0.0
	}

	return ratio * pxTarget / unknownPseudoDF
}

// interpolatePseudoDiscountFactor interpolates pseudo-DF at target date using log-linear interpolation
func (c *Curve) interpolatePseudoDiscountFactor(target time.Time, pseudoDF map[time.Time]float64, quotedDates []time.Time) float64 {
	// If exact match, return it
	if px, ok := pseudoDF[target]; ok {
		return px
	}

	// Find bracketing dates using binary search (handles boundary cases with extrapolation)
	d1, d2 := findBracketOrBoundary(quotedDates, target)

	// Log-linear interpolation (or extrapolation for boundary cases)
	px1 := pseudoDF[d1]
	px2 := pseudoDF[d2]
	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, target, c.curveDayCount)

	forwardRate := math.Log(px1/px2) / (t2 - t1)
	return px1 * math.Exp(-forwardRate*(tTarget-t1))
}

func (c *Curve) buildZero() map[time.Time]float64 {
	zc := make(map[time.Time]float64, len(c.paymentDates))

	for i, d := range c.paymentDates {
		if i == 0 {
			zc[d] = utils.RoundTo(c.parRates[d]*100, 12)
		} else {
			df := c.discountFactors[d]
			yearFrac := utils.YearFraction(c.settlement, d, c.curveDayCount)
			zc[d] = utils.RoundTo(-math.Log(df)/yearFrac*100, 12)

		}
	}
	return zc
}

func (c *Curve) paymentDatesToTenor() map[time.Time]float64 {
	m := make(map[time.Time]float64, len(c.paymentDates))
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
	df := c.DF(t)
	yearFrac := utils.YearFraction(c.settlement, t, c.curveDayCount)
	if yearFrac == 0 {
		return 0
	}
	return utils.RoundTo(-math.Log(df)/yearFrac*100, 12)
}

func (c *Curve) DF(t time.Time) float64 {
	if df, ok := c.discountFactors[t]; ok {
		return df
	}
	d1, d2 := utils.AdjacentDates(t, c.paymentDates)
	df1 := c.discountFactors[d1]
	df2 := c.discountFactors[d2]

	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)

	if t2 == t1 {
		return df1
	}
	forwardRate := math.Log(df1/df2) / (t2 - t1)
	return utils.RoundTo(df1*math.Exp(-forwardRate*(tTarget-t1)), 12)
}

// Settlement returns the curve's settlement date.
func (c *Curve) Settlement() time.Time {
	return c.settlement
}

// DayCount returns the curve's day count convention.
func (c *Curve) DayCount() string {
	return c.curveDayCount
}

// PillarDFs returns all bootstrapped discount factors keyed by date.
// For diagnostic purposes only.
func (c *Curve) PillarDFs() map[time.Time]float64 {
	result := make(map[time.Time]float64)
	for k, v := range c.discountFactors {
		result[k] = v
	}
	return result
}

// PaymentDates returns the curve's payment date grid.
func (c *Curve) PaymentDates() []time.Time {
	return c.paymentDates
}

// ParQuotes returns the input par quotes (tenor -> rate%).
func (c *Curve) ParQuotes() map[float64]float64 {
	return c.parQuotes
}
