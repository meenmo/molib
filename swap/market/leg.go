package market

import (
	"time"

	"github.com/meenmo/molib/calendar"
)

// LegType distinguishes floating vs fixed.
type LegType string

const (
	LegFloating LegType = "FLOATING"
	LegFixed    LegType = "FIXED"
)

// Frequency enumerates payment/reset frequencies in months.
type Frequency int

const (
	FreqAnnual    Frequency = 12
	FreqSemi      Frequency = 6
	FreqQuarterly Frequency = 3
	FreqMonthly   Frequency = 1
	FreqDaily     Frequency = 0
)

// BusinessDayAdjustment roll convention.
type BusinessDayAdjustment string

const (
	ModifiedFollowing BusinessDayAdjustment = "MODIFIED_FOLLOWING"
)

// RollConvention for month-end handling.
type RollConvention string

const (
	BackwardEOM RollConvention = "BACKWARD_EOM"
)

// ResetPosition indicates fixing timing.
type ResetPosition string

const (
	ResetInAdvance ResetPosition = "IN_ADVANCE"
	ResetInArrears ResetPosition = "IN_ARREARS"
)

// DayCount enum.
type DayCount string

const (
	Act360  DayCount = "ACT/360"
	Act365  DayCount = "ACT/365"
	Act365F DayCount = "ACT/365F"
	Dc30360 DayCount = "30/360"
)

// LegConvention captures standard swap leg settings.
type LegConvention struct {
	LegType                 LegType
	ReferenceRate           ReferenceRate
	DayCount                DayCount
	ResetFrequency          Frequency
	PayFrequency            Frequency
	FixingLagDays           int
	PayDelayDays            int
	BusinessDayAdjustment   BusinessDayAdjustment
	RollConvention          RollConvention
	Calendar                calendar.CalendarID
	ResetPosition           ResetPosition
	RateCutoffDays          int
	IncludeInitialPrincipal bool
	IncludeFinalPrincipal   bool
}

// SwapSpec describes a basis swap trade.
type SwapSpec struct {
	Notional       float64
	EffectiveDate  time.Time
	MaturityDate   time.Time
	PayLeg         LegConvention
	RecLeg         LegConvention
	DiscountingOIS LegConvention
	PayLegSpreadBP float64
	RecLegSpreadBP float64
}
