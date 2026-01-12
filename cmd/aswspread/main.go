package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/meenmo/molib/bond"
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	krxch "github.com/meenmo/molib/swap/clearinghouse/krx"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
)

type aswFixture struct {
	CurveDate             string `json:"curve_date"`
	CurveType             string `json:"curve_type"`
	CurveFixedLegDayCount string `json:"curve_fixed_leg_day_count"`
	CurveFloatIndex       string `json:"curve_float_index"`
	// FloatingSwapLeg is the swap floating-leg convention preset used for PV01.
	// Example values: "EURIBOR6MFloating", "ESTRFloating", "KRXCD91DFloating".
	FloatingSwapLeg string `json:"floating_swap_leg"`
	// FloatLegConvention is deprecated (kept for backward compatibility).
	FloatLegConvention     string       `json:"float_leg_convention"`
	CurveSettlementLagDays int          `json:"curve_settlement_lag_days"`
	CurveQuotes            []curveQuote `json:"curve_quotes"`
	Bonds                  []bondCase   `json:"bonds"`
	// ASWType selects the spread calculation method: "PAR-PAR" (default) or "MMS".
	ASWType string `json:"asw_type"`
}

type curveQuote struct {
	Tenor string  `json:"tenor"`
	Rate  float64 `json:"rate"`
}

type bondCase struct {
	ISIN           string        `json:"isin"`
	Notional       float64       `json:"notional"`
	BondDirtyPrice json.Number   `json:"bond_dirty_price"`
	Cashflows      []cashflowRow `json:"cashflows"`
}

type cashflowRow struct {
	Date      string `json:"date"`
	Coupon    int64  `json:"coupon"`
	Principal int64  `json:"principal"`
}

type aswOutput struct {
	CurveDate           string  `json:"curve_date"`
	SettlementDate      string  `json:"settlement_date"`
	ISIN                string  `json:"isin"`
	CurveType           string  `json:"curve_type"`
	CurveSettlementDays int     `json:"curve_settlement_days"`
	CurveDayCount       string  `json:"curve_day_count,omitempty"`
	BondMaturityDate    string  `json:"bond_maturity_date"`
	BondNotional        float64 `json:"bond_notional"`
	BondDirtyPrice      float64 `json:"bond_dirty_price"`
	BondPVOIS           float64 `json:"bond_pv_ois"`
	SwapPV01BP          float64 `json:"swap_pv01_bp"`
	ASWSpreadBP         float64 `json:"asw_spread_bp"`
	ASWType             string  `json:"asw_type"`
}

