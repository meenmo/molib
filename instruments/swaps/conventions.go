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
	ESTRFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.ESTR,
		DayCount:                market.Act365F,
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
	}

	EURIBOR3MFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.EURIBOR3M,
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

	EURIBOR6MFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.EURIBOR6M,
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

	TONARFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TONAR,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqDaily,
		PayFrequency:            market.FreqAnnual,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           market.ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBOR3MFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR3M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqQuarterly,
		PayFrequency:            market.FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            2,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBOR6MFloat = market.LegConvention{
		LegType:                 market.LegFloating,
		ReferenceRate:           market.TIBOR6M,
		DayCount:                market.Act365F,
		ResetFrequency:          market.FreqSemi,
		PayFrequency:            market.FreqSemi,
		FixingLagDays:           2,
		PayDelayDays:            2,
		BusinessDayAdjustment:   market.ModifiedFollowing,
		RollConvention:          market.BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           market.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	// EUR IRS fixed leg: annual payments, ACT/360, TARGET calendar.
	// This mirrors ficclib's EUR_IRS_FIXED convention for OIS.
	EurFixedAnnual = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          1,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.TARGET,
	}

	// EUR IBOR IRS fixed leg: annual payments, 30/360, TARGET calendar.
	// Used for EURIBOR swaps (pre-2020 IBOR discounting) where fixed leg uses 30/360.
	Euribor6MFixed = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Dc30360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          2,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.TARGET,
		ScheduleDirection:     market.ScheduleBackward,
	}

	// JPY IRS fixed leg: semiannual payments, ACT/365F, JPN calendar.
	// This is the natural fixed leg for plain JPY TIBOR IRS examples.
	JpyFixedSemi = market.LegConvention{
		LegType:               market.LegFixed,
		DayCount:              market.Act365F,
		PayFrequency:          market.FreqSemi,
		FixingLagDays:         0,
		PayDelayDays:          0,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.JPN,
	}

	// EUR OIS fixed leg: annual payments, ACT/360, TARGET calendar.
	// Mirrors ficclib's ESTR_FIXED convention.
	EstrFixedAnnual = market.LegConvention{
		LegType:               market.LegFixed,
		ReferenceRate:         market.ESTR,
		DayCount:              market.Act360,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          1,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.TARGET,
	}

	// JPY OIS fixed leg: annual payments, ACT/365F, JPN calendar.
	// Mirrors ficclib's TONAR_FIXED convention.
	TonarFixedAnnual = market.LegConvention{
		LegType:               market.LegFixed,
		ReferenceRate:         market.TONAR,
		DayCount:              market.Act365F,
		PayFrequency:          market.FreqAnnual,
		FixingLagDays:         0,
		PayDelayDays:          2,
		BusinessDayAdjustment: market.ModifiedFollowing,
		RollConvention:        market.BackwardEOM,
		Calendar:              calendar.JPN,
	}
)

// Preset basis structures for common EUR and JPY basis trades.
var (
	// EUR IRS-style basis: pay EURIBOR 6M, receive EURIBOR 3M, discount on ESTR OIS.
	// Naming omits redundant currency prefixes: the EUR nature is clear
	// from the EURIBOR / ESTR indices themselves.
	BasisEuribor3M6MEstr = BasisPreset{
		PayLeg:      EURIBOR6MFloat,
		RecLeg:      EURIBOR3MFloat,
		DiscountOIS: ESTRFloat,
	}

	// JPY basis: pay TIBOR 6M, receive TIBOR 3M, discount on TONAR OIS.
	// Likewise, the currency is implied by the TIBOR / TONAR indices,
	// so the name focuses only on the indices.
	BasisTibor3M6MTonar = BasisPreset{
		PayLeg:      TIBOR6MFloat,
		RecLeg:      TIBOR3MFloat,
		DiscountOIS: TONARFloat,
	}

	// EUR IRS: fixed vs EURIBOR 3M, discounted on ESTR OIS.
	IrsEuribor3MEstr = IRSPreset{
		FixedLeg:    EurFixedAnnual,
		FloatLeg:    EURIBOR3MFloat,
		DiscountOIS: ESTRFloat,
	}

	// EUR IRS: fixed vs EURIBOR 6M, discounted on ESTR OIS.
	IrsEuribor6MEstr = IRSPreset{
		FixedLeg:    EurFixedAnnual,
		FloatLeg:    EURIBOR6MFloat,
		DiscountOIS: ESTRFloat,
	}

	// JPY IRS: fixed vs TIBOR 3M, discounted on TONAR OIS.
	IrsTibor3MTonar = IRSPreset{
		FixedLeg:    JpyFixedSemi,
		FloatLeg:    TIBOR3MFloat,
		DiscountOIS: TONARFloat,
	}

	// JPY IRS: fixed vs TIBOR 6M, discounted on TONAR OIS.
	IrsTibor6MTonar = IRSPreset{
		FixedLeg:    JpyFixedSemi,
		FloatLeg:    TIBOR6MFloat,
		DiscountOIS: TONARFloat,
	}

	// EUR OIS: fixed vs ESTR.
	OisEstr = OISPreset{
		FixedLeg: EstrFixedAnnual,
		FloatLeg: ESTRFloat,
	}

	// JPY OIS: fixed vs TONAR.
	OisTonar = OISPreset{
		FixedLeg: TonarFixedAnnual,
		FloatLeg: TONARFloat,
	}
)
