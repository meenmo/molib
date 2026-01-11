package bond

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
	"github.com/meenmo/molib/utils"
)

type ASWInput struct {
	SettlementDate time.Time
	DirtyPrice     float64
	Notional       float64
	Cashflows      []Cashflow

	// FloatLeg is the floating leg convention used for PV01.
	// It defines what the spread is "over" (e.g., EURIBOR6M or ESTR OIS).
	FloatLeg market.LegConvention

	DiscountCurve swap.DiscountCurve
}

type ASWResult struct {
	SpreadBP float64
	PVBondRF float64
	PV01     float64
}

// ComputeASWSpread computes the asset swap spread (in bp) using the approximation:
//
//	ASW ≈ (PV_bond^{rf} - P_dirty) / PV01
//
// where PV01 is the PV of receiving 1bp on the floating leg over the swap schedule.
func ComputeASWSpread(in ASWInput) (ASWResult, error) {
	if in.SettlementDate.IsZero() {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: SettlementDate is required")
	}
	if in.Notional <= 0 {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: Notional must be positive")
	}
	if in.DiscountCurve == nil {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: DiscountCurve is required")
	}
	if len(in.Cashflows) == 0 {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: Cashflows are required")
	}

	maturity := in.SettlementDate
	for _, cf := range in.Cashflows {
		if cf.Date.After(maturity) {
			maturity = cf.Date
		}
	}
	if !maturity.After(in.SettlementDate) {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: maturity (%s) must be after settlement (%s)", maturity.Format("2006-01-02"), in.SettlementDate.Format("2006-01-02"))
	}

	pvBondRF := 0.0
	for _, cf := range in.Cashflows {
		if cf.Date.Before(in.SettlementDate) {
			continue
		}
		pvBondRF += cf.Amount() * in.DiscountCurve.DF(cf.Date)
	}

	periods, err := swap.GenerateSchedule(in.SettlementDate, maturity, in.FloatLeg)
	if err != nil {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: float leg schedule: %w", err)
	}

	pv01 := 0.0
	for _, p := range periods {
		if p.PayDate.Before(in.SettlementDate) {
			continue
		}
		accrual := utils.YearFraction(p.StartDate, p.EndDate, string(in.FloatLeg.DayCount))
		pv01 += in.Notional * accrual * 1e-4 * in.DiscountCurve.DF(p.PayDate)
	}
	if pv01 == 0 {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: PV01 is zero")
	}

	spreadBP := (pvBondRF - in.DirtyPrice) / pv01
	return ASWResult{
		SpreadBP: spreadBP,
		PVBondRF: pvBondRF,
		PV01:     pv01,
	}, nil
}