func main() {
	inputParams := flag.String("input-params", "", "ASW fixture JSON path")
	input := flag.String("input", "", "ASW fixture JSON path (alias of -input-params)")
	flag.Parse()

	path := strings.TrimSpace(*inputParams)
	if path == "" {
		path = strings.TrimSpace(*input)
	}
	if path == "" {
		fmt.Fprintf(os.Stderr, "usage: aswspread -input-params /path/to/input.json\n")
		os.Exit(2)
	}

	path = resolvePath(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}

	var fixture aswFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		fmt.Fprintf(os.Stderr, "parse input: %v\n", err)
		os.Exit(1)
	}
	if fixture.CurveType == "" {
		fmt.Fprintf(os.Stderr, "input: curve_type is required\n")
		os.Exit(1)
	}

	curveDate, err := time.Parse("2006-01-02", fixture.CurveDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "input: curve_date parse: %v\n", err)
		os.Exit(1)
	}

	floatLeg, err := floatLegFromFixture(fixture)
	if err != nil {
		fmt.Fprintf(os.Stderr, "input: float leg: %v\n", err)
		os.Exit(1)
	}

	curveCal, err := calendarFromFloatLeg(floatLeg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "input: calendar: %v\n", err)
		os.Exit(1)
	}

	settlement := curveDate
	if fixture.CurveSettlementLagDays > 0 {
		settlement = calendar.AddBusinessDays(curveCal, curveDate, fixture.CurveSettlementLagDays)
	}

	disc, err := buildDiscountCurve(fixture, settlement, curveCal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build curve: %v\n", err)
		os.Exit(1)
	}

	outputs := make([]aswOutput, 0, len(fixture.Bonds))

	for _, tc := range fixture.Bonds {
		cfs := make([]bond.Cashflow, 0, len(tc.Cashflows))
		for _, r := range tc.Cashflows {
			d, err := time.Parse("2006-01-02", r.Date)
			if err != nil {
				fmt.Fprintf(os.Stderr, "isin=%s cashflow date parse: %v\n", tc.ISIN, err)
				os.Exit(1)
			}
			cfs = append(cfs, bond.Cashflow{
				Date:      d,
				Coupon:    float64(r.Coupon),
				Principal: float64(r.Principal),
			})
		}

		pxDirty, _ := tc.BondDirtyPrice.Float64()
		dirtyPrice := tc.Notional * pxDirty / 100.0

		// Determine ASW type from fixture (default: PAR-PAR).
		aswType := bond.ASWTypeParPar
		if strings.EqualFold(fixture.ASWType, "MMS") {
			aswType = bond.ASWTypeMMS
		}

		res, err := bond.ComputeASWSpread(bond.ASWInput{
			SettlementDate: settlement,
			DirtyPrice:     dirtyPrice,
			Notional:       tc.Notional,
			Cashflows:      cfs,
			FloatLeg:       floatLeg,
			DiscountCurve:  disc,
			ASWType:        aswType,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "isin=%s ComputeASWSpread: %v\n", tc.ISIN, err)
			os.Exit(1)
		}

		maturity := maturityDate(cfs)

		out := aswOutput{
			CurveDate:           fixture.CurveDate,
			SettlementDate:      settlement.Format("2006-01-02"),
			ISIN:                tc.ISIN,
			CurveType:           fixture.CurveType,
			CurveSettlementDays: fixture.CurveSettlementLagDays,
			CurveDayCount:       fixture.CurveFixedLegDayCount,
			BondMaturityDate:    maturity.Format("2006-01-02"),
			BondNotional:        tc.Notional,
			BondDirtyPrice:      pxDirty,
			BondPVOIS:           res.PVBondRF,
			SwapPV01BP:          res.PV01,
			ASWSpreadBP:         res.SpreadBP,
			ASWType:             string(aswType),
		}
		outputs = append(outputs, out)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(outputs); err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		os.Exit(1)
	}
}

func resolvePath(value string) string {
	if value == "" {
		return value
	}
	if _, err := os.Stat(value); err == nil {
		return value
	}

	clean := filepath.Clean(value)
	if strings.HasPrefix(clean, "bond"+string(filepath.Separator)) {
		trimmed := strings.TrimPrefix(clean, "bond"+string(filepath.Separator))
		if _, err := os.Stat(trimmed); err == nil {
			return trimmed
		}
	}

	return value
}

func maturityDate(cfs []bond.Cashflow) time.Time {
	var maturity time.Time
	for _, cf := range cfs {
		if cf.Date.After(maturity) {
			maturity = cf.Date
		}
	}
	return maturity
}

func calendarFromFloatLeg(floatLeg market.LegConvention) (calendar.CalendarID, error) {
	if floatLeg.Calendar != "" {
		return floatLeg.Calendar, nil
	}
	return "", fmt.Errorf("calendar is required (no calendar on float leg)")
}

func buildDiscountCurve(fixture aswFixture, settlement time.Time, cal calendar.CalendarID) (swap.DiscountCurve, error) {
	switch strings.ToUpper(strings.TrimSpace(fixture.CurveType)) {
	case "KRXIRS":
		return buildKRXCurve(settlement, fixture.CurveQuotes)
	case "IRS", "OIS", "OIS/IRS":
		if fixture.CurveFixedLegDayCount == "" {
			return nil, fmt.Errorf("curve_fixed_leg_day_count is required for curve_type=%q", fixture.CurveType)
		}
		quotes := make(map[string]float64, len(fixture.CurveQuotes))
		for _, q := range fixture.CurveQuotes {
			quotes[q.Tenor] = q.Rate
		}
		return buildCurveFromConvention(settlement, quotes, cal, fixture.CurveFixedLegDayCount)
	default:
		return nil, fmt.Errorf("unsupported curve_type=%q", fixture.CurveType)
	}
}

