package swap

import "github.com/meenmo/molib/calendar"

// DayCountConvention holds day count conventions for different leg types.
type DayCountConvention struct {
	// OIS is the day count for overnight index swaps (e.g., TONAR, ESTR, SOFR).
	OIS string

	// FloatIBOR is the day count for floating IBOR legs (e.g., EURIBOR, TIBOR).
	FloatIBOR string

	// FixedIBOR is the day count for fixed legs in IBOR swaps.
	FixedIBOR string

	// FixedFreqMonths is the payment frequency in months for fixed legs.
	FixedFreqMonths int
}

// GetDayCountConvention returns the day count conventions for a given calendar.
// These match Bloomberg SWPM and standard market conventions.
func GetDayCountConvention(cal calendar.CalendarID) DayCountConvention {
	switch cal {
	case calendar.TARGET:
		// EUR conventions:
		// - OIS (ESTR): ACT/360
		// - IBOR floating: ACT/360
		// - IBOR fixed: 30E/360, annual
		return DayCountConvention{
			OIS:             "ACT/360",
			FloatIBOR:       "ACT/360",
			FixedIBOR:       "30E/360",
			FixedFreqMonths: 12,
		}

	case calendar.JP:
		// JPY conventions:
		// - OIS (TONAR): ACT/365F
		// - IBOR floating: ACT/365F
		// - IBOR fixed: ACT/365F, semi-annual
		return DayCountConvention{
			OIS:             "ACT/365F",
			FloatIBOR:       "ACT/365F",
			FixedIBOR:       "ACT/365F",
			FixedFreqMonths: 6,
		}

	case calendar.KR:
		// KRW conventions:
		// - OIS: ACT/365F
		// - IBOR (CD): ACT/365F
		// - Fixed: ACT/365F, quarterly
		return DayCountConvention{
			OIS:             "ACT/365F",
			FloatIBOR:       "ACT/365F",
			FixedIBOR:       "ACT/365F",
			FixedFreqMonths: 3,
		}

	default:
		// Default to JPY-like conventions
		return DayCountConvention{
			OIS:             "ACT/365F",
			FloatIBOR:       "ACT/365F",
			FixedIBOR:       "ACT/365F",
			FixedFreqMonths: 6,
		}
	}
}
