package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	marketdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	krx "github.com/meenmo/molib/swap/clearinghouse/krx"
)

// This example demonstrates swap pricing in molib:
//
//  1. KRX CD 91D IRS priced by the legacy KRW engine (swap/clearinghouse/krx).
//  2. OTC IRS/OIS priced by the unified swap API (swap.InterestRateSwap).
//
// Run:
//
//	cd molib
//	go run ./examples/npv_plain_swap.go
func main() {
	fmt.Println("====================================================================")
	fmt.Println("Plain swap NPV examples (molib)")
	fmt.Println("====================================================================")

	exampleKRXCDIRS()
	fmt.Println()

	exampleEURIRS()
	fmt.Println()

	exampleJPYIRS()
	fmt.Println()

	exampleEUROIS()
	fmt.Println()

	exampleJPYOIS()
}

// --------------------------------------------------------------------
// 1) KRX CD 91D IRS – legacy KRW engine
// --------------------------------------------------------------------

func exampleKRXCDIRS() {
	fmt.Println("1) KRX CD 91D IRS (KRW) – legacy engine")

	// Trade and settlement conventions.
	tradeDate := time.Date(2025, 11, 21, 0, 0, 0, 0, time.UTC)
	settlementDate := calendar.AddBusinessDays(calendar.KRW, tradeDate, 1) // KRX CD swaps are typically T+1.

	// 10Y swap from settlement date.
	effectiveDate := settlementDate
	terminationDate := calendar.AdjustFollowing(calendar.KRW, settlementDate.AddDate(10, 0, 0))

	// Notional and fixed rate.
	notional := 10_000_000_000.0 // KRW 10bn
	fixedRatePercent := 3.20

	// Par swap quotes for the KRW CD curve (percent). Tenors in years.
	parQuotes := krx.ParSwapQuotes{
		0.25: 3.20,
		1.0:  3.25,
		3.0:  3.35,
		5.0:  3.40,
		10.0: 3.45,
	}

	curve := krx.BootstrapCurve(settlementDate.Format("2006-01-02"), parQuotes)

	irs := krx.InterestRateSwap{
		EffectiveDate:   effectiveDate.Format("2006-01-02"),
		TerminationDate: terminationDate.Format("2006-01-02"),
		SettlementDate:  settlementDate.Format("2006-01-02"),
		FixedRate:       fixedRatePercent,
		Notional:        notional,
		Direction:       krx.PositionReceive, // receive fixed, pay floating
		SwapQuotes:      parQuotes,
		ReferenceRate:   calendar.DefaultReferenceFeed(),
	}

	pvFixed, pvFloat := irs.PVByLeg(curve)
	npv := irs.NPV(curve)

	fmt.Printf("   Trade date:      %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("   Settlement date: %s\n", settlementDate.Format("2006-01-02"))
	fmt.Printf("   Effective date:  %s\n", effectiveDate.Format("2006-01-02"))
	fmt.Printf("   Termination:     %s\n", terminationDate.Format("2006-01-02"))
	fmt.Printf("   Notional:        KRW %.0f\n", notional)
	fmt.Printf("   Fixed rate:      %.2f%%\n", fixedRatePercent)
	fmt.Println()
	fmt.Printf("   PV (receive fixed):   %12.2f KRW\n", pvFixed)
	fmt.Printf("   PV (pay floating):    %12.2f KRW\n", pvFloat)
	fmt.Printf("   Swap NPV (receiver):  %12.2f KRW\n", npv)
}

// --------------------------------------------------------------------
// 2) EUR IRS – fixed vs EURIBOR, discounted on ESTR OIS
// --------------------------------------------------------------------

func exampleEURIRS() {
	fmt.Println("2) EUR IRS – fixed vs EURIBOR, discounted on ESTR OIS (BGN / LCH)")

	// Curve dates must match the embedded fixture files in marketdata/.
	curveDateBGN := time.Date(2025, 6, 26, 0, 0, 0, 0, time.UTC) // marketdata/fixtures_bgn_euribor.go
	curveDateLCH := time.Date(2025, 9, 24, 0, 0, 0, 0, time.UTC) // marketdata/fixtures_lch_euribor.go

	notional := 10_000_000.0 // EUR 10m
	forwardTenorYears := 0
	swapTenorYears := 10

	// Fixed coupon used to demonstrate NPV (choose any value; par will be solved too).
	fixedRatePercent := 2.50

	runIRSExample(
		"EUR IRS (BGN): fixed vs EURIBOR 3M, disc ESTR (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDateBGN,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsEuribor3MEstr,
		marketdata.BGNEstr,
		marketdata.BGNEuribor3M,
	)
	runIRSExample(
		"EUR IRS (BGN): fixed vs EURIBOR 6M, disc ESTR (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDateBGN,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsEuribor6MEstr,
		marketdata.BGNEstr,
		marketdata.BGNEuribor6M,
	)
	runIRSExample(
		"EUR IRS (LCH): fixed vs EURIBOR 3M, disc ESTR (LCH)",
		swap.DataSourceLCH,
		swap.ClearingHouseOTC,
		curveDateLCH,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsEuribor3MEstr,
		marketdata.LCHEstr,
		marketdata.LCHEuribor3M,
	)
	runIRSExample(
		"EUR IRS (LCH): fixed vs EURIBOR 6M, disc ESTR (LCH)",
		swap.DataSourceLCH,
		swap.ClearingHouseOTC,
		curveDateLCH,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsEuribor6MEstr,
		marketdata.LCHEstr,
		marketdata.LCHEuribor6M,
	)
}

// --------------------------------------------------------------------
// 3) JPY IRS – fixed vs TIBOR, discounted on TONAR OIS (BGN)
// --------------------------------------------------------------------

func exampleJPYIRS() {
	fmt.Println("3) JPY TIBOR IRS – fixed vs DTIBOR, discounted on TONAR OIS (BGN)")

	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC) // marketdata/fixtures_bgn_tibor.go
	notional := 1_000_000_000.0                                // JPY 1bn

	forwardTenorYears := 0
	swapTenorYears := 10

	fixedRatePercent := 1.50

	runIRSExample(
		"JPY IRS: fixed vs DTIBOR 3M (BGNS), disc TONAR OIS (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDate,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsTibor3MTonar,
		marketdata.BGNTonar,
		marketdata.BGNSTibor3M,
	)
	runIRSExample(
		"JPY IRS: fixed vs DTIBOR 6M (BGNS), disc TONAR OIS (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDate,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.IrsTibor6MTonar,
		marketdata.BGNTonar,
		marketdata.BGNSTibor6M,
	)
}

