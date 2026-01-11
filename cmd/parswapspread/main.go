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

// PricingInput defines the JSON input schema for basis swap pricing.
type PricingInput struct {
	TaskID string `json:"task_id,omitempty"`

	CurveDate    string             `json:"curve_date"`    // "2025-12-16"
	TradeDate    string             `json:"trade_date"`    // "2025-12-17"
	ForwardTenor int                `json:"forward_tenor"` // years
	SwapTenor    int                `json:"swap_tenor"`    // years
	Notional     float64            `json:"notional"`
	PayLeg       string             `json:"pay_leg"`    // "TIBOR6M", "TIBOR3M", "EURIBOR6M", "EURIBOR3M"
	RecLeg       string             `json:"rec_leg"`    // "TIBOR3M", "TONAR", "EURIBOR3M", "ESTR"
	OISIndex     string             `json:"ois_index"`  // "TONAR", "ESTR"
	OISQuotes    map[string]float64 `json:"ois_quotes"` // tenor -> rate%
	PayLegQuotes map[string]float64 `json:"pay_leg_quotes"`
	RecLegQuotes map[string]float64 `json:"rec_leg_quotes"`
}

// PricingOutput defines the JSON output schema.
type PricingOutput struct {
	TaskID string `json:"task_id,omitempty"`

	SpreadBP      float64 `json:"spread_bp"`
	PayLegPV      float64 `json:"pay_leg_pv"`
	RecLegPV      float64 `json:"rec_leg_pv"`
	TotalNPV      float64 `json:"total_npv"`
	EffectiveDate string  `json:"effective_date"`
	MaturityDate  string  `json:"maturity_date"`
	Error         string  `json:"error,omitempty"`
}

// legConventions maps string identifiers to LegConvention.
var legConventions = map[string]market.LegConvention{
	"TIBOR6M":   swaps.TIBOR6MFloating,
	"TIBOR3M":   swaps.TIBOR3MFloating,
	"TONAR":     swaps.TONARFloating,
	"EURIBOR6M": swaps.EURIBOR6MFloating,
	"EURIBOR3M": swaps.EURIBOR3MFloating,
	"ESTR":      swaps.ESTRFloating,
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
		out, err := calculateSpread(in)
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
	fmt.Println("  parswapspread < input.json")
	fmt.Println("  parswapspread -input /path/to/input.json")
	fmt.Println()
	fmt.Println("Read JSON input, calculate par swap spread, output JSON to stdout.")
	fmt.Println()
	fmt.Println("Example input:")
	fmt.Println(`  {`)
	fmt.Println(`    "curve_date": "2025-12-16",`)
	fmt.Println(`    "trade_date": "2025-12-17",`)
	fmt.Println(`    "forward_tenor": 5,`)
	fmt.Println(`    "swap_tenor": 5,`)
	fmt.Println(`    "notional": 10000000,`)
	fmt.Println(`    "pay_leg": "TIBOR6M",`)
	fmt.Println(`    "rec_leg": "TIBOR3M",`)
	fmt.Println(`    "ois_index": "TONAR",`)
	fmt.Println(`    "ois_quotes": {"1Y": 0.842, "2Y": 1.05, ...},`)
	fmt.Println(`    "pay_leg_quotes": {"1Y": 1.14, "2Y": 1.37, ...},`)
	fmt.Println(`    "rec_leg_quotes": {"1Y": 1.17, "2Y": 1.40, ...}`)
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

func calculateSpread(input PricingInput) (*PricingOutput, error) {
	curveDate, err := time.Parse("2006-01-02", input.CurveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid curve_date: %v", err)
	}

	tradeDate, err := time.Parse("2006-01-02", input.TradeDate)
	if err != nil {
		return nil, fmt.Errorf("invalid trade_date: %v", err)
	}

	payLeg, ok := legConventions[input.PayLeg]
	if !ok {
		return nil, fmt.Errorf("unknown pay_leg: %s", input.PayLeg)
	}

	recLeg, ok := legConventions[input.RecLeg]
	if !ok {
		return nil, fmt.Errorf("unknown rec_leg: %s", input.RecLeg)
	}

	oisLeg, ok := legConventions[input.OISIndex]
	if !ok {
		return nil, fmt.Errorf("unknown ois_index: %s", input.OISIndex)
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
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisLeg,
		OISQuotes:         input.OISQuotes,
		PayLegQuotes:      input.PayLegQuotes,
		RecLegQuotes:      input.RecLegQuotes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build swap: %v", err)
	}

	spreadBP, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		return nil, fmt.Errorf("failed to solve par spread: %v", err)
	}

	return &PricingOutput{
		TaskID:        input.TaskID,
		SpreadBP:      spreadBP,
		PayLegPV:      pv.PayLegPV,
		RecLegPV:      pv.RecLegPV,
		TotalNPV:      pv.TotalPV,
		EffectiveDate: trade.Spec.EffectiveDate.Format("2006-01-02"),
		MaturityDate:  trade.Spec.MaturityDate.Format("2006-01-02"),
	}, nil
}
