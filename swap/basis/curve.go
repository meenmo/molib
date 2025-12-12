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
	curveDayCount   string
}

// defaultCurveDayCount selects a time basis for curve construction
// based on the calendar (i.e., currency).
func defaultCurveDayCount(cal calendar.CalendarID) string {
	switch cal {
	case calendar.JPN:
		return "ACT/365F"
	case calendar.TARGET:
		return "ACT/365F"
	default:
		return "ACT/365F"
	}
}

// BuildCurve creates a par/zero curve using KRX-like bootstrap with 3M spacing.
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
	}
	c.paymentDates = c.generatePaymentDates()
	c.parRates = c.buildParCurve()
	c.discountFactors = c.bootstrapDiscountFactors()
	c.zeros = c.buildZero()
	return c
}

// NewCurveFromDFs creates a curve from explicitly provided discount factors.
// This is useful for diagnostics where we want to isolate valuation from bootstrap by injecting
// exact discount factors from another system (e.g. SWPM).
func NewCurveFromDFs(settlement time.Time, dfs map[time.Time]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	c := &Curve{
		settlement:      settlement,
		parQuotes:       make(map[float64]float64), // No quotes
		cal:             cal,
		freqMonths:      freqMonths,
		curveDayCount:   defaultCurveDayCount(cal),
		discountFactors: make(map[time.Time]float64),
	}
	
	// Copy DFs
	for t, df := range dfs {
		c.discountFactors[t] = df
	}
	
	// We need paymentDates for interpolation logic in DF() and ZeroRateAt().
	// In standard BuildCurve, this is a dense monthly grid.
	// Here, we can either replicate that grid and interpolate the input DFs onto it,
	// or assume the input DFs *are* the grid.
	// For safety, let's generate the standard grid and interpolate the input DFs onto it.
	c.paymentDates = c.generatePaymentDates()
	
	// Sort input dates
	var inputDates []time.Time
	for t := range dfs {
		inputDates = append(inputDates, t)
	}
	
	// Efficient sort
	utils.SortDates(inputDates)
	// c.paymentDates = inputDates // Use standard grid for now to avoid gaps? 
	// Actually, using standard grid is safer for interpolation if input is sparse.
	// We already populated discountFactors from input. 
	// If the grid points are NOT in input, DF() will panic or return 0?
	// DF() calls ZeroRateAt() which calls adjacentDates(t, c.paymentDates).
	// Then it looks up c.zeros[d1].
	// c.zeros is built from c.discountFactors[d].
	// So we need c.discountFactors to have entries for ALL c.paymentDates.
	
	// Let's populate the grid DFs by interpolating from the input DFs.
	for _, d := range c.paymentDates {
		if _, ok := c.discountFactors[d]; !ok {
			// Interpolate from inputDates
			c.discountFactors[d] = c.interpolateDF(d, inputDates, dfs)
		}
	}

	c.zeros = c.buildZero()
	return c
}

