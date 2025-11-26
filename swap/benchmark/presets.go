package benchmark

import "github.com/meenmo/molib/calendar"

// Preset leg conventions for EUR and JPY.
var (
	ESTRFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           ESTR,
		DayCount:                Act365F,
		ResetFrequency:          FreqDaily,
		PayFrequency:            FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            1,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	EURIBOR3MFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           EURIBOR3M,
		DayCount:                Act360,
		ResetFrequency:          FreqQuarterly,
		PayFrequency:            FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	EURIBOR6MFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           EURIBOR6M,
		DayCount:                Act360,
		ResetFrequency:          FreqSemi,
		PayFrequency:            FreqSemi,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.TARGET,
		ResetPosition:           ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TONARFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           TONAR,
		DayCount:                Act365F,
		ResetFrequency:          FreqDaily,
		PayFrequency:            FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            2,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBOR3MFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           TIBOR3M,
		DayCount:                Act365F,
		ResetFrequency:          FreqQuarterly,
		PayFrequency:            FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	TIBOR6MFloat = LegConvention{
		LegType:                 LegFloating,
		ReferenceRate:           TIBOR6M,
		DayCount:                Act365F,
		ResetFrequency:          FreqSemi,
		PayFrequency:            FreqSemi,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   ModifiedFollowing,
		RollConvention:          BackwardEOM,
		Calendar:                calendar.JPN,
		ResetPosition:           ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}
)
