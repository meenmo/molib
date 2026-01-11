package bonds

import (
	"time"

	"github.com/meenmo/molib/bond"
)

// CashflowCents mirrors the Bloomberg-style cashflow feed where coupon/principal
// are stored as integer minor units (e.g., cents for EUR).
type CashflowCents struct {
	Date           time.Time
	CouponCents    int64
	PrincipalCents int64
}

func (c CashflowCents) ToCashflow() bond.Cashflow {
	return bond.Cashflow{
		Date:      c.Date,
		Coupon:    float64(c.CouponCents) / 100.0,
		Principal: float64(c.PrincipalCents) / 100.0,
	}
}

func ToCashflows(in []CashflowCents) []bond.Cashflow {
	out := make([]bond.Cashflow, 0, len(in))
	for _, cf := range in {
		out = append(out, cf.ToCashflow())
	}
	return out
}
