package bond_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/meenmo/molib/bond"
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	krxch "github.com/meenmo/molib/swap/clearinghouse/krx"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/swap/market"
	"github.com/meenmo/molib/utils"
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
}

type curveQuote struct {
	Tenor string  `json:"tenor"`
	Rate  float64 `json:"rate"`
}

type bondCase struct {
	ISIN          string        `json:"isin"`
	Notional      float64       `json:"notional"`
	PXDirtyMid    float64       `json:"px_dirty_mid"`
	ExpectedASWBP *float64      `json:"expected_asw_bp"`
	Cashflows     []cashflowRow `json:"cashflows"`
}

type cashflowRow struct {
	Date      string `json:"date"`
	Coupon    int64  `json:"coupon"`
	Principal int64  `json:"principal"`
}

var (
	inputParamsPath = flag.String("input-params", "", "ASW fixture JSON path")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestComputeASW_FromFixture(t *testing.T) {
	t.Parallel()

	paths, err := fixturePaths(*inputParamsPath)
	if err != nil {
		t.Fatalf("fixture paths: %v", err)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var fixture aswFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if fixture.CurveType == "" {
				t.Fatalf("curve_type is required")
			}

			curveDate, err := time.Parse("2006-01-02", fixture.CurveDate)
			if err != nil {
				t.Fatalf("curve_date parse: %v", err)
			}

			floatLeg, err := floatLegFromFixture(fixture)
			if err != nil {
				t.Fatalf("float leg: %v", err)
			}

			curveCal, err := calendarFromFixture(fixture, floatLeg)
			if err != nil {
				t.Fatalf("calendar: %v", err)
			}

			settlement := curveDate
			if fixture.CurveSettlementLagDays > 0 {
				settlement = calendar.AddBusinessDays(curveCal, curveDate, fixture.CurveSettlementLagDays)
			}

			var disc swap.DiscountCurve
			switch strings.ToUpper(strings.TrimSpace(fixture.CurveType)) {
			case "KRXIRS":
				krxDisc, err := buildKRXCurve(settlement, fixture.CurveQuotes)
				if err != nil {
					t.Fatalf("build KRX curve: %v", err)
				}
				if krxDisc == nil {
					t.Fatalf("failed to build KRX discount curve")
				}
				disc = krxDisc
			case "IRS", "OIS", "OIS/IRS":
				if fixture.CurveFixedLegDayCount == "" {
					t.Fatalf("curve_fixed_leg_day_count is required for curve_type=%q", fixture.CurveType)
				}
				quotes := make(map[string]float64, len(fixture.CurveQuotes))
				for _, q := range fixture.CurveQuotes {
					quotes[q.Tenor] = q.Rate
				}
				eurDisc, err := buildCurveFromConvention(settlement, quotes, curveCal, fixture.CurveFixedLegDayCount)
				if err != nil {
					t.Fatalf("build curve: %v", err)
				}
				if eurDisc == nil {
					t.Fatalf("failed to build discount curve")
				}
				disc = eurDisc
			default:
				t.Fatalf("unsupported curve_type=%q", fixture.CurveType)
			}

			const tolBP = 3.0 // schedule/curve interpolation differences vs Bloomberg

			for _, tc := range fixture.Bonds {
				tc := tc
				t.Run(tc.ISIN, func(t *testing.T) {
					t.Parallel()

					cfs := make([]bond.Cashflow, 0, len(tc.Cashflows))
					for _, r := range tc.Cashflows {
						d, err := time.Parse("2006-01-02", r.Date)
						if err != nil {
							t.Fatalf("cashflow date parse: %v", err)
						}
						cfs = append(cfs, bond.Cashflow{
							Date:      d,
							Coupon:    float64(r.Coupon) / 100.0,
							Principal: float64(r.Principal) / 100.0,
						})
					}

					dirtyPrice := tc.Notional * tc.PXDirtyMid / 100.0
					got, err := bond.ComputeASWSpread(bond.ASWInput{
						SettlementDate: settlement,
						DirtyPrice:     dirtyPrice,
						Notional:       tc.Notional,
						Cashflows:      cfs,
						FloatLeg:       floatLeg,
						DiscountCurve:  disc,
					})
					if err != nil {
						t.Fatalf("ComputeASWSpread: %v", err)
					}

					maturity := maturityDate(cfs)
					years := utils.YearFraction(curveDate, maturity, "ACT/365F")
					t.Logf("curve_date=%s isin=%s maturity=%s years_to_maturity=%.6f asw_bp=%.6f",
						fixture.CurveDate, tc.ISIN, maturity.Format("2006-01-02"), years, got.SpreadBP)

					if strings.EqualFold(fixture.CurveType, "IRS") {
						if tc.ExpectedASWBP == nil {
							t.Fatalf("expected_asw_bp is required for IRS benchmarked fixtures")
						}
						if math.Abs(got.SpreadBP-*tc.ExpectedASWBP) > tolBP {
							t.Fatalf("ASW mismatch: got %.6f bp want %.6f bp (tol %.2f bp) [pvBondRF=%.6f pv01=%.6f]",
								got.SpreadBP, *tc.ExpectedASWBP, tolBP, got.PVBondRF, got.PV01)
						}
					}
				})
			}
		})
	}
}

func calendarFromFixture(fixture aswFixture, floatLeg market.LegConvention) (calendar.CalendarID, error) {
	if floatLeg.Calendar != "" {
		return floatLeg.Calendar, nil
	}
	return "", fmt.Errorf("calendar is required (no calendar on float leg)")
}

func fixturePaths(value string) ([]string, error) {
	if value == "" {
		entries, err := os.ReadDir("testdata")
		if err != nil {
			return nil, err
		}

		var paths []string
		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}
			name := ent.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".json") {
				continue
			}
			if !strings.HasPrefix(name, "input_asw_spread_") {
				continue
			}
			paths = append(paths, filepath.Join("testdata", name))
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no fixtures found under testdata (expected input_asw_spread_*.json)")
		}
		sort.Strings(paths)
		return paths, nil
	}

	if _, err := os.Stat(value); err == nil {
		return []string{value}, nil
	}

	clean := filepath.Clean(value)
	if strings.HasPrefix(clean, "bond"+string(filepath.Separator)) {
		trimmed := strings.TrimPrefix(clean, "bond"+string(filepath.Separator))
		if _, err := os.Stat(trimmed); err == nil {
			return []string{trimmed}, nil
		}
	}

	// Preserve the original error surface if the user passed a bad path.
	return []string{value}, nil
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

func buildCurveFromConvention(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, fixedLegDC string) (*curve.Curve, error) {
	switch fixedLegDC {
	case "30/360":
		return curve.BuildIBORDiscountCurve(settlement, quotes, cal, 1), nil
	case "ACT/360":
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

	// Common KRX short-end aliases.
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
		// 91D is quoted as the 3M (0.25Y) node in KRX CD IRS curves.
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
