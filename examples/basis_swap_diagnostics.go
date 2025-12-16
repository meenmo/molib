package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/utils"
)

// This diagnostic tool prints detailed cashflow information for the TONAR vs TIBOR6M basis swap.

func main() {
	fmt.Println("====================================================================")
	fmt.Println("Basis Swap Diagnostics: TONAR vs TIBOR6M")
	fmt.Println("====================================================================")

	tradeDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	curveDate := time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	forwardTenor := 10
	swapTenor := 30

	recLeg := swaps.TONARFloat
	payLeg := swaps.TIBOR6MFloat
	oisLeg := swaps.TONARFloat

	oisQuotes := basisdata.BGNTonar
	payQuotes := basisdata.BGNSTibor6M
	recQuotes := basisdata.BGNTonar

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

	spot := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	eff := calendar.AdjustFollowing(oisLeg.Calendar, spot.AddDate(forwardTenor, 0, 0))
	mat := calendar.AdjustFollowing(oisLeg.Calendar, eff.AddDate(swapTenor, 0, 0))

	fmt.Println("\n=== Trade Configuration ===")
	fmt.Printf("Trade date:      %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("Curve date:      %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("Valuation date:  %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("Spot date:       %s\n", spot.Format("2006-01-02"))
	fmt.Printf("Effective date:  %s\n", eff.Format("2006-01-02"))
	fmt.Printf("Maturity date:   %s\n", mat.Format("2006-01-02"))
	fmt.Printf("Notional:        %.0f JPY\n", notional)

	// Generate schedules
	paySchedule, err := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	if err != nil {
		panic(err)
	}
	recSchedule, err := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nPay leg (TIBOR6M): %d periods\n", len(paySchedule))
	fmt.Printf("Rec leg (TONAR):   %d periods\n", len(recSchedule))

	// Get forward rates
	payFwds, err := swap.GetForwardRates(trade.PayProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	if err != nil {
		panic(err)
	}
	recFwds, err := swap.GetForwardRates(trade.RecProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)
	if err != nil {
		panic(err)
	}

	// Get discount factors
	payDates := make([]time.Time, len(paySchedule))
	for i, p := range paySchedule {
		payDates[i] = p.PayDate
	}
	recDates := make([]time.Time, len(recSchedule))
	for i, p := range recSchedule {
		recDates[i] = p.PayDate
	}

	payDFs, err := swap.GetDiscountFactors(trade.DiscountCurve, payDates)
	if err != nil {
		panic(err)
	}
	recDFs, err := swap.GetDiscountFactors(trade.DiscountCurve, recDates)
	if err != nil {
		panic(err)
	}

	// Show first 5 periods of each leg
	fmt.Println("\n=== Pay Leg (TIBOR6M) - First 5 Periods ===")
	fmt.Println("Period | PayDate    | StartDate  | EndDate    | Days | FwdRate(%) | DF       | Notional   | Payment    | PV")
	fmt.Println("-------|------------|------------|------------|------|------------|----------|------------|------------|------------")
	for i := 0; i < min(5, len(paySchedule)); i++ {
		p := paySchedule[i]
		fwd := payFwds[i]
		df := payDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.PayLeg.DayCount))
		payment := -notional * fwd.Rate * yearFrac
		pv := payment * df
		fmt.Printf("%-6d | %s | %s | %s | %4d | %10.6f | %.6f | %10.0f | %10.2f | %10.2f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			fwd.Rate*100,
			df,
			-notional,
			payment,
			pv,
		)
	}

	fmt.Println("\n=== Rec Leg (TONAR) - First 5 Periods ===")
	fmt.Println("Period | PayDate    | StartDate  | EndDate    | Days | FwdRate(%) | DF       | Notional   | Payment    | PV")
	fmt.Println("-------|------------|------------|------------|------|------------|----------|------------|------------|------------")
	for i := 0; i < min(5, len(recSchedule)); i++ {
		p := recSchedule[i]
		fwd := recFwds[i]
		df := recDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.RecLeg.DayCount))
		payment := notional * fwd.Rate * yearFrac
		pv := payment * df
		fmt.Printf("%-6d | %s | %s | %s | %4d | %10.6f | %.6f | %10.0f | %10.2f | %10.2f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			fwd.Rate*100,
			df,
			notional,
			payment,
			pv,
		)
	}

	// Calculate and show NPV breakdown
	pv0, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== NPV @ 0 Spread ===")
	fmt.Printf("Pay leg PV:  %12.2f JPY\n", pv0.PayLegPV)
	fmt.Printf("Rec leg PV:  %12.2f JPY\n", pv0.RecLegPV)
	fmt.Printf("Total NPV:   %12.2f JPY\n", pv0.TotalPV)

	// Solve for fair spread
	spreadBP, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Fair Spread (NPV = 0) ===")
	fmt.Printf("Fair rec spread: %.6f bp\n", spreadBP)
	fmt.Printf("Pay leg PV:      %12.2f JPY\n", pv.PayLegPV)
	fmt.Printf("Rec leg PV:      %12.2f JPY\n", pv.RecLegPV)
	fmt.Printf("Total NPV:       %12.2f JPY (should be ~0)\n", pv.TotalPV)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
