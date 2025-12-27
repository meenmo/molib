package swap

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
	"github.com/meenmo/molib/utils"
)

func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// SpotEffectiveMaturity computes spot (T+2), effective, and maturity dates from a trade date.
//
// Conventions:
// - spot = tradeDate + 2 business days on cal
// - effective = spot (+ forwardTenorYears, adjusted following)
// - maturity = effective (+ swapTenorYears, adjusted following)
func SpotEffectiveMaturity(tradeDate time.Time, cal calendar.CalendarID, forwardTenorYears, swapTenorYears int) (spot, effective, maturity time.Time) {
	return SpotEffectiveMaturityWithSpotLag(tradeDate, cal, 2, forwardTenorYears, swapTenorYears)
}

// SpotEffectiveMaturityWithSpotLag computes spot (trade + spotLagBD), effective, and maturity dates from a trade date.
//
// Conventions:
// - spot = tradeDate + spotLagBD business days on cal
// - effective = spot (+ forwardTenorYears, adjusted following)
// - maturity = effective (+ swapTenorYears, adjusted following)
func SpotEffectiveMaturityWithSpotLag(tradeDate time.Time, cal calendar.CalendarID, spotLagBD, forwardTenorYears, swapTenorYears int) (spot, effective, maturity time.Time) {
	spot = calendar.AddBusinessDays(cal, tradeDate, spotLagBD)

	if forwardTenorYears > 0 {
		effective = calendar.AdjustFollowing(cal, spot.AddDate(forwardTenorYears, 0, 0))
	} else {
		effective = spot
	}
	maturity = calendar.AdjustFollowing(cal, effective.AddDate(swapTenorYears, 0, 0))
	return spot, effective, maturity
}

// GenerateSchedule builds the payment schedule for a leg.
//
// It returns business-day adjusted StartDate/EndDate/PayDate along with integer accrual days.
// When leg.ScheduleDirection is ScheduleBackward, periods are generated from maturity
// backward (Bloomberg SWPM convention for IBOR swaps), creating a front stub if needed.
func GenerateSchedule(effective, maturity time.Time, leg market.LegConvention) ([]SchedulePeriod, error) {
	if maturity.Before(effective) {
		return nil, fmt.Errorf("GenerateSchedule: maturity %s before effective %s", maturity.Format("2006-01-02"), effective.Format("2006-01-02"))
	}
	if leg.PayFrequency <= 0 {
		return nil, fmt.Errorf("GenerateSchedule: unsupported pay frequency %d", leg.PayFrequency)
	}

	// Use backward generation if specified (Bloomberg SWPM convention for IBOR)
	if leg.ScheduleDirection == market.ScheduleBackward {
		return generateScheduleBackward(effective, maturity, leg)
	}

	// Default: forward generation from effective date
	return generateScheduleForward(effective, maturity, leg)
}

// generateScheduleForward generates periods rolling forward from effective date.
func generateScheduleForward(effective, maturity time.Time, leg market.LegConvention) ([]SchedulePeriod, error) {
	periods := make([]SchedulePeriod, 0, 64)
	months := int(leg.PayFrequency)
	start := effective
	var prevAdjustedEnd time.Time // Track the previous period's adjusted end for chaining

	for {
		var next time.Time
		if leg.RollConvention == market.BackwardEOM {
			next = utils.AddMonth(start, months)
		} else {
			next = start.AddDate(0, months, 0)
		}
		if next.After(maturity.AddDate(0, 0, 1)) {
			break
		}

		// OIS swaps (overnight rates) use chained accrual periods per Bloomberg SWPM convention
		isOIS := market.IsOvernight(leg.ReferenceRate)

		// For OIS, chain from previous period's end; for others, use independent periods
		var accrualStart time.Time
		if isOIS && !prevAdjustedEnd.IsZero() {
			accrualStart = prevAdjustedEnd
		} else {
			accrualStart = calendar.Adjust(leg.Calendar, start)
		}
		accrualEnd := calendar.Adjust(leg.Calendar, next)

		// For OIS with PayDelayDays=0 (Bloomberg SWPM convention),
		// the payment date IS the accrual end date
		var paymentDate time.Time
		if isOIS && leg.PayDelayDays == 0 {
			paymentDate = accrualEnd
			// No need to adjust accrualEnd, it's already the payment date
		} else if isOIS {
			// If there are payment delays, adjust the end date accordingly
			paymentDate = calendar.AddBusinessDays(leg.Calendar, accrualEnd, leg.PayDelayDays)
			accrualEnd = paymentDate // Use payment date as accrual end for OIS
		} else {
			// Standard convention (IBOR/Fixed): payment date is after accrual end
			paymentDate = calendar.AddBusinessDays(leg.Calendar, accrualEnd, leg.PayDelayDays)
		}

		fixingDate := calendar.AddBusinessDays(leg.Calendar, accrualStart, -leg.FixingLagDays)
		if leg.ResetPosition == market.ResetInArrears {
			fixingDate = calendar.AddBusinessDays(leg.Calendar, accrualEnd, -(leg.RateCutoffDays + leg.FixingLagDays))
		}

		periods = append(periods, SchedulePeriod{
			StartDate:   accrualStart,
			EndDate:     accrualEnd,
			PayDate:     paymentDate,
			AccrualDays: int(utils.Days(accrualStart, accrualEnd)),
			FixingDate:  fixingDate,
		})

		// Save the adjusted end for chaining (if OIS)
		if isOIS {
			prevAdjustedEnd = accrualEnd
		}

		// Always use the unadjusted date for the next iteration to avoid drift
		start = next
	}

	return periods, nil
}