// --------------------------------------------------------------------
// 4) EUR OIS – fixed vs ESTR, projected/discounted on ESTR OIS
// --------------------------------------------------------------------

func exampleEUROIS() {
	fmt.Println("4) EUR OIS – fixed vs ESTR, discounted on ESTR OIS (BGN / LCH)")

	curveDateBGN := time.Date(2025, 6, 26, 0, 0, 0, 0, time.UTC)
	curveDateLCH := time.Date(2025, 9, 24, 0, 0, 0, 0, time.UTC)

	notional := 10_000_000.0 // EUR 10m
	forwardTenorYears := 0
	swapTenorYears := 5

	fixedRatePercent := 2.00

	runOISExample(
		"EUR OIS (BGN): fixed vs ESTR, disc ESTR (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDateBGN,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.OisEstr,
		marketdata.BGNEstr,
	)
	runOISExample(
		"EUR OIS (LCH): fixed vs ESTR, disc ESTR (LCH)",
		swap.DataSourceLCH,
		swap.ClearingHouseOTC,
		curveDateLCH,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.OisEstr,
		marketdata.LCHEstr,
	)
}

// --------------------------------------------------------------------
// 5) JPY OIS – fixed vs TONAR, projected/discounted on TONAR OIS (BGN)
// --------------------------------------------------------------------

func exampleJPYOIS() {
	fmt.Println("5) JPY OIS – fixed vs TONAR, discounted on TONAR OIS (BGN)")

	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	notional := 1_000_000_000.0 // JPY 1bn

	forwardTenorYears := 0
	swapTenorYears := 5

	fixedRatePercent := 1.25

	runOISExample(
		"JPY OIS (BGN): fixed vs TONAR, disc TONAR (BGN)",
		swap.DataSourceBGN,
		swap.ClearingHouseOTC,
		curveDate,
		forwardTenorYears,
		swapTenorYears,
		notional,
		fixedRatePercent,
		swaps.OisTonar,
		marketdata.BGNTonar,
	)

	// marketdata does not currently include an LCH TONAR fixture. If you add one
	// (e.g., scripts/generate_fixtures.py output for a TONAR curve), you can run:
	// runOISExample("JPY OIS (LCH): fixed vs TONAR, disc TONAR (LCH)", swap.DataSourceLCH, swap.ClearingHouseOTC, ...)
	// with the corresponding LCH quotes.
}