func (c *Curve) interpolateDF(t time.Time, sortedPillars []time.Time, dfs map[time.Time]float64) float64 {
	// Simple log-linear
	if len(sortedPillars) == 0 { return 1.0 }
	
	// Find brackets
	var d1, d2 time.Time
	for i := 0; i < len(sortedPillars)-1; i++ {
		if (sortedPillars[i].Before(t) || sortedPillars[i].Equal(t)) &&
			(t.Before(sortedPillars[i+1]) || t.Equal(sortedPillars[i+1])) {
			d1 = sortedPillars[i]
			d2 = sortedPillars[i+1]
			break
		}
	}
	
	if d1.IsZero() {
		// Extrapolate flat
		if t.Before(sortedPillars[0]) { return dfs[sortedPillars[0]] }
		return dfs[sortedPillars[len(sortedPillars)-1]]
	}
	
	df1 := dfs[d1]
	df2 := dfs[d2]
	
	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)
	
	if t2 == t1 { return df1 }
	
	forwardRate := math.Log(df1/df2) / (t2 - t1)
	return df1 * math.Exp(-forwardRate*(tTarget-t1))
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
				t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
				t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
				tTarget := utils.YearFraction(c.settlement, d, c.curveDayCount)
				forwardRate := math.Log(df1/df2) / (t2 - t1)
				df[d] = utils.RoundTo(df1*math.Exp(-forwardRate*(tTarget-t1)), 12)
			}
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
func (c *Curve) buildOISCoupons(maturity time.Time) []oisCoupon {
	coupons := []oisCoupon{}
	start := c.settlement
	current := start

	// Determine conventions based on calendar
	payDelay := 0
	accrualDC := "ACT/365F" // Default

	if c.cal == calendar.JPN {
		payDelay = 2
		accrualDC = "ACT/365F"
	} else if c.cal == calendar.TARGET {
		payDelay = 1
		accrualDC = "ACT/360"
	}

	// Generate annual coupons
	for {
		// Move forward 1 year
		nextUnadj := current.AddDate(1, 0, 0)

		// Check if we reached or passed maturity
		if !nextUnadj.Before(maturity) {
			break
		}

		// Intermediate coupon
		accrualEnd := calendar.Adjust(c.cal, nextUnadj)
		payDate := calendar.AddBusinessDays(c.cal, accrualEnd, payDelay)
		alpha := utils.YearFraction(start, accrualEnd, accrualDC)

		coupons = append(coupons, oisCoupon{PaymentDate: payDate, Accrual: alpha})

		start = accrualEnd
		current = nextUnadj
	}

	// Final coupon ending at maturity
	// Note: 'maturity' passed here is usually an adjusted date from the grid
	payDate := calendar.AddBusinessDays(c.cal, maturity, payDelay)
	alpha := utils.YearFraction(start, maturity, accrualDC)
	coupons = append(coupons, oisCoupon{PaymentDate: payDate, Accrual: alpha})

	return coupons
}

// solveOISDiscountFactor solves for the discount factor at maturity using Newton-Raphson.
// It handles cases where intermediate coupons fall between pillars.
func (c *Curve) solveOISDiscountFactor(quotedDates []time.Time, df map[time.Time]float64, coupons []oisCoupon, parRate float64) float64 {
	maturity := quotedDates[len(quotedDates)-1]
	prevPillar := quotedDates[len(quotedDates)-2]
	dfPrev := df[prevPillar]

	// Initial guess: assume flat forward from previous pillar
	guess := dfPrev * math.Exp(-0.0 * utils.YearFraction(prevPillar, maturity, c.curveDayCount)) // r=0
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
	// Find bracketing pillars
	// quotedDates is sorted.
	// We only look up to len(quotedDates)-2 (excluding the one currently being solved if passed?
	// The caller passes quotedDates[:i+1] which INCLUDES current.
	// But this function is called for t <= prevPillar.
	// So we search in quotedDates.
	var d1, d2 time.Time
	for i := 0; i < len(quotedDates)-1; i++ {
		if (quotedDates[i].Before(t) || quotedDates[i].Equal(t)) &&
			(t.Before(quotedDates[i+1]) || t.Equal(quotedDates[i+1])) {
			d1 = quotedDates[i]
			d2 = quotedDates[i+1]
			break
		}
	}

	if d1.IsZero() {
		return df[quotedDates[0]] // Fallback
	}

	// Interpolate
	df1 := df[d1]
	df2 := df[d2]
	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, t, c.curveDayCount)

	forwardRate := math.Log(df1/df2) / (t2 - t1)
	return df1 * math.Exp(-forwardRate*(tTarget-t1))
}