// generateScheduleBackward generates periods rolling backward from maturity date.
// This matches Bloomberg SWPM convention for IBOR swaps, where intermediate dates
// align with maturity and the first period becomes a front stub if needed.
func generateScheduleBackward(effective, maturity time.Time, leg market.LegConvention) ([]SchedulePeriod, error) {
	months := int(leg.PayFrequency)

	// Generate unadjusted dates backward from maturity
	// Stop when we reach or pass effective date
	var unadjustedDates []time.Time
	current := maturity
	for current.After(effective) {
		unadjustedDates = append([]time.Time{current}, unadjustedDates...)
		if leg.RollConvention == market.BackwardEOM {
			current = utils.AddMonth(current, -months)
		} else {
			current = current.AddDate(0, -months, 0)
		}
	}

	// If the first backward-rolled date is very close to effective (within 7 days),
	// skip it to avoid creating a tiny stub period (Bloomberg convention).
	// The first period will be a long stub from effective to the next regular date.
	if len(unadjustedDates) > 0 {
		firstDate := unadjustedDates[0]
		daysDiff := int(utils.Days(effective, firstDate))
		if daysDiff > 0 && daysDiff <= 7 {
			// Skip the first backward-rolled date (it's too close to effective)
			unadjustedDates = unadjustedDates[1:]
		}
	}

	// Prepend effective date as the start of the first (potentially stub) period
	unadjustedDates = append([]time.Time{effective}, unadjustedDates...)

	// Build periods from consecutive date pairs
	periods := make([]SchedulePeriod, 0, len(unadjustedDates)-1)
	for i := 0; i < len(unadjustedDates)-1; i++ {
		startUnadj := unadjustedDates[i]
		endUnadj := unadjustedDates[i+1]

		accrualStart := calendar.Adjust(leg.Calendar, startUnadj)
		accrualEnd := calendar.Adjust(leg.Calendar, endUnadj)

		paymentDate := calendar.AddBusinessDays(leg.Calendar, accrualEnd, leg.PayDelayDays)

		fixingDate := calendar.AddBusinessDays(leg.Calendar, accrualStart, -leg.FixingLagDays)
		if leg.ResetPosition == market.ResetInArrears {
			fixingDate = calendar.AddBusinessDays(leg.Calendar, accrualEnd, -(leg.RateCutoffDays + leg.FixingLagDays))
		}

		periods = append(periods, SchedulePeriod{
			StartDate:   accrualStart,
			EndDate:     accrualEnd,
			PayDate:     paymentDate,
			AccrualDays: int(utils.Days(accrualStart, accrualEnd)),
			FixingDate:  fixingDate,
		})
	}

	return periods, nil
}

// GetDiscountFactors returns discount factors for the given dates using the curve's interpolation rules.
func GetDiscountFactors(curve DiscountCurve, dates []time.Time) ([]float64, error) {
	if isNilInterface(curve) {
		return nil, ErrNilCurve
	}
	dfs := make([]float64, len(dates))
	for i, d := range dates {
		dfs[i] = curve.DF(d)
	}
	return dfs, nil
}

