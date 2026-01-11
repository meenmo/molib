package bond

import "time"

// Cashflow is a single dated cash payment for a bond.
//
// Amounts are in currency units (e.g., EUR), not price-per-100.
type Cashflow struct {
	Date      time.Time
	Coupon    float64
	Principal float64
}

func (c Cashflow) Amount() float64 {
	return c.Coupon + c.Principal
}
