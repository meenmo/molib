package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
)

// This example shows how to use the molib basis pricer to
// solve for the fair receive-leg spread (NPV = 0) on a
// two-index basis swap, using the same machinery as cmd/basiscalc.
//
// It covers:
//   - JPY DTIBOR 3M / DTIBOR 6M, BGNS, discounted on TONAR OIS (BGN)
//   - EUR EURIBOR 3M / EURIBOR 6M, BGN, discounted on ESTR OIS (BGN)
//   - EUR EURIBOR 3M / EURIBOR 6M, LCH, discounted on ESTR OIS (LCH)
//
// Run with:
//
//   cd molib
//   go run ./examples/basis_swap_par_spread.go

func main() {
	fmt.Println("====================================================================")
	fmt.Println("Basis swap fair spread examples (molib)")
	fmt.Println("====================================================================")

	exampleSwap()
}

func exampleSwap() {
	// Common dates for this scenario.
	tradeDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	curveDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	forwardTenor := 10
	swapTenor := 10

	recLeg := swaps.EURIBOR3MFloat
	payLeg := swaps.EURIBOR6MFloat
	oisLeg := swaps.ESTRFloat

	oisQuotes := basisdata.BGNEstr
	payQuotes := basisdata.BGNEuribor6M
	recQuotes := basisdata.BGNEuribor3M

	notional := 10_000_000.0

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: forwardTenor,
		SwapTenorYears:    swapTenor,
		Notional:          notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisLeg,
		OISQuotes:         oisQuotes,
		PayLegQuotes:      payQuotes,
		RecLegQuotes:      recQuotes,
	})
	if err != nil {
		panic(err)
	}
	pv0, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}

	spreadBP, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		panic(err)
	}

	// Reconstruct dates for reporting.
	spot := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	eff := calendar.AdjustFollowing(oisLeg.Calendar, spot.AddDate(forwardTenor, 0, 0))
	mat := calendar.AdjustFollowing(oisLeg.Calendar, eff.AddDate(swapTenor, 0, 0))

	fmt.Println("   Configuration:")
	fmt.Printf("      Trade date:      %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("      Curve date:      %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("      Valuation date:  %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("      Spot date:       %s\n", spot.Format("2006-01-02"))
	fmt.Printf("      Effective date:  %s\n", eff.Format("2006-01-02"))
	fmt.Printf("      Maturity date:   %s\n", mat.Format("2006-01-02"))
	fmt.Printf("      Forward tenor:   %dy\n", forwardTenor)
	fmt.Printf("      Swap tenor:      %dy\n", swapTenor)
	fmt.Printf("      Pay leg:         EURIBOR6M, semiannual\n")
	fmt.Printf("      Rec leg:         EURIBOR3M, quarterly\n")
	fmt.Printf("      Discount curve:  ESTR OIS (BGN)\n")
	fmt.Printf("      Notional:        %.0f EUR\n", notional)
	fmt.Println()
	fmt.Printf("      NPV @ 0 spread:        %12.2f\n", pv0.TotalPV)
	fmt.Printf("      Fair rec 3M spread:    %.6f bp\n", spreadBP)
	fmt.Printf("      Pay-leg PV:            %12.2f\n", pv.PayLegPV)
	fmt.Printf("      Rec-leg PV:            %12.2f\n", pv.RecLegPV)
	fmt.Printf("      Total PV:              %12.2f (should be ~0 at fair spread)\n", pv.TotalPV)
}

func exampleSwap2() {
	// Common dates for this scenario.
	tradeDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	curveDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	forwardTenor := 5
	swapTenor := 5

	recLeg := swaps.TONARFloat
	payLeg := swaps.TIBOR6MFloat
	oisLeg := swaps.TonarFixedAnnual

	recQuotes := basisdata.BGNTonar
	payQuotes := basisdata.BGNSTibor6M
	oisQuotes := basisdata.BGNTonar

	notional := 10_000_000.0

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: forwardTenor,
		SwapTenorYears:    swapTenor,
		Notional:          notional,
		PayLeg:            payLeg,
		RecLeg:            recLeg,
		DiscountingOIS:    oisLeg,
		OISQuotes:         oisQuotes,
		PayLegQuotes:      payQuotes,
		RecLegQuotes:      recQuotes,
	})
	if err != nil {
		panic(err)
	}
	pv0, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}

	spreadBP, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		panic(err)
	}

	// Reconstruct dates for reporting.
	spot := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	eff := calendar.AdjustFollowing(oisLeg.Calendar, spot.AddDate(forwardTenor, 0, 0))
	mat := calendar.AdjustFollowing(oisLeg.Calendar, eff.AddDate(swapTenor, 0, 0))

	fmt.Println("   Configuration:")
	fmt.Printf("      Trade date:      %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("      Curve date:      %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("      Valuation date:  %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("      Spot date:       %s\n", spot.Format("2006-01-02"))
	fmt.Printf("      Effective date:  %s\n", eff.Format("2006-01-02"))
	fmt.Printf("      Maturity date:   %s\n", mat.Format("2006-01-02"))
	fmt.Printf("      Forward tenor:   %dy\n", forwardTenor)
	fmt.Printf("      Swap tenor:      %dy\n", swapTenor)
	fmt.Printf("      Pay leg:         EURIBOR6M, semiannual\n")
	fmt.Printf("      Rec leg:         EURIBOR3M, quarterly\n")
	fmt.Printf("      Discount curve:  ESTR OIS (BGN)\n")
	fmt.Printf("      Notional:        %.0f EUR\n", notional)
	fmt.Println()
	fmt.Printf("      NPV @ 0 spread:        %12.2f\n", pv0.TotalPV)
	fmt.Printf("      Fair rec 3M spread:    %.6f bp\n", spreadBP)
	fmt.Printf("      Pay-leg PV:            %12.2f\n", pv.PayLegPV)
	fmt.Printf("      Rec-leg PV:            %12.2f\n", pv.RecLegPV)
	fmt.Printf("      Total PV:              %12.2f (should be ~0 at fair spread)\n", pv.TotalPV)
}