// GetZeroRates returns continuously-compounded zero rates (in percent) for the given dates.
func GetZeroRates(curve DiscountCurve, dates []time.Time) ([]float64, error) {
	if isNilInterface(curve) {
		return nil, ErrNilCurve
	}
	zeros := make([]float64, len(dates))
	for i, d := range dates {
		zeros[i] = curve.ZeroRateAt(d)
	}
	return zeros, nil
}

func forwardRate(projCurve ProjectionCurve, start, end time.Time, dayCount string) float64 {
	dfStart := projCurve.DF(start)
	dfEnd := projCurve.DF(end)
	alpha := utils.YearFraction(start, end, dayCount)
	if alpha == 0 {
		return 0
	}
	return (dfStart/dfEnd - 1.0) / alpha
}

// GetForwardRates returns simple forward rates for each schedule period of a floating leg.
//
// Rate is returned as a decimal (e.g., 0.025 == 2.5%).
func GetForwardRates(projCurve ProjectionCurve, effective, maturity time.Time, leg market.LegConvention) ([]ForwardRate, error) {
	if isNilInterface(projCurve) {
		return nil, ErrNilCurve
	}
	if leg.LegType != market.LegFloating {
		return nil, fmt.Errorf("GetForwardRates: leg must be floating, got %s", leg.LegType)
	}

	periods, err := GenerateSchedule(effective, maturity, leg)
	if err != nil {
		return nil, err
	}

	out := make([]ForwardRate, 0, len(periods))
	for _, p := range periods {
		r := forwardRate(projCurve, p.StartDate, p.EndDate, string(leg.DayCount))
		out = append(out, ForwardRate{
			FixingDate: p.FixingDate,
			StartDate:  p.StartDate,
			EndDate:    p.EndDate,
			Rate:       r,
		})
	}
	return out, nil
}

func validateSwapSpec(spec market.SwapSpec) error {
	if spec.MaturityDate.Before(spec.EffectiveDate) {
		return fmt.Errorf("maturity %s before effective %s", spec.MaturityDate.Format("2006-01-02"), spec.EffectiveDate.Format("2006-01-02"))
	}
	if spec.PayLeg.PayFrequency <= 0 || spec.RecLeg.PayFrequency <= 0 {
		return fmt.Errorf("unsupported pay frequency (pay=%d, rec=%d)", spec.PayLeg.PayFrequency, spec.RecLeg.PayFrequency)
	}
	return nil
}

func legPV(
	spec market.SwapSpec,
	leg market.LegConvention,
	projCurve ProjectionCurve,
	discCurve DiscountCurve,
	valuationDate time.Time,
	spreadBP float64,
	isPayLeg bool,
) (float64, error) {
	if isNilInterface(discCurve) {
		return 0, ErrNilCurve
	}
	if leg.LegType == market.LegFloating && isNilInterface(projCurve) {
		return 0, ErrNilCurve
	}

	periods, err := GenerateSchedule(spec.EffectiveDate, spec.MaturityDate, leg)
	if err != nil {
		return 0, err
	}

	spread := spreadBP * 1e-4

	signCoupon := 1.0
	if isPayLeg {
		signCoupon = -1.0
	}

	totalPV := 0.0
	for _, p := range periods {
		if p.PayDate.Before(valuationDate) {
			continue
		}

		accrual := utils.YearFraction(p.StartDate, p.EndDate, string(leg.DayCount))

		base := 0.0
		if leg.LegType == market.LegFloating {
			base = forwardRate(projCurve, p.StartDate, p.EndDate, string(leg.DayCount))
		}
		rate := base + spread

		payment := spec.Notional * accrual * rate
		df := discCurve.DF(p.PayDate)
		totalPV += signCoupon * payment * df
	}

	if leg.IncludeInitialPrincipal && !spec.EffectiveDate.Before(valuationDate) {
		sign := -1.0
		if isPayLeg {
			sign = 1.0
		}
		totalPV += sign * spec.Notional * discCurve.DF(spec.EffectiveDate)
	}
	if leg.IncludeFinalPrincipal && !spec.MaturityDate.Before(valuationDate) {
		sign := 1.0
		if isPayLeg {
			sign = -1.0
		}
		totalPV += sign * spec.Notional * discCurve.DF(spec.MaturityDate)
	}

	return totalPV, nil
}

