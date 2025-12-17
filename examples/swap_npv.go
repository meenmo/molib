package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/instruments/swaps"
	"github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/utils"
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
	fmt.Println("Swap NPV examples (molib)")
	fmt.Println("====================================================================")
	fmt.Println()

	exampleSwap()
}

func exampleSwap() {
	fmt.Println("JPY Basis Swap: Pay TIBOR6M, Rec TONAR + 52bp, Disc TONAR")

	curveDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 17, 0, 0, 0, 0, time.UTC)
	valuationDate := tradeDate

	forwardTenorYears := 5
	swapTenorYears := 5
	notional := 5000000000.0
	recSpreadBP := 52.0

	trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
		DataSource:        swap.DataSourceBGN,
		ClearingHouse:     swap.ClearingHouseOTC,
		CurveDate:         curveDate,
		TradeDate:         tradeDate,
		ValuationDate:     valuationDate,
		ForwardTenorYears: forwardTenorYears,
		SwapTenorYears:    swapTenorYears,
		Notional:          notional,
		PayLeg:            swaps.TIBOR6MFloat,
		RecLeg:            swaps.TONARFloat,
		DiscountingOIS:    swaps.TONARFloat,
		OISQuotes:         marketdata.BGNTonar,
		PayLegQuotes:      marketdata.BGNSTibor6M,
		RecLegQuotes:      marketdata.BGNTonar,
		RecLegSpreadBP:    recSpreadBP,
	})
	if err != nil {
		panic(err)
	}

	pv, err := trade.PVByLeg()
	if err != nil {
		panic(err)
	}

	fmt.Printf("   Curve date:      %s\n", curveDate.Format("2006-01-02"))
	fmt.Printf("   Trade date:      %s\n", tradeDate.Format("2006-01-02"))
	fmt.Printf("   Valuation date:  %s\n", valuationDate.Format("2006-01-02"))
	fmt.Printf("   Effective:       %s\n", trade.Spec.EffectiveDate.Format("2006-01-02"))
	fmt.Printf("   Maturity:        %s\n", trade.Spec.MaturityDate.Format("2006-01-02"))
	fmt.Printf("   Notional:        %.0f JPY\n", notional)
	fmt.Printf("   Pay leg:         TIBOR6M (flat)\n")
	fmt.Printf("   Rec leg:         TONAR + %.0f bp\n", recSpreadBP)
	fmt.Printf("   Discounting:     TONAR\n")
	fmt.Println()
	fmt.Printf("   PV (pay leg):    %12.2f JPY\n", pv.PayLegPV)
	fmt.Printf("   PV (rec leg):    %12.2f JPY\n", pv.RecLegPV)
	fmt.Printf("   NPV:             %12.2f JPY\n", pv.TotalPV)

	// Display cashflows by period
	fmt.Println()
	fmt.Println("   === Pay Leg (TIBOR6M) Cashflows ===")
	fmt.Printf("   %-3s | %-10s | %-10s | %-10s | %4s | %9s | %10s | %8s | %12s\n",
		"#", "PayDate", "AccrStart", "AccrEnd", "Days", "FwdRate%", "Cashflow", "DF", "PV")
	fmt.Println("   ----+------------+------------+------------+------+-----------+------------+----------+-------------")

	paySchedule, _ := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	payFwds, _ := swap.GetForwardRates(trade.PayProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	payDates := make([]time.Time, len(paySchedule))
	for i, p := range paySchedule {
		payDates[i] = p.PayDate
	}
	payDFs, _ := swap.GetDiscountFactors(trade.DiscountCurve, payDates)

	var payTotal float64
	for i, p := range paySchedule {
		fwd := payFwds[i]
		df := payDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.PayLeg.DayCount))
		cashflow := -notional * fwd.Rate * yearFrac
		pvCF := cashflow * df
		payTotal += pvCF
		fmt.Printf("   %-3d | %s | %s | %s | %4d | %9.5f | %10.0f | %8.6f | %12.0f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			fwd.Rate*100,
			cashflow,
			df,
			pvCF,
		)
	}
	fmt.Printf("   %s\n", "                                                                              -------------")
	fmt.Printf("   %s %12.0f\n", "                                                            Coupon PV:", payTotal)

	// Add principal exchanges for pay leg
	dfEff := trade.DiscountCurve.DF(trade.Spec.EffectiveDate)
	dfMat := trade.DiscountCurve.DF(trade.Spec.MaturityDate)
	payInitPrincipal := notional * dfEff   // receive notional at start
	payFinalPrincipal := -notional * dfMat // pay notional at end
	fmt.Printf("   %s %12.0f  (DF=%.6f)\n", "                                                      Initial Principal:", payInitPrincipal, dfEff)
	fmt.Printf("   %s %12.0f  (DF=%.6f)\n", "                                                        Final Principal:", payFinalPrincipal, dfMat)
	fmt.Printf("   %s\n", "                                                                              -------------")
	fmt.Printf("   %s %12.0f\n", "                                                             Pay Leg PV:", payTotal+payInitPrincipal+payFinalPrincipal)

	fmt.Println()
	fmt.Println("   === Rec Leg (TONAR + spread) Cashflows ===")
	fmt.Printf("   %-3s | %-10s | %-10s | %-10s | %4s | %9s | %10s | %8s | %12s\n",
		"#", "PayDate", "AccrStart", "AccrEnd", "Days", "FwdRate%", "Cashflow", "DF", "PV")
	fmt.Println("   ----+------------+------------+------------+------+-----------+------------+----------+-------------")

	recSchedule, _ := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)
	recFwds, _ := swap.GetForwardRates(trade.RecProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)
	recDates := make([]time.Time, len(recSchedule))
	for i, p := range recSchedule {
		recDates[i] = p.PayDate
	}
	recDFs, _ := swap.GetDiscountFactors(trade.DiscountCurve, recDates)

	spread := recSpreadBP / 10000.0
	var recTotal float64
	for i, p := range recSchedule {
		fwd := recFwds[i]
		df := recDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.RecLeg.DayCount))
		rate := fwd.Rate + spread
		cashflow := notional * rate * yearFrac
		pvCF := cashflow * df
		recTotal += pvCF
		fmt.Printf("   %-3d | %s | %s | %s | %4d | %9.5f | %10.0f | %8.6f | %12.0f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			rate*100,
			cashflow,
			df,
			pvCF,
		)
	}
	fmt.Printf("   %s\n", "                                                                              -------------")
	fmt.Printf("   %s %12.0f\n", "                                                            Coupon PV:", recTotal)

	// Add principal exchanges for rec leg (opposite sign from pay leg)
	recInitPrincipal := -notional * dfEff // pay notional at start
	recFinalPrincipal := notional * dfMat // receive notional at end
	fmt.Printf("   %s %12.0f  (DF=%.6f)\n", "                                                      Initial Principal:", recInitPrincipal, dfEff)
	fmt.Printf("   %s %12.0f  (DF=%.6f)\n", "                                                        Final Principal:", recFinalPrincipal, dfMat)
	fmt.Printf("   %s\n", "                                                                              -------------")
	fmt.Printf("   %s %12.0f\n", "                                                             Rec Leg PV:", recTotal+recInitPrincipal+recFinalPrincipal)
}
