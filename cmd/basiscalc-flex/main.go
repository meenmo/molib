package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	swaps "github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/market"
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
	var payLeg, recLeg, oisLeg market.LegConvention
	dataSource := swap.DataSourceBGN
	clearingHouse := swap.ClearingHouseOTC

	switch {
	case *provider == "BGN" && *currency == "EUR":
		oisQuotes = marketdata.BGNEstr
		ibor3mQuotes = marketdata.BGNEuribor3M
		ibor6mQuotes = marketdata.BGNEuribor6M
		payLeg = swaps.EURIBOR6MFloat
		recLeg = swaps.EURIBOR3MFloat
		oisLeg = swaps.ESTRFloat

	case *provider == "LCH" && *currency == "EUR":
		dataSource = swap.DataSourceLCH
		oisQuotes = marketdata.LCHEstr
		ibor3mQuotes = marketdata.LCHEuribor3M
		ibor6mQuotes = marketdata.LCHEuribor6M
		payLeg = swaps.EURIBOR6MFloat
		recLeg = swaps.EURIBOR3MFloat
		oisLeg = swaps.ESTRFloat

	case *provider == "BGN" && *currency == "JPY":
		oisQuotes = marketdata.BGNTonar
		ibor3mQuotes = marketdata.BGNTibor3M
		ibor6mQuotes = marketdata.BGNTibor6M
		payLeg = swaps.TIBOR6MFloat
		recLeg = swaps.TIBOR3MFloat
		oisLeg = swaps.TONARFloat

	default:
		fmt.Fprintf(os.Stderr, "Unsupported combination: provider=%s, currency=%s\n", *provider, *currency)
		fmt.Fprintf(os.Stderr, "Supported: BGN/EUR, LCH/EUR, BGN/JPY\n")
		os.Exit(1)
	}

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        dataSource,
		ClearingHouse:     clearingHouse,
		CurveDate:         curveDate,
		TradeDate:         curveDate,
		ValuationDate:     curveDate,
		ForwardTenorYears: *fwdStart,
		SwapTenorYears:    *swapTenor,
		Notional:          10_000_000.0,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisLeg,
		OISQuotes:         oisQuotes,
		PayLegQuotes:      ibor6mQuotes,
		RecLegQuotes:      ibor3mQuotes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building trade: %v\n", err)
		os.Exit(1)
	}

	spread, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error solving spread: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Curve Date:  %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Provider:    %s\n", *provider)
	fmt.Printf("Currency:    %s\n", *currency)
	fmt.Printf("Structure:   %dY forward-starting %dY swap\n", *fwdStart, *swapTenor)
	fmt.Printf("Spread:      %.6f bp\n", spread)
	fmt.Printf("NPV:         %.2f\n", pv.TotalPV)
}
