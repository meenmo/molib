package swap

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
)

// DataSource identifies the source of market conventions and quotes.
type DataSource string

const (
	// DataSourceBGN represents Bloomberg Generic (BGN) data.
	DataSourceBGN DataSource = "BGN"
	// DataSourceLCH represents London Clearing House (LCH) data.
	DataSourceLCH DataSource = "LCH"
	// DataSourceTradition represents Tradition data.
	DataSourceTradition DataSource = "Tradition"
)

// ClearingHouse identifies where the swap is cleared and determines venue-specific rules.
//
// Note: For exchange-specific products (e.g., KRX), use the dedicated swap/clearinghouse/krx package.
type ClearingHouse string

const (
	// ClearingHouseOTC represents generic over-the-counter (bilateral) swaps.
	ClearingHouseOTC ClearingHouse = "OTC"
	// ClearingHouseLCH represents London Clearing House cleared swaps.
	ClearingHouseLCH ClearingHouse = "LCH"
	// ClearingHouseKRX represents Korea Exchange cleared swaps.
	ClearingHouseKRX ClearingHouse = "KRX"
	// ClearingHouseEUREX represents Eurex cleared swaps.
	ClearingHouseEUREX ClearingHouse = "EUREX"
)

// InterestRateSwapParams defines inputs to construct a generic two-leg interest rate swap trade.
//
// This builder is clearing-house-aware for date conventions (e.g., spot lag), but pricing is driven by:
// - the provided leg conventions (day count, pay delay, calendars, etc.)
// - the provided curve quotes (OIS discounting + optional IBOR projection curves)
type InterestRateSwapParams struct {
	DataSource    DataSource
	ClearingHouse ClearingHouse

	// Dates
	CurveDate     time.Time
	TradeDate     time.Time
	ValuationDate time.Time

	// SpotLagDays overrides the clearing house default (typical OTC is T+2, KRX is T+1).
	// If zero, a clearing house default is used.
	SpotLagDays int

	// Tenors (used when EffectiveDate / MaturityDate are not provided)
	ForwardTenorYears int
	SwapTenorYears    int

	// Optional explicit dates (override ForwardTenorYears / SwapTenorYears if set)
	EffectiveDate time.Time
	MaturityDate  time.Time

	// Economics
	Notional float64

	// Legs and discounting convention
	PayLeg         market.LegConvention
	RecLeg         market.LegConvention
	DiscountingOIS market.LegConvention

	// Quotes used to build curves as-of CurveDate.
	//
	// OISQuotes is required.
	// PayLegQuotes / RecLegQuotes are required only for IBOR floating legs.
	OISQuotes    map[string]float64
	PayLegQuotes map[string]float64
	RecLegQuotes map[string]float64

	// Spreads (in bp). For fixed legs, spread is interpreted as the fixed coupon in bp.
	PayLegSpreadBP float64
	RecLegSpreadBP float64
}

// SwapTrade is a fully specified swap trade paired with valuation curves.
type SwapTrade struct {
	DataSource    DataSource
	ClearingHouse ClearingHouse

	CurveDate     time.Time
	TradeDate     time.Time
	ValuationDate time.Time
	SpotDate      time.Time

	Spec market.SwapSpec

	DiscountCurve DiscountCurve
	PayProjCurve  ProjectionCurve
	RecProjCurve  ProjectionCurve

	// IsOISBasisSwap indicates this is an OIS basis swap where both legs reference
	// the same overnight index but from different venues (e.g., LCHS vs JSCC TONAR).
	// When true, SolveParSpread uses par rate difference instead of cross-curve NPV.
	IsOISBasisSwap bool
}

func defaultSpotLagDays(ch ClearingHouse) int {
	switch ch {
	case ClearingHouseKRX:
		return 1
	default:
		return 2
	}
}

