package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
)

// PricingInput defines the JSON input schema for basis swap pricing.
type PricingInput struct {
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
	"TIBOR6M":   swaps.TIBOR6MFloat,
	"TIBOR3M":   swaps.TIBOR3MFloat,
	"TONAR":     swaps.TONARFloat,
	"EURIBOR6M": swaps.EURIBOR6MFloat,
	"EURIBOR3M": swaps.EURIBOR3MFloat,
	"ESTR":      swaps.ESTRFloat,
}

func main() {
	jsonMode := flag.Bool("json", false, "Run in JSON stdin/stdout mode")
	flag.Parse()

	if *jsonMode {
		runJSONMode()
	} else {
		fmt.Println("Usage: basiscalc --json < input.json")
		fmt.Println()
		fmt.Println("Read JSON from stdin, calculate basis swap spread, output JSON to stdout.")
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
}

func runJSONMode() {
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError(fmt.Sprintf("failed to read stdin: %v", err))
		return
	}

	var input PricingInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		writeError(fmt.Sprintf("failed to parse JSON input: %v", err))
		return
	}

	output, err := calculateSpread(input)
	if err != nil {
		writeError(err.Error())
		return
	}

	outputBytes, _ := json.Marshal(output)
	fmt.Println(string(outputBytes))
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
		SpreadBP:      spreadBP,
		PayLegPV:      pv.PayLegPV,
		RecLegPV:      pv.RecLegPV,
		TotalNPV:      pv.TotalPV,
		EffectiveDate: trade.Spec.EffectiveDate.Format("2006-01-02"),
		MaturityDate:  trade.Spec.MaturityDate.Format("2006-01-02"),
	}, nil
}
