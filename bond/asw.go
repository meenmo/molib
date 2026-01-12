package bond

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
	"github.com/meenmo/molib/utils"
)

// ASWType specifies the asset swap spread calculation method.
type ASWType string

const (
	// ASWTypeParPar uses par notional for PV01 calculation (Par-Par ASW Spread).
	ASWTypeParPar ASWType = "PAR-PAR"
	// ASWTypeMMS uses dirty price as notional for PV01 (Matched-Maturity ASW Spread).
	ASWTypeMMS ASWType = "MMS"
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

	// ASWType selects the spread calculation method.
	// "PAR-PAR" (default): PV01 uses par notional.
	// "mms": PV01 uses dirty price as notional (Matched-Maturity Spread).
	ASWType ASWType
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

	// Compute annuity factor (sum of discounted accruals).
	annuityFactor := 0.0
	for _, p := range periods {
		if p.PayDate.Before(in.SettlementDate) {
			continue
		}
		accrual := utils.YearFraction(p.StartDate, p.EndDate, string(in.FloatLeg.DayCount))
		annuityFactor += accrual * in.DiscountCurve.DF(p.PayDate)
	}
	if annuityFactor == 0 {
		return ASWResult{}, fmt.Errorf("ComputeASWSpread: annuity factor is zero")
	}

	// Select notional based on ASW type.
	// Par-Par (default): uses par notional.
	// MMS: uses dirty price as notional.
	notionalForPV01 := in.Notional
	if in.ASWType == ASWTypeMMS {
		notionalForPV01 = in.DirtyPrice
	}

	pv01 := notionalForPV01 * annuityFactor * 1e-4
	spreadBP := (pvBondRF - in.DirtyPrice) / pv01

	return ASWResult{
		SpreadBP: spreadBP,
		PVBondRF: pvBondRF,
		PV01:     pv01,
	}, nil
}
