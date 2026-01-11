package krxirs

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/meenmo/molib/calendar"
	krx "github.com/meenmo/molib/swap/clearinghouse/krx"
)

// PricingInput defines the JSON input schema for KRX CD91 IRS NPV.
//
// Conventions:
// - rates are in percent (e.g., 3.24 means 3.24%)
// - curve_quotes are par swap rates in percent
type PricingInput struct {
	SettlementDate  string       `json:"settlement_date"`  // "2025-11-21"
	EffectiveDate   string       `json:"effective_date"`   // "2024-01-25"
	TerminationDate string       `json:"termination_date"` // "2044-01-25"
	Direction       string       `json:"direction"`        // "REC" or "PAY"
	Notional        float64      `json:"notional"`
	FixedRatePct    float64      `json:"fixed_rate"`
	CurveQuotes     []curveQuote `json:"curve_quotes"`

	// ReferenceRateFixings is optional (date -> rate%).
	// If omitted, cmd uses calendar.DefaultReferenceFeed(), which may not cover all dates.
	ReferenceRateFixings map[string]float64 `json:"reference_rate_fixings"`
}

type curveQuote struct {
	Tenor string  `json:"tenor"` // "1D", "91D", "6M", "1Y", ...
	Rate  float64 `json:"rate"`  // percent
}

type PricingOutput struct {
	FixedPV    float64 `json:"fixed_pv"`
	FloatingPV float64 `json:"floating_pv"`
	NPV        float64 `json:"npv"`
	Error      string  `json:"error,omitempty"`
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("krx-irs", flag.ContinueOnError)
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
	fmt.Fprintln(w, "  npv krx-irs < input.json")
	fmt.Fprintln(w, "  npv krx-irs -input /path/to/input.json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Read JSON input, calculate KRX CD91 IRS NPV, output JSON to stdout.")
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
	if input.SettlementDate == "" || input.EffectiveDate == "" || input.TerminationDate == "" {
		return nil, fmt.Errorf("settlement_date, effective_date, termination_date are required")
	}
	if input.Notional == 0 {
		return nil, fmt.Errorf("notional is required")
	}
	if input.Direction == "" {
		return nil, fmt.Errorf("direction is required (REC or PAY)")
	}
	if len(input.CurveQuotes) == 0 {
		return nil, fmt.Errorf("curve_quotes are required")
	}

	quotes := make(krx.ParSwapQuotes, len(input.CurveQuotes))
	for _, q := range input.CurveQuotes {
		tenorYears, err := krxTenorToYears(q.Tenor)
		if err != nil {
			return nil, fmt.Errorf("parse tenor %q: %w", q.Tenor, err)
		}
		quotes[tenorYears] = q.Rate
	}

	refFeed := calendar.DefaultReferenceFeed()
	if len(input.ReferenceRateFixings) > 0 {
		refFeed = calendar.NewMapReferenceRateFeed(input.ReferenceRateFixings)
	}

	trade := krx.InterestRateSwap{
		EffectiveDate:   input.EffectiveDate,
		TerminationDate: input.TerminationDate,
		SettlementDate:  input.SettlementDate,
		FixedRate:       input.FixedRatePct,
		Notional:        input.Notional,
		Direction:       krx.Position(strings.ToUpper(strings.TrimSpace(input.Direction))),
		SwapQuotes:      quotes,
		ReferenceIndex:  refFeed,
	}

	curve := krx.BootstrapCurve(trade.SettlementDate, trade.SwapQuotes)
	if curve == nil {
		return nil, fmt.Errorf("failed to bootstrap curve")
	}

	fixedPV, floatPV, npv, err := safePrice(trade, curve)
	if err != nil {
		return nil, err
	}

	return &PricingOutput{
		FixedPV:    fixedPV,
		FloatingPV: floatPV,
		NPV:        npv,
	}, nil
}

func safePrice(trade krx.InterestRateSwap, curve *krx.Curve) (fixedPV, floatPV, npv float64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("krx pricer panic: %v", r)
		}
	}()

	fixedPV, floatPV = trade.PVByLeg(curve)
	npv = trade.NPV(curve)
	return fixedPV, floatPV, npv, nil
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
