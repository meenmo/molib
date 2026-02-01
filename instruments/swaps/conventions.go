package swaps

import (
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
)

// BasisPreset groups pay, receive, and discounting leg conventions
// for common basis swap structures (e.g., EUR 3M/6M vs ESTR, JPY TIBOR vs TONAR).
type BasisPreset struct {
	PayLeg      market.LegConvention
	RecLeg      market.LegConvention
	DiscountOIS market.LegConvention
}

// IRSPreset groups fixed, floating, and discounting leg conventions
// for a vanilla fixed-vs-floating IRS (e.g., EUR fixed vs EURIBOR3M, disc. ESTR).
type IRSPreset struct {
	FixedLeg    market.LegConvention
	FloatLeg    market.LegConvention
	DiscountOIS market.LegConvention
}

// OISPreset groups fixed and overnight leg conventions for an OIS swap.
// Discounting is typically on the overnight curve itself.
type OISPreset struct {
	FixedLeg market.LegConvention
	FloatLeg market.LegConvention
}

// Preset leg conventions for EUR and JPY.
var (
	SOFRFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          2,
		BusinessDayAdjustment: market.ModifiedFollowing,
		Calendar:              calendar.FD,
	}

	SOFRFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.SOFR,
		DayCount:                market.Act360,
		ResetFrequency:          market.FreqDaily,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            2,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.Backward,
		Calendar:                calendar.FD,
		FixingCalendar:          calendar.GT,
		ResetPosition:           market.ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	ESTRFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          1,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.TARGET,
		ScheduleDirection:     market.ScheduleBackward,
	}

	ESTRFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.ESTR,
		DayCount:                market.Act360,
		ResetFrequency:          market.FreqDaily,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            1,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           market.ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
		ScheduleDirection:       market.ScheduleBackward,
	}

	EURIBORFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          0,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.TARGET,
	}

	EURIBOR3MFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.EURIBOR3M,
		DayCount:                market.Act360,
		ResetFrequency:          market.FreqQuarterly,
		PayFrequency:            market.FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
		ScheduleDirection:       market.ScheduleBackward,
	}

	EURIBOR6MFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.EURIBOR6M,
		DayCount:                market.Act360,
		ResetFrequency:          market.FreqSemi,
		PayFrequency:            market.FreqSemi,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
		ScheduleDirection:       market.ScheduleBackward,
	}

	TONARFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act365F,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          2,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.JP,
	}

	TONARFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.TONAR,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqDaily,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JP,
		ResetPosition:           market.ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBORFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act365F,
		PayFrequency:          market.FreqSemi,
		FixingLagDays:         0,
		PayDelayDays:          0,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.JP,
	}

	TIBOR3MFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.TIBOR3M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqQuarterly,
		PayFrequency:            market.FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            2,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JP,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBOR6MFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqSemi,
		PayFrequency:            market.FreqSemi,
		FixingLagDays:           2,
		PayDelayDays:            2,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JP,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	KRXCD91DFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act365F,
		PayFrequency:          market.FreqQuarterly,
		FixingLagDays:         0,
		PayDelayDays:          0,
		BusinessDayAdjustment: market.ModifiedFollowing,
		Calendar:              calendar.KR,
	}

	KRXCD91DFloating = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceIndex:          market.CD91D,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqQuarterly,
		PayFrequency:            market.FreqQuarterly,
		FixingLagDays:           0,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.KR,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}
)

// Preset basis structures for common EUR and JPY basis trades.
var (
	// EUR IRS-style basis: pay EURIBOR 6M, receive EURIBOR 3M, discount on ESTR OIS.
	// Naming omits redundant currency prefixes: the EUR nature is clear
	// from the EURIBOR / ESTR indices themselves.
	BasisEURIBOR3M6MESTR = BasisPreset{
		PayLeg:      EURIBOR6MFloating,
		RecLeg:      EURIBOR3MFloating,
		DiscountOIS: ESTRFloating,
	}

	// JPY basis: pay TIBOR 6M, receive TIBOR 3M, discount on TONAR OIS.
	// Likewise, the currency is implied by the TIBOR / TONAR indices,
	// so the name focuses only on the indices.
	BasisTIBOR3M6MTONAR = BasisPreset{
		PayLeg:      TIBOR6MFloating,
		RecLeg:      TIBOR3MFloating,
		DiscountOIS: TONARFloating,
	}

	// EUR IRS: fixed vs EURIBOR 3M, discounted on ESTR OIS.
	IRSEURIBOR3MESTR = IRSPreset{
		FixedLeg:    EURIBORFixed,
		FloatLeg:    EURIBOR3MFloating,
		DiscountOIS: ESTRFloating,
	}

	// EUR IRS: fixed vs EURIBOR 6M, discounted on ESTR OIS.
	IRSEURIBOR6MESTR = IRSPreset{
		FixedLeg:    EURIBORFixed,
		FloatLeg:    EURIBOR6MFloating,
		DiscountOIS: ESTRFloating,
	}

	// JPY IRS: fixed vs TIBOR 3M, discounted on TONAR OIS.
	IRSTIBOR3MTONAR = IRSPreset{
		FixedLeg:    TIBORFixed,
		FloatLeg:    TIBOR3MFloating,
		DiscountOIS: TONARFloating,
	}

	// JPY IRS: fixed vs TIBOR 6M, discounted on TONAR OIS.
	IRSTIBOR6MTONAR = IRSPreset{
		FixedLeg:    TIBORFixed,
		FloatLeg:    TIBOR6MFloating,
		DiscountOIS: TONARFloating,
	}

	// EUR OIS: fixed vs ESTR.
	OISESTR = OISPreset{
		FixedLeg: ESTRFixed,
		FloatLeg: ESTRFloating,
	}

	// JPY OIS: fixed vs TONAR.
	OISTONAR = OISPreset{
		FixedLeg: TONARFixed,
		FloatLeg: TONARFloating,
	}
)
