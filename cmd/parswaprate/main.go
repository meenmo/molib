package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	krx "github.com/meenmo/molib/swap/clearinghouse/krx"
	"github.com/meenmo/molib/swap/market"
)

// PricingInput defines the JSON input schema for OIS par swap rate calculation.
type PricingInput struct {
	TaskID string `json:"task_id,omitempty"`

	CurveDate         string             `json:"curve_date"`
	TradeDate         string             `json:"trade_date"`
	ForwardTenor      int                `json:"forward_tenor"`
	SwapTenor         int                `json:"swap_tenor"`
	EffectiveDate     string             `json:"effective_date,omitempty"`
	MaturityDate      string             `json:"maturity_date,omitempty"`
	Notional          float64            `json:"notional"`
	FloatingRateIndex string             `json:"floating_rate_index"`
	CurveQuotes         map[string]float64 `json:"curve_quotes"`
	CurveSource       string             `json:"curve_source,omitempty"`

	// ReferenceRateFixings is required for CD91D when the first floating
	// period's reset date precedes the trade date. Maps "YYYY-MM-DD" to
	// the fixing in percent.
	ReferenceRateFixings map[string]float64 `json:"reference_rate_fixings,omitempty"`
}

// PricingOutput defines the JSON output schema.
type PricingOutput struct {
	TaskID        string  `json:"task_id,omitempty"`
	ParRatePct    float64 `json:"par_rate"`
	FixedLegPV    float64 `json:"fixed_leg_pv"`
	FloatingLegPV float64 `json:"floating_leg_pv"`
	TotalNPV      float64 `json:"total_npv"`
	EffectiveDate string  `json:"effective_date"`
	MaturityDate  string  `json:"maturity_date"`
	Error         string  `json:"error,omitempty"`
}

// OISPreset groups fixed and floating leg conventions for an OIS swap.
type OISPreset struct {
	FixedLeg market.LegConvention
	FloatLeg market.LegConvention
}

// oisPresets maps OIS index names to their leg conventions.
// Floating legs have principal exchange disabled for standard OIS par rate calculation.
var oisPresets = map[string]OISPreset{
	"TONAR": {
		FixedLeg: swaps.TONARFixed,
		FloatLeg: func() market.LegConvention {
			l := swaps.TONARFloating
			l.IncludeInitialPrincipal = false
			l.IncludeFinalPrincipal = false
			return l
		}(),
	},
	"ESTR": {
		FixedLeg: swaps.ESTRFixed,
		FloatLeg: func() market.LegConvention {
			l := swaps.ESTRFloating
			l.IncludeInitialPrincipal = false
			l.IncludeFinalPrincipal = false
			return l
		}(),
	},
	"SOFR": {
		FixedLeg: swaps.SOFRFixed,
		FloatLeg: func() market.LegConvention {
			l := swaps.SOFRFloating
			l.IncludeInitialPrincipal = false
			l.IncludeFinalPrincipal = false
			return l
		}(),
	},
	"SONIA": {
		FixedLeg: swaps.SONIAFixed,
		FloatLeg: func() market.LegConvention {
			l := swaps.SONIAFloating
			l.IncludeInitialPrincipal = false
			l.IncludeFinalPrincipal = false
			return l
		}(),
	},
}

func main() {
	inputPath := flag.String("input", "", "JSON input path (optional; if set, ignores stdin)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		usage()
		return
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" {
		if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			usage()
			os.Exit(2)
		}
	}

	inputBytes, err := readInput(path)
	if err != nil {
		writeError(fmt.Sprintf("failed to read input: %v", err))
		return
	}

	inputs, isArray, err := parseInputs(inputBytes)
	if err != nil {
		writeError(fmt.Sprintf("failed to parse JSON input: %v", err))
		return
	}

	hadError := false
	outputs := make([]PricingOutput, 0, len(inputs))
	for _, in := range inputs {
		out, err := calculateParRate(in)
		if err != nil {
			hadError = true
			outputs = append(outputs, PricingOutput{
				TaskID: in.TaskID,
				Error:  err.Error(),
			})
			continue
		}
		outputs = append(outputs, *out)
	}

	if isArray {
		outputBytes, _ := json.Marshal(outputs)
		fmt.Println(string(outputBytes))
	} else {
		outputBytes, _ := json.Marshal(outputs[0])
		fmt.Println(string(outputBytes))
	}

	if hadError {
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  parswaprate < input.json")
	fmt.Println("  parswaprate -input /path/to/input.json")
	fmt.Println()
	fmt.Println("Read JSON input, calculate OIS par swap rate, output JSON to stdout.")
	fmt.Println()
	fmt.Println("Example input:")
	fmt.Println(`  {`)
	fmt.Println(`    "curve_date": "2026-01-09",`)
	fmt.Println(`    "trade_date": "2026-01-09",`)
	fmt.Println(`    "forward_tenor": 1,`)
	fmt.Println(`    "swap_tenor": 4,`)
	fmt.Println(`    "notional": 1000000,`)
	fmt.Println(`    "floating_rate_index": "TONAR",`)
	fmt.Println(`    "curve_quotes": {"1Y": 0.9125, "2Y": 1.165, ...}`)
	fmt.Println(`  }`)
}