// InterestRateSwap builds curves and constructs a swap trade that can be priced via NPV/SolveParSpread.
//
// For OIS discounting, it builds an OIS curve from params.OISQuotes.
// For floating legs:
// - Overnight indices project off the OIS curve (single-curve).
// - IBOR indices build a dual projection curve bootstrapped using OIS discounting.
func InterestRateSwap(params InterestRateSwapParams) (*SwapTrade, error) {
	if params.CurveDate.IsZero() {
		return nil, fmt.Errorf("InterestRateSwap: CurveDate is required")
	}
	if params.TradeDate.IsZero() {
		return nil, fmt.Errorf("InterestRateSwap: TradeDate is required")
	}
	if params.ValuationDate.IsZero() {
		params.ValuationDate = params.TradeDate
	}
	if params.Notional == 0 {
		return nil, fmt.Errorf("InterestRateSwap: Notional is required")
	}
	if params.OISQuotes == nil {
		return nil, fmt.Errorf("InterestRateSwap: OISQuotes is required")
	}

	spotLag := params.SpotLagDays
	if spotLag == 0 {
		spotLag = defaultSpotLagDays(params.ClearingHouse)
	}

	var spot, effective, maturity time.Time
	if !params.EffectiveDate.IsZero() && !params.MaturityDate.IsZero() {
		spot = params.EffectiveDate
		effective = params.EffectiveDate
		maturity = params.MaturityDate
	} else {
		spot, effective, maturity = SpotEffectiveMaturityWithSpotLag(
			params.TradeDate,
			params.DiscountingOIS.Calendar,
			spotLag,
			params.ForwardTenorYears,
			params.SwapTenorYears,
		)
	}

	// Curve settlement is spot date (curve date + spot lag), not the curve date itself.
	// This matches the standard convention where quotes are for swaps starting at spot.
	curveSettlement := calendar.AddBusinessDays(params.DiscountingOIS.Calendar, params.CurveDate, spotLag)

	disc := curve.BuildCurve(curveSettlement, params.OISQuotes, params.DiscountingOIS.Calendar, 1)
	if disc == nil {
		return nil, fmt.Errorf("InterestRateSwap: failed to build discount curve")
	}

	buildProj := func(leg market.LegConvention, quotes map[string]float64) (ProjectionCurve, error) {
		if leg.LegType != market.LegFloating {
			return nil, nil
		}
		// For all floating legs, quotes must be provided explicitly
		if quotes == nil {
			return nil, fmt.Errorf("missing quotes for %s projection curve", leg.ReferenceRate)
		}

		// For overnight rates (OIS), build curve directly from quotes
		if market.IsOvernight(leg.ReferenceRate) {
			// Build OIS curve for this leg using provided quotes
			// This enables OIS basis swaps (e.g., JSCC TONAR vs LCH TONAR)
			oisCurve := curve.BuildCurve(curveSettlement, quotes, leg.Calendar, 1)
			if oisCurve == nil {
				return nil, fmt.Errorf("failed to build OIS projection curve for %s", leg.ReferenceRate)
			}
			return oisCurve, nil
		}

		// For IBOR rates, build projection curve with forward-discount basis adjustment
		return curve.BuildProjectionCurve(curveSettlement, leg, quotes, disc), nil
	}

	projPay, err := buildProj(params.PayLeg, params.PayLegQuotes)
	if err != nil {
		return nil, fmt.Errorf("InterestRateSwap: pay leg: %w", err)
	}
	projRec, err := buildProj(params.RecLeg, params.RecLegQuotes)
	if err != nil {
		return nil, fmt.Errorf("InterestRateSwap: receive leg: %w", err)
	}

	spec := market.SwapSpec{
		Notional:       params.Notional,
		EffectiveDate:  effective,
		MaturityDate:   maturity,
		PayLeg:         params.PayLeg,
		RecLeg:         params.RecLeg,
		DiscountingOIS: params.DiscountingOIS,
		PayLegSpreadBP: params.PayLegSpreadBP,
		RecLegSpreadBP: params.RecLegSpreadBP,
	}

	// Detect OIS basis swap: both legs are overnight rates with the same reference index
	isOISBasisSwap := market.IsOvernight(params.PayLeg.ReferenceRate) &&
		market.IsOvernight(params.RecLeg.ReferenceRate) &&
		params.PayLeg.ReferenceRate == params.RecLeg.ReferenceRate

	return &SwapTrade{
		DataSource:     params.DataSource,
		ClearingHouse:  params.ClearingHouse,
		CurveDate:      params.CurveDate,
		TradeDate:      params.TradeDate,
		ValuationDate:  params.ValuationDate,
		SpotDate:       spot,
		Spec:           spec,
		DiscountCurve:  disc,
		PayProjCurve:   projPay,
		RecProjCurve:   projRec,
		IsOISBasisSwap: isOISBasisSwap,
	}, nil
}

// NPV returns the swap NPV for the trade's current spreads.
func (t *SwapTrade) NPV() (float64, error) {
	return NPV(t.Spec, t.PayProjCurve, t.RecProjCurve, t.DiscountCurve, t.ValuationDate)
}

// PVByLeg returns leg PVs and net PV for the trade's current spreads.
func (t *SwapTrade) PVByLeg() (PV, error) {
	return PVByLeg(t.Spec, t.PayProjCurve, t.RecProjCurve, t.DiscountCurve, t.ValuationDate)
}

// SolveParSpread solves for the target leg spread (in bp) such that NPV = 0, and updates the trade spec.
//
// For OIS basis swaps (same overnight index, different venues), it computes the difference
// in par swap rates between the two curves instead of using cross-curve NPV optimization.
func (t *SwapTrade) SolveParSpread(target SpreadTarget) (float64, PV, error) {
	var spreadBP float64
	var err error

	if t.IsOISBasisSwap {
		// For OIS basis swaps, compute the difference in par rates between the two curves.
		// Both par rates use the same discount curve (t.DiscountCurve) but different projection curves.
		// The basis is: pay curve par rate - rec curve par rate
		spreadBP, err = SolveOISBasisSpread(t.Spec, t.PayProjCurve.(DiscountCurve), t.RecProjCurve.(DiscountCurve), t.DiscountCurve, t.ValuationDate)
		if err != nil {
			return 0, PV{}, err
		}
	} else {
		spreadBP, err = SolveParSpread(t.Spec, t.PayProjCurve, t.RecProjCurve, t.DiscountCurve, t.ValuationDate, target)
		if err != nil {
			return 0, PV{}, err
		}
	}

	switch target {
	case SpreadTargetPayLeg:
		t.Spec.PayLegSpreadBP = spreadBP
	case SpreadTargetRecLeg:
		t.Spec.RecLegSpreadBP = spreadBP
	default:
		return 0, PV{}, fmt.Errorf("SolveParSpread: unknown target %d", target)
	}

	pv, err := t.PVByLeg()
	if err != nil {
		return 0, PV{}, err
	}
	return spreadBP, pv, nil
}