func buildCurveFromConvention(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, fixedLegDC string) (*curve.Curve, error) {
	switch fixedLegDC {
	case "30/360":
		return curve.BuildIBORDiscountCurve(settlement, quotes, cal, 1), nil
	case "ACT/360", "ACT/ACT":
		return curve.BuildCurve(settlement, quotes, cal, 1), nil
	default:
		return nil, fmt.Errorf("unknown curve_fixed_leg_day_count %q", fixedLegDC)
	}
}

func floatLegFromFixture(fixture aswFixture) (market.LegConvention, error) {
	if fixture.FloatingSwapLeg != "" {
		return floatLegFromString(fixture.FloatingSwapLeg)
	}
	if fixture.FloatLegConvention != "" {
		return floatLegFromString(fixture.FloatLegConvention)
	}
	if fixture.CurveFloatIndex != "" {
		return floatLegFromString(fixture.CurveFloatIndex)
	}
	return market.LegConvention{}, fmt.Errorf("swap floating leg is required (set floating_swap_leg)")
}

func floatLegFromString(value string) (market.LegConvention, error) {
	switch value {
	case "ESTR", "ESTRFloating":
		return swaps.ESTRFloating, nil
	case "EURIBOR3M", "EURIBOR3MFloating":
		return swaps.EURIBOR3MFloating, nil
	case "EURIBOR6M", "EURIBOR6MFloating":
		return swaps.EURIBOR6MFloating, nil
	case "KRXCD91DFloating":
		return swaps.KRXCD91DFloating, nil
	case "TIBOR3M", "TIBOR3MFloating":
		return swaps.TIBOR3MFloating, nil
	case "TIBOR6M", "TIBOR6MFloating":
		return swaps.TIBOR6MFloating, nil
	case "TONAR", "TONARFloating":
		return swaps.TONARFloating, nil
	case "SOFR", "SOFRFloating":
		return swaps.SOFRFloating, nil
	default:
		return market.LegConvention{}, fmt.Errorf("unknown float leg %q", value)
	}
}

func buildKRXCurve(settlement time.Time, quotes []curveQuote) (*krxch.Curve, error) {
	if settlement.IsZero() {
		return nil, fmt.Errorf("settlement date is required")
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("curve_quotes are required")
	}

	parsed := make(krxch.ParSwapQuotes, len(quotes))
	for _, q := range quotes {
		tenorYears, err := krxTenorToYears(q.Tenor)
		if err != nil {
			return nil, fmt.Errorf("parse KRX tenor %q: %w", q.Tenor, err)
		}
		parsed[tenorYears] = q.Rate
	}

	return krxch.BootstrapCurve(settlement.Format("2006-01-02"), parsed), nil
}

func krxTenorToYears(value string) (float64, error) {
	t := strings.ToUpper(strings.TrimSpace(value))
	if t == "" {
		return 0, fmt.Errorf("empty tenor")
	}

	if t == "1D" {
		return 0, nil
	}

	parseNum := func(s string) (float64, error) {
		return strconv.ParseFloat(s, 64)
	}

	switch {
	case strings.HasSuffix(t, "Y"):
		return parseNum(strings.TrimSuffix(t, "Y"))
	case strings.HasSuffix(t, "M"):
		n, err := parseNum(strings.TrimSuffix(t, "M"))
		if err != nil {
			return 0, err
		}
		return n / 12.0, nil
	case strings.HasSuffix(t, "D"):
		n, err := parseNum(strings.TrimSuffix(t, "D"))
		if err != nil {
			return 0, err
		}
		if n == 91 {
			return 0.25, nil
		}
		if n == 1 {
			return 0, nil
		}
		return n / 365.0, nil
	default:
		return parseNum(t)
	}
}
