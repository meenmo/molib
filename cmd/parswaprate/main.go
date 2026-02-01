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

	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
)

// PricingInput defines the JSON input schema for OIS par swap rate calculation.
type PricingInput struct {
	TaskID string `json:"task_id,omitempty"`

	CurveDate         string             `json:"curve_date"`
	TradeDate         string             `json:"trade_date"`
	ForwardTenor      int                `json:"forward_tenor"`
	SwapTenor         int                `json:"swap_tenor"`
	Notional          float64            `json:"notional"`
	FloatingRateIndex string             `json:"floating_rate_index"`
	OISQuotes         map[string]float64 `json:"ois_quotes"`
	CurveSource       string             `json:"curve_source,omitempty"`
}

// PricingOutput defines the JSON output schema.
type PricingOutput struct {
	TaskID        string  `json:"task_id,omitempty"`
	ParRatePct    float64 `json:"par_rate_pct"`
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
	fmt.Println(`    "ois_quotes": {"1Y": 0.9125, "2Y": 1.165, ...}`)
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
		return nil, fmt.Errorf("unknown floating_rate_index: %s (must be TONAR, ESTR, or SOFR)", input.FloatingRateIndex)
	}

	if input.OISQuotes == nil || len(input.OISQuotes) == 0 {
		return nil, fmt.Errorf("ois_quotes is required")
	}

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     tradeDate,
		ForwardTenorYears: input.ForwardTenor,
		SwapTenorYears:    input.SwapTenor,
		Notional:          input.Notional,
		PayLeg:            preset.FixedLeg,
		RecLeg:            preset.FloatLeg,
		DiscountingOIS:    preset.FloatLeg,
		OISQuotes:         input.OISQuotes,
		RecLegQuotes:      input.OISQuotes,
	})
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
