package ois

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
)

// PricingInput defines the JSON input schema for vanilla OIS NPV.
//
// Conventions:
// - rates are in percent (e.g., 2.50 means 2.50%)
// - spreads are in bp (e.g., 10 means +10bp)
type PricingInput struct {
	CurveDate     string `json:"curve_date"`     // "2025-12-15"
	TradeDate     string `json:"trade_date"`     // "2025-12-15"
	ValuationDate string `json:"valuation_date"` // optional, defaults to trade_date

	ForwardTenorYears int `json:"forward_tenor"` // years
	SwapTenorYears    int `json:"swap_tenor"`    // years

	Notional float64 `json:"notional"`

	// Direction is from the trader perspective:
	// - PAY (pay fixed, receive floating)
	// - REC (receive fixed, pay floating)
	Direction string `json:"direction"`

	// OISIndex is the overnight index (e.g., ESTR, TONAR).
	OISIndex string `json:"ois_index"`

	FixedRatePct  float64            `json:"fixed_rate"`
	FloatSpreadBP float64            `json:"float_spread_bp"`
	OISQuotesPct  map[string]float64 `json:"ois_quotes"`
}

type PricingOutput struct {
	PayLegPV      float64 `json:"pay_leg_pv"`
	RecLegPV      float64 `json:"rec_leg_pv"`
	TotalNPV      float64 `json:"total_npv"`
	SpotDate      string  `json:"spot_date"`
	EffectiveDate string  `json:"effective_date"`
	MaturityDate  string  `json:"maturity_date"`
	Error         string  `json:"error,omitempty"`
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ois", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "JSON input path (optional; if set, ignores stdin)")
	help := fs.Bool("h", false, "Show help")
	fs.BoolVar(help, "help", false, "Show help")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		usage(stderr)
		return 0
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" {
		if f, ok := stdin.(*os.File); ok {
			if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
				usage(stderr)
				return 2
			}
		}
	}

	inputBytes, err := readInput(stdin, path)
	if err != nil {
		return writeError(stdout, fmt.Sprintf("failed to read input: %v", err))
	}

	var input PricingInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return writeError(stdout, fmt.Sprintf("failed to parse JSON input: %v", err))
	}

	output, err := calculateNPV(input)
	if err != nil {
		return writeError(stdout, err.Error())
	}

	outputBytes, _ := json.Marshal(output)
	fmt.Fprintln(stdout, string(outputBytes))
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  npv ois < input.json")
	fmt.Fprintln(w, "  npv ois -input /path/to/input.json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Read JSON input, calculate vanilla OIS NPV, output JSON to stdout.")
}

func readInput(stdin io.Reader, path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(stdin)
}

func writeError(stdout io.Writer, msg string) int {
	output := PricingOutput{Error: msg}
	outputBytes, _ := json.Marshal(output)
	fmt.Fprintln(stdout, string(outputBytes))
	return 1
}