func runIRSExample(
	label string,
	dataSource swap.DataSource,
	clearingHouse swap.ClearingHouse,
	curveDate time.Time,
	forwardTenorYears int,
	swapTenorYears int,
	notional float64,
	fixedRatePercent float64,
	preset swaps.IRSPreset,
	oisQuotes map[string]float64,
	iborQuotes map[string]float64,
) {
	// Build a trade paying fixed and receiving floating.
	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        dataSource,
		ClearingHouse:     clearingHouse,
		CurveDate:         curveDate,
		TradeDate:         curveDate,
		ValuationDate:     curveDate,
		ForwardTenorYears: forwardTenorYears,
		SwapTenorYears:    swapTenorYears,
		Notional:          notional,
		PayLeg:            preset.FixedLeg,
		RecLeg:            preset.FloatLeg,
		DiscountingOIS:    preset.DiscountOIS,
		OISQuotes:         oisQuotes,
		RecLegQuotes:      iborQuotes,
		PayLegSpreadBP:    fixedRatePercent * 100.0, // fixed coupon in bp
	})
	if err != nil {
		panic(err)
	}

	pv, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}
	npv, err := trade.NPV()
	if err != nil {
		panic(err)
	}

	fmt.Printf("   %s\n", label)
	fmt.Printf("      Curve date:   %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("      Notional:     %.0f\n", notional)
	fmt.Printf("      Fixed rate:   %.4f%%\n", fixedRatePercent)
	fmt.Printf("      PV (fixed):   %12.2f\n", pv.PayLegPV)
	fmt.Printf("      PV (float):   %12.2f\n", pv.RecLegPV)
	fmt.Printf("      NPV:          %12.2f\n", npv)

	// Solve for the par fixed rate (pay-leg spread) so NPV = 0.
	parFixedBP, _, err := trade.SolveParSpread(swap.SpreadTargetPayLeg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("      Par fixed:    %.6f%% (NPV ~ 0)\n", parFixedBP/100.0)
	fmt.Println()
}

func runOISExample(
	label string,
	dataSource swap.DataSource,
	clearingHouse swap.ClearingHouse,
	curveDate time.Time,
	forwardTenorYears int,
	swapTenorYears int,
	notional float64,
	fixedRatePercent float64,
	preset swaps.OISPreset,
	oisQuotes map[string]float64,
) {
	// Build a trade paying fixed and receiving overnight floating.
	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        dataSource,
		ClearingHouse:     clearingHouse,
		CurveDate:         curveDate,
		TradeDate:         curveDate,
		ValuationDate:     curveDate,
		ForwardTenorYears: forwardTenorYears,
		SwapTenorYears:    swapTenorYears,
		Notional:          notional,
		PayLeg:            preset.FixedLeg,
		RecLeg:            preset.FloatLeg,
		DiscountingOIS:    preset.FloatLeg, // OIS discounting on the overnight curve itself
		OISQuotes:         oisQuotes,
		PayLegSpreadBP:    fixedRatePercent * 100.0,
	})
	if err != nil {
		panic(err)
	}

	pv, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}
	npv, err := trade.NPV()
	if err != nil {
		panic(err)
	}

	fmt.Printf("   %s\n", label)
	fmt.Printf("      Curve date:   %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("      Notional:     %.0f\n", notional)
	fmt.Printf("      Fixed rate:   %.4f%%\n", fixedRatePercent)
	fmt.Printf("      PV (fixed):   %12.2f\n", pv.PayLegPV)
	fmt.Printf("      PV (float):   %12.2f\n", pv.RecLegPV)
	fmt.Printf("      NPV:          %12.2f\n", npv)

	parFixedBP, _, err := trade.SolveParSpread(swap.SpreadTargetPayLeg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("      Par fixed:    %.6f%% (NPV ~ 0)\n", parFixedBP/100.0)
	fmt.Println()
}