func readInput(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func parseInputs(raw []byte) ([]PricingInput, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, fmt.Errorf("empty input")
	}

	if trimmed[0] == '[' {
		var inputs []PricingInput
		if err := json.Unmarshal(trimmed, &inputs); err != nil {
			return nil, true, err
		}
		if len(inputs) == 0 {
			return nil, true, fmt.Errorf("empty input array")
		}
		return inputs, true, nil
	}

	var input PricingInput
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return nil, false, err
	}
	return []PricingInput{input}, false, nil
}

func writeError(msg string) {
	output := PricingOutput{Error: msg}
	outputBytes, _ := json.Marshal(output)
	fmt.Println(string(outputBytes))
	os.Exit(1)
}

func calculateParRate(input PricingInput) (*PricingOutput, error) {
	switch strings.ToUpper(strings.TrimSpace(input.FloatingRateIndex)) {
	case "CD91", "CD91D":
		return calculateKRXParRate(input)
	}

	curveDate, err := time.Parse("2006-01-02", input.CurveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid curve_date: %v", err)
	}

	tradeDate, err := time.Parse("2006-01-02", input.TradeDate)
	if err != nil {
		return nil, fmt.Errorf("invalid trade_date: %v", err)
	}

	preset, ok := oisPresets[input.FloatingRateIndex]
	if !ok {
		return nil, fmt.Errorf("unknown floating_rate_index: %s (must be TONAR, ESTR, SOFR, SONIA, or CD91D)", input.FloatingRateIndex)
	}

	if input.CurveQuotes == nil || len(input.CurveQuotes) == 0 {
		return nil, fmt.Errorf("curve_quotes is required")
	}

	hasExplicitDates := input.EffectiveDate != "" || input.MaturityDate != ""
	hasTenors := input.ForwardTenor > 0 || input.SwapTenor > 0

	if hasExplicitDates && hasTenors {
		return nil, fmt.Errorf("specify either (effective_date + maturity_date) or (forward_tenor + swap_tenor), not both")
	}
	if hasExplicitDates && (input.EffectiveDate == "" || input.MaturityDate == "") {
		return nil, fmt.Errorf("both effective_date and maturity_date are required when using explicit dates")
	}
	if !hasExplicitDates && input.SwapTenor <= 0 {
		return nil, fmt.Errorf("swap_tenor is required when effective_date/maturity_date are not specified")
	}

	params := swap.InterestRateSwapParams{
		DataSource:     swap.DataSourceBGN,
		ClearingHouse:  swap.ClearingHouseOTC,
		CurveDate:      curveDate,
		TradeDate:      tradeDate,
		ValuationDate:  tradeDate,
		Notional:       input.Notional,
		PayLeg:         preset.FixedLeg,
		RecLeg:         preset.FloatLeg,
		DiscountingOIS: preset.FloatLeg,
		OISQuotes:      input.CurveQuotes,
		RecLegQuotes:   input.CurveQuotes,
	}

	if hasExplicitDates {
		effDate, err := time.Parse("2006-01-02", input.EffectiveDate)
		if err != nil {
			return nil, fmt.Errorf("invalid effective_date: %v", err)
		}
		matDate, err := time.Parse("2006-01-02", input.MaturityDate)
		if err != nil {
			return nil, fmt.Errorf("invalid maturity_date: %v", err)
		}
		params.EffectiveDate = effDate
		params.MaturityDate = matDate
	} else {
		params.ForwardTenorYears = input.ForwardTenor
		params.SwapTenorYears = input.SwapTenor
	}

	trade, err := swap.InterestRateSwap(params)
	if err != nil {
		return nil, fmt.Errorf("failed to build swap: %v", err)
	}

	spreadBP, pv, err := trade.SolveParSpread(swap.SpreadTargetPayLeg)
	if err != nil {
		return nil, fmt.Errorf("failed to solve par rate: %v", err)
	}

	parRatePct := spreadBP / 100.0

	return &PricingOutput{
		TaskID:        input.TaskID,
		ParRatePct:    parRatePct,
		FixedLegPV:    pv.PayLegPV,
		FloatingLegPV: pv.RecLegPV,
		TotalNPV:      pv.TotalPV,
		EffectiveDate: trade.Spec.EffectiveDate.Format("2006-01-02"),
		MaturityDate:  trade.Spec.MaturityDate.Format("2006-01-02"),
	}, nil
}

