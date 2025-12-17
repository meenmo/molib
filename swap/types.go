package swap

import (
	"errors"
	"time"
)

var (
	// ErrNilCurve is returned when a required curve argument is nil.
	ErrNilCurve = errors.New("nil curve")
)

// DiscountCurve provides discount factors and zero rates for valuation.
type DiscountCurve interface {
	DF(t time.Time) float64
	ZeroRateAt(t time.Time) float64
}

// ProjectionCurve provides discount factors used to infer forward rates.
type ProjectionCurve interface {
	DF(t time.Time) float64
}

// SpreadTarget selects which leg's spread is solved for in SolveParSpread.
type SpreadTarget int

const (
	// SpreadTargetPayLeg solves for spec.PayLegSpreadBP.
	SpreadTargetPayLeg SpreadTarget = iota
	// SpreadTargetRecLeg solves for spec.RecLegSpreadBP.
	SpreadTargetRecLeg
)

// SchedulePeriod is a cashflow period for a single leg.
//
// Dates are business-day adjusted per the provided leg convention.
type SchedulePeriod struct {
	StartDate   time.Time
	EndDate     time.Time
	PayDate     time.Time
	AccrualDays int
	FixingDate  time.Time
}

// ForwardRate is a simple forward rate over an accrual period, associated with its fixing date.
//
// Rate is returned as a decimal (e.g., 0.025 == 2.5%).
type ForwardRate struct {
	FixingDate time.Time
	StartDate  time.Time
	EndDate    time.Time
	Rate       float64
}

// PV contains present values for each leg and the net sum.
type PV struct {
	PayLegPV float64
	RecLegPV float64
	TotalPV  float64
}