func calculateNPV(input PricingInput) (*PricingOutput, error) {
	curveDate, err := time.Parse("2006-01-02", input.CurveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid curve_date: %v", err)
	}
	tradeDate, err := time.Parse("2006-01-02", input.TradeDate)
	if err != nil {
		return nil, fmt.Errorf("invalid trade_date: %v", err)
	}
	valuationDate := tradeDate
	if strings.TrimSpace(input.ValuationDate) != "" {
		valuationDate, err = time.Parse("2006-01-02", input.ValuationDate)
		if err != nil {
			return nil, fmt.Errorf("invalid valuation_date: %v", err)
		}
	}

	if input.Notional == 0 {
		return nil, fmt.Errorf("notional is required")
	}
	if input.ForwardTenorYears < 0 || input.SwapTenorYears <= 0 {
		return nil, fmt.Errorf("forward_tenor must be >=0 and swap_tenor must be >0")
	}
	if strings.TrimSpace(input.Direction) == "" {
		return nil, fmt.Errorf("direction is required (PAY or REC)")
	}
	if input.OISQuotesPct == nil || len(input.OISQuotesPct) == 0 {
		return nil, fmt.Errorf("ois_quotes is required")
	}

	oisFloat, err := floatLegFromString(input.OISIndex)
	if err != nil {
		return nil, err
	}
	if !market.IsOvernight(oisFloat.ReferenceIndex) {
		return nil, fmt.Errorf("ois_index must be an overnight index, got %q", input.OISIndex)
	}
	oisFloat = withoutPrincipal(oisFloat)

	fixedLeg, err := fixedLegFromOISIndex(input.OISIndex)
	if err != nil {
		return nil, err
	}
	fixedLeg = withoutPrincipal(fixedLeg)

	fixedRateBP := input.FixedRatePct * 100.0

	dir := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.Direction), "-", "_"))
	var (
		payLeg       market.LegConvention
		recLeg       market.LegConvention
		paySpreadBP  float64
		recSpreadBP  float64
		payLegQuotes map[string]float64
		recLegQuotes map[string]float64
	)

	switch dir {
	case "PAY_FIXED", "PAY":
		payLeg = fixedLeg
		recLeg = oisFloat
		paySpreadBP = fixedRateBP
		recSpreadBP = input.FloatSpreadBP
		recLegQuotes = input.OISQuotesPct
	case "REC_FIXED", "REC":
		payLeg = oisFloat
		recLeg = fixedLeg
		paySpreadBP = input.FloatSpreadBP
		recSpreadBP = fixedRateBP
		payLegQuotes = input.OISQuotesPct
	default:
		return nil, fmt.Errorf("invalid direction %q (use PAY or REC)", input.Direction)
	}

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: input.ForwardTenorYears,
		SwapTenorYears:    input.SwapTenorYears,
		Notional:          input.Notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisFloat, // OIS discounting
		OISQuotes:         input.OISQuotesPct,
		PayLegQuotes:      payLegQuotes,
		RecLegQuotes:      recLegQuotes,
		PayLegSpreadBP:    paySpreadBP,
		RecLegSpreadBP:    recSpreadBP,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build OIS: %v", err)
	}

	pv, err := trade.PVByLeg()
	if err != nil {
		return nil, fmt.Errorf("failed to price OIS: %v", err)
	}

	return &PricingOutput{
		PayLegPV:      pv.PayLegPV,
		RecLegPV:      pv.RecLegPV,
		TotalNPV:      pv.TotalPV,
		SpotDate:      trade.SpotDate.Format("2006-01-02"),
		EffectiveDate: trade.Spec.EffectiveDate.Format("2006-01-02"),
		MaturityDate:  trade.Spec.MaturityDate.Format("2006-01-02"),
	}, nil
}

func withoutPrincipal(leg market.LegConvention) market.LegConvention {
	leg.IncludeInitialPrincipal = false
	leg.IncludeFinalPrincipal = false
	return leg
}

func floatLegFromString(value string) (market.LegConvention, error) {
	switch strings.TrimSpace(value) {
	case "ESTR", "ESTRFloating":
		return swaps.ESTRFloating, nil
	case "SOFR", "SOFRFloating":
		return swaps.SOFRFloating, nil
	case "TONAR", "TONARFloating":
		return swaps.TONARFloating, nil
	default:
		return market.LegConvention{}, fmt.Errorf("unknown ois_index %q", value)
	}
}

func fixedLegFromOISIndex(value string) (market.LegConvention, error) {
	switch strings.TrimSpace(value) {
	case "ESTR", "ESTRFloating":
		return swaps.ESTRFixed, nil
	case "SOFR", "SOFRFloating":
		return swaps.SOFRFixed, nil
	case "TONAR", "TONARFloating":
		return swaps.TONARFixed, nil
	default:
		return market.LegConvention{}, fmt.Errorf("unknown ois_index %q", value)
	}
}
