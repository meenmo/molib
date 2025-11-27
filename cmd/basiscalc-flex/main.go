package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/meenmo/molib/swap/basis"
	"github.com/meenmo/molib/swap/basis/data"
	"github.com/meenmo/molib/swap/benchmark"
)

func main() {
	// Define command-line flags
	dateStr := flag.String("date", "2025-11-21", "Curve date in YYYY-MM-DD format")
	provider := flag.String("provider", "BGN", "Data provider: BGN or LCH")
	currency := flag.String("currency", "EUR", "Currency: EUR or JPY")
	fwdStart := flag.Int("forward", 10, "Forward start in years")
	swapTenor := flag.Int("tenor", 10, "Swap tenor in years")

	flag.Parse()

	// Parse date
	curveDate, err := time.Parse("2006-01-02", *dateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date format: %s (use YYYY-MM-DD)\n", *dateStr)
		os.Exit(1)
	}

	// Select data based on provider and currency
	var oisQuotes, ibor3mQuotes, ibor6mQuotes map[string]float64
	var payLeg, recLeg, oisLeg benchmark.LegConvention

	switch {
	case *provider == "BGN" && *currency == "EUR":
		oisQuotes = data.BGNEstr
		ibor3mQuotes = data.BGNEuribor3M
		ibor6mQuotes = data.BGNEuribor6M
		payLeg = benchmark.EURIBOR6MFloat
		recLeg = benchmark.EURIBOR3MFloat
		oisLeg = benchmark.ESTRFloat

	case *provider == "LCH" && *currency == "EUR":
		oisQuotes = data.LCHEstr
		ibor3mQuotes = data.LCHEuribor3M
		ibor6mQuotes = data.LCHEuribor6M
		payLeg = benchmark.EURIBOR6MFloat
		recLeg = benchmark.EURIBOR3MFloat
		oisLeg = benchmark.ESTRFloat

	case *provider == "BGN" && *currency == "JPY":
		oisQuotes = data.BGNTonar
		ibor3mQuotes = data.BGNTibor3M
		ibor6mQuotes = data.BGNTibor6M
		payLeg = benchmark.TIBOR6MFloat
		recLeg = benchmark.TIBOR3MFloat
		oisLeg = benchmark.TONARFloat

	default:
		fmt.Fprintf(os.Stderr, "Unsupported combination: provider=%s, currency=%s\n", *provider, *currency)
		fmt.Fprintf(os.Stderr, "Supported: BGN/EUR, LCH/EUR, BGN/JPY\n")
		os.Exit(1)
	}

	// Calculate spread
	spread, pv := basis.CalculateSpread(
		curveDate,
		*fwdStart,
		*swapTenor,
		payLeg,
		recLeg,
		oisLeg,
		oisQuotes,
		ibor6mQuotes,
		ibor3mQuotes,
		10_000_000.0,
	)

	fmt.Printf("Curve Date:  %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Provider:    %s\n", *provider)
	fmt.Printf("Currency:    %s\n", *currency)
	fmt.Printf("Structure:   %dY forward-starting %dY swap\n", *fwdStart, *swapTenor)
	fmt.Printf("Spread:      %.6f bp\n", spread)
	fmt.Printf("NPV:         %.2f\n", pv.TotalPV)
}