// interpolateUnknownDF interpolates DF at t where endpoint DF(maturity) = x is unknown.
// Returns DF(t) and d(DF(t))/dx.
func (c *Curve) interpolateUnknownDF(t, start time.Time, dfStart float64, end time.Time, x float64) (float64, float64) {
	// Log-linear interpolation:
	// D(t) = D(start) * (D(end)/D(start)) ^ ratio
	// ratio = (t - start) / (end - start)
	// Let r = ratio.
	// D(t) = dfStart * (x / dfStart) ^ r
	//      = dfStart * x^r * dfStart^(-r)
	//      = dfStart^(1-r) * x^r

	// Derivative dD(t)/dx:
	// = dfStart^(1-r) * r * x^(r-1)
	// = r * (dfStart^(1-r) * x^r) / x
	// = r * D(t) / x

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

// BuildDualCurve creates an IBOR projection curve using dual-curve bootstrap.
// It uses OIS discount factors for discounting while solving for IBOR forward rates
// that match market IBOR swap quotes.
func BuildDualCurve(settlement time.Time, iborQuotes map[string]float64, oisCurve *Curve, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range iborQuotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement:    settlement,
		parQuotes:     parsed,
		cal:           cal,
		freqMonths:    freqMonths,
		// Use the same time basis as the OIS curve for consistency.
		curveDayCount: oisCurve.curveDayCount,
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
				t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
				t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
				tTarget := utils.YearFraction(c.settlement, d, c.curveDayCount)
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
	prevPillar := quotedDates[len(quotedDates)-2]

	// Initial guess: use previous pseudo DF (better continuity)
	guess := pseudoDF[prevPillar]
	if guess == 0 {
		guess = oisCurve.DF(maturity)
	}

	// Newton-Raphson solver
	tolerance := 1e-12
	maxIter := 100

	for iter := 0; iter < maxIter; iter++ {
		// Calculate NPV and derivative
		npv, derivative := c.evalIBORSwapNPV(quotedDates, pseudoDF, oisCurve, parRate, guess)

		// Robust checks for NaN/Inf
		if math.IsNaN(npv) || math.IsInf(npv, 0) || math.IsNaN(derivative) || math.IsInf(derivative, 0) {
			// Backtrack or reset if numeric instability
			guess = 0.9 * guess // decay
			if guess < 1e-9 { guess = 1e-9 }
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
		
		// Damping: prevent large steps that might cross zero
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
	floatPV := 0.0
	floatDerivative := 0.0

	// Generate floating periods using explicit calendar adjustment.
	// This ensures proper matching with maturity date for derivative calculation.
	floatingDates := []time.Time{start}
	curr := start
	for {
		nextUnadj := curr.AddDate(0, c.freqMonths, 0)
		nextAdj := calendar.Adjust(c.cal, nextUnadj)
		
		if nextAdj.After(maturity) && !nextAdj.Equal(maturity) {
			// Stop if we overshoot maturity
			break
		}
		
		floatingDates = append(floatingDates, nextAdj)
		
		if nextAdj.Equal(maturity) {
			break
		}
		curr = nextUnadj // Advance using unadjusted
	}
	
	// Force the last date to be maturity if it wasn't added and we are close?
	// Standard bootstrap pillars should align. If they don't, we might have issues.
	// But let's trust the loop found maturity if it exists.
	// If the loop didn't hit maturity, we need to add it?
	if !floatingDates[len(floatingDates)-1].Equal(maturity) {
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
	// Fixed leg frequency depends on currency:
	// - TARGET (EUR): Annual (12M), ACT/360
	// - JPN (JPY): Semi-Annual (6M), ACT/365F
	fixedDayCount := "ACT/365F"
	fixedFreqMonths := 12
	if c.cal == calendar.TARGET {
		fixedDayCount = "ACT/360"
		fixedFreqMonths = 12
	} else if c.cal == calendar.JPN {
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
	
	// Handle case where we didn't hit maturity?
	// If standard grid, we should hit it.
	// If not, force append?
	if !prevAdj.Equal(maturity) {
		accrual := utils.YearFraction(prevAdj, maturity, fixedDayCount)
		oisDF := oisCurve.DF(maturity)
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
	t1 := utils.YearFraction(c.settlement, d1, c.curveDayCount)
	t2 := utils.YearFraction(c.settlement, d2, c.curveDayCount)
	tTarget := utils.YearFraction(c.settlement, target, c.curveDayCount)

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
			yearFrac := utils.YearFraction(c.settlement, d, c.curveDayCount)
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
	yearFrac := utils.YearFraction(c.settlement, t, c.curveDayCount)
	return math.Exp(-yearFrac * (z / 100.0))
}