// NPV calculates the net present value of a swap by summing discounted cashflows across both legs.
func NPV(spec market.SwapSpec, projPay ProjectionCurve, projRec ProjectionCurve, discCurve DiscountCurve, valuationDate time.Time) (float64, error) {
	if err := validateSwapSpec(spec); err != nil {
		return 0, fmt.Errorf("NPV: %w", err)
	}
	if isNilInterface(discCurve) {
		return 0, ErrNilCurve
	}

	pvPay, err := legPV(spec, spec.PayLeg, projPay, discCurve, valuationDate, spec.PayLegSpreadBP, true)
	if err != nil {
		return 0, fmt.Errorf("NPV: pay leg: %w", err)
	}
	pvRec, err := legPV(spec, spec.RecLeg, projRec, discCurve, valuationDate, spec.RecLegSpreadBP, false)
	if err != nil {
		return 0, fmt.Errorf("NPV: receive leg: %w", err)
	}

	return pvPay + pvRec, nil
}

// PVByLeg calculates discounted PVs for each leg and returns the net sum.
func PVByLeg(spec market.SwapSpec, projPay ProjectionCurve, projRec ProjectionCurve, discCurve DiscountCurve, valuationDate time.Time) (PV, error) {
	if err := validateSwapSpec(spec); err != nil {
		return PV{}, fmt.Errorf("PVByLeg: %w", err)
	}
	if isNilInterface(discCurve) {
		return PV{}, ErrNilCurve
	}

	pvPay, err := legPV(spec, spec.PayLeg, projPay, discCurve, valuationDate, spec.PayLegSpreadBP, true)
	if err != nil {
		return PV{}, fmt.Errorf("PVByLeg: pay leg: %w", err)
	}
	pvRec, err := legPV(spec, spec.RecLeg, projRec, discCurve, valuationDate, spec.RecLegSpreadBP, false)
	if err != nil {
		return PV{}, fmt.Errorf("PVByLeg: receive leg: %w", err)
	}
	return PV{
		PayLegPV: pvPay,
		RecLegPV: pvRec,
		TotalPV:  pvPay + pvRec,
	}, nil
}

func pv01TargetLegPerDec(spec market.SwapSpec, discCurve DiscountCurve, valuationDate time.Time, target SpreadTarget) (float64, error) {
	if isNilInterface(discCurve) {
		return 0, ErrNilCurve
	}

	var (
		leg  market.LegConvention
		sign float64
	)
	switch target {
	case SpreadTargetPayLeg:
		leg = spec.PayLeg
		sign = -1.0
	case SpreadTargetRecLeg:
		leg = spec.RecLeg
		sign = 1.0
	default:
		return 0, fmt.Errorf("pv01TargetLegPerDec: unknown target %d", target)
	}

	periods, err := GenerateSchedule(spec.EffectiveDate, spec.MaturityDate, leg)
	if err != nil {
		return 0, err
	}

	pv01 := 0.0
	for _, p := range periods {
		if p.PayDate.Before(valuationDate) {
			continue
		}
		accrual := utils.YearFraction(p.StartDate, p.EndDate, string(leg.DayCount))
		pv01 += sign * spec.Notional * accrual * discCurve.DF(p.PayDate)
	}
	return pv01, nil
}