// calculateKRXParRate solves the par fixed rate for a KRW CD91 IRS using the
// KRX single-curve bootstrapper. PV_fixed scales linearly with FixedRate, so
// probing at 1% lets us back out the par rate as PV_float / PV_fixed_at_1pct.
func calculateKRXParRate(input PricingInput) (*PricingOutput, error) {
	if input.ForwardTenor > 0 || input.SwapTenor > 0 {
		return nil, fmt.Errorf("forward_tenor/swap_tenor are not supported for CD91D; specify effective_date and maturity_date")
	}
	if input.EffectiveDate == "" || input.MaturityDate == "" {
		return nil, fmt.Errorf("effective_date and maturity_date are required for CD91D")
	}
	if input.Notional == 0 {
		return nil, fmt.Errorf("notional is required")
	}
	if len(input.CurveQuotes) == 0 {
		return nil, fmt.Errorf("curve_quotes is required (CD91 par swap quotes keyed by tenor)")
	}
	if input.TradeDate == "" {
		return nil, fmt.Errorf("trade_date is required (used as curve settlement date)")
	}

	effDate, err := time.Parse("2006-01-02", input.EffectiveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid effective_date: %v", err)
	}
	matDate, err := time.Parse("2006-01-02", input.MaturityDate)
	if err != nil {
		return nil, fmt.Errorf("invalid maturity_date: %v", err)
	}
	tradeDate, err := time.Parse("2006-01-02", input.TradeDate)
	if err != nil {
		return nil, fmt.Errorf("invalid trade_date: %v", err)
	}

	if !matDate.After(effDate) {
		return nil, fmt.Errorf("maturity_date must be after effective_date")
	}

	quotes := make(krx.ParSwapQuotes, len(input.CurveQuotes))
	for tenor, rate := range input.CurveQuotes {
		years, err := krx.TenorToYears(tenor)
		if err != nil {
			return nil, fmt.Errorf("parse tenor %q: %w", tenor, err)
		}
		quotes[years] = rate
	}

	// For a spot-start trade (effective == trade_date), the first-period
	// floating fixing is the previous business day's CD91 — which the
	// curve already carries as its "91D" par quote. Synthesize that one
	// entry so callers don't have to repeat market data the curve provides.
	// Explicit reference_rate_fixings always take precedence and are required
	// for back-dated trades where prior fixings can't be derived from today's
	// curve.
	refFeed := calendar.DefaultReferenceFeed()
	switch {
	case len(input.ReferenceRateFixings) > 0:
		refFeed = calendar.NewMapReferenceRateFeed(input.ReferenceRateFixings)
	case effDate.Equal(tradeDate):
		if rate91D, ok := input.CurveQuotes["91D"]; ok {
			fixingDate := calendar.AddBusinessDays(calendar.KR, tradeDate, -1)
			refFeed = calendar.NewMapReferenceRateFeed(map[string]float64{
				fixingDate.Format("2006-01-02"): rate91D,
			})
		}
	}

	curve := krx.BootstrapCurve(input.TradeDate, quotes)
	if curve == nil {
		return nil, fmt.Errorf("failed to bootstrap KRX curve")
	}

	const probeRatePct = 1.0
	probe := krx.InterestRateSwap{
		EffectiveDate:   input.EffectiveDate,
		TerminationDate: input.MaturityDate,
		SettlementDate:  input.TradeDate,
		FixedRate:       probeRatePct,
		Notional:        input.Notional,
		Direction:       krx.PositionReceive,
		SwapQuotes:      quotes,
		ReferenceIndex:  refFeed,
	}

	pvFixed, pvFloat, err := safeKRXPVByLeg(probe, curve)
	if err != nil {
		return nil, err
	}
	if pvFixed == 0 {
		return nil, fmt.Errorf("zero fixed-leg PV at probe rate; cannot back out par rate")
	}

	parRatePct := probeRatePct * pvFloat / pvFixed
	parPV := pvFloat

	return &PricingOutput{
		TaskID:        input.TaskID,
		ParRatePct:    parRatePct,
		FixedLegPV:    parPV,
		FloatingLegPV: parPV,
		TotalNPV:      0,
		EffectiveDate: effDate.Format("2006-01-02"),
		MaturityDate:  matDate.Format("2006-01-02"),
	}, nil
}

func safeKRXPVByLeg(trade krx.InterestRateSwap, curve *krx.Curve) (pvFixed, pvFloat float64, err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%v", r)
			if strings.Contains(msg, "missing reference rate fixing") {
				err = fmt.Errorf("%s — supply it via the reference_rate_fixings input field (key: \"YYYY-MM-DD\", value: rate in percent)", msg)
				return
			}
			err = fmt.Errorf("KRX pricer panic: %v", r)
		}
	}()
	pvFixed, pvFloat = trade.PVByLeg(curve)
	return pvFixed, pvFloat, nil
}