// SolveParSpread finds the spread (in bp) on the target leg such that swap NPV equals 0.
//
// It uses Newton-Raphson with an analytically computed PV01 (the objective is linear in spread),
// so it typically converges in a single iteration.
func SolveParSpread(spec market.SwapSpec, projPay ProjectionCurve, projRec ProjectionCurve, discCurve DiscountCurve, valuationDate time.Time, target SpreadTarget) (float64, error) {
	if err := validateSwapSpec(spec); err != nil {
		return 0, fmt.Errorf("SolveParSpread: %w", err)
	}
	if isNilInterface(discCurve) {
		return 0, ErrNilCurve
	}

	pv01Dec, err := pv01TargetLegPerDec(spec, discCurve, valuationDate, target)
	if err != nil {
		return 0, err
	}
	pv01PerBP := pv01Dec * 1e-4
	if pv01PerBP == 0 {
		return 0, fmt.Errorf("SolveParSpread: PV01 is zero for target leg")
	}

	spreadBP := spec.RecLegSpreadBP
	if target == SpreadTargetPayLeg {
		spreadBP = spec.PayLegSpreadBP
	}

	tolPV := 1e-10 * math.Max(1.0, math.Abs(spec.Notional))
	maxIter := 10

	for i := 0; i < maxIter; i++ {
		tmp := spec
		switch target {
		case SpreadTargetPayLeg:
			tmp.PayLegSpreadBP = spreadBP
		case SpreadTargetRecLeg:
			tmp.RecLegSpreadBP = spreadBP
		default:
			return 0, fmt.Errorf("SolveParSpread: unknown target %d", target)
		}

		npv, err := NPV(tmp, projPay, projRec, discCurve, valuationDate)
		if err != nil {
			return 0, err
		}
		if math.Abs(npv) <= tolPV {
			return spreadBP, nil
		}

		spreadBP = spreadBP - npv/pv01PerBP
	}

	tmp := spec
	if target == SpreadTargetPayLeg {
		tmp.PayLegSpreadBP = spreadBP
	} else {
		tmp.RecLegSpreadBP = spreadBP
	}
	npv, _ := NPV(tmp, projPay, projRec, discCurve, valuationDate)
	return spreadBP, fmt.Errorf("SolveParSpread: did not converge (spread=%.12f bp, npv=%.6g)", spreadBP, npv)
}

// ComputeOISParRateWithDiscount computes the par swap rate (in decimal) for an OIS leg
// using a separate projection curve and discount curve.
// Par rate = sum(fwd_proj * accrual * df_disc) / sum(accrual * df_disc)
func ComputeOISParRateWithDiscount(spec market.SwapSpec, projCurve, discCurve DiscountCurve, valuationDate time.Time, leg market.LegConvention) (float64, error) {
	if isNilInterface(projCurve) || isNilInterface(discCurve) {
		return 0, ErrNilCurve
	}

	periods, err := GenerateSchedule(spec.EffectiveDate, spec.MaturityDate, leg)
	if err != nil {
		return 0, err
	}

	floatLegPV := 0.0
	annuity := 0.0

	for _, p := range periods {
		if p.PayDate.Before(valuationDate) {
			continue
		}
		accrual := utils.YearFraction(p.StartDate, p.EndDate, string(leg.DayCount))
		df := discCurve.DF(p.PayDate)
		fwd := forwardRate(projCurve, p.StartDate, p.EndDate, string(leg.DayCount))
		floatLegPV += fwd * accrual * df
		annuity += accrual * df
	}

	if annuity == 0 {
		return 0, fmt.Errorf("ComputeOISParRateWithDiscount: annuity is zero")
	}

	return floatLegPV / annuity, nil
}

// SolveOISBasisSpread computes the basis spread (in bp) between two OIS curves.
// This is the difference in par swap rates: payLegCurve par rate - recLegCurve par rate.
// Both par rates are computed using the same discount curve (discCurve).
// Used for OIS basis swaps where both legs reference the same overnight index
// but from different venues (e.g., LCHS vs JSCC TONAR).
func SolveOISBasisSpread(spec market.SwapSpec, payProjCurve, recProjCurve, discCurve DiscountCurve, valuationDate time.Time) (float64, error) {
	// Pay leg par rate: projection from payProjCurve, discount from discCurve
	payParRate, err := ComputeOISParRateWithDiscount(spec, payProjCurve, discCurve, valuationDate, spec.PayLeg)
	if err != nil {
		return 0, fmt.Errorf("SolveOISBasisSpread: pay leg: %w", err)
	}

	// Rec leg par rate: projection from recProjCurve, discount from discCurve
	recParRate, err := ComputeOISParRateWithDiscount(spec, recProjCurve, discCurve, valuationDate, spec.RecLeg)
	if err != nil {
		return 0, fmt.Errorf("SolveOISBasisSpread: rec leg: %w", err)
	}

	// Basis = pay leg par rate - rec leg par rate, in bp
	basisBP := (payParRate - recParRate) * 10000
	return basisBP, nil
}
