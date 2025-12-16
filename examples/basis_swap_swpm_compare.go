package main

import (
	"fmt"
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/utils"
)

// SWPM Reference Data for TONAR vs TIBOR6M (10x30 basis swap)
// Receive: TONAR (annual), Pay: TIBOR6M (semi-annual)

type SWPMCashflow struct {
	PayDate      string
	AccrualStart string
	AccrualEnd   string
	AccrualDays  int
	EquivCoupon  float64 // in percent
	Payment      float64
	Discount     float64
	ZeroRate     float64
	PV           float64
}

// SWPM Receive Leg (TONAR, annual) - first few periods
var swpmRecLeg = []SWPMCashflow{
	{"12/22/2036", "12/18/2035", "12/18/2036", 366, 3.29566, 330469, 0.814069, 1.865909, 269024},
	{"12/22/2037", "12/18/2036", "12/18/2037", 365, 3.52965, 352965, 0.790658, 1.953404, 279074},
	{"12/22/2038", "12/18/2037", "12/20/2038", 367, 3.78253, 380325, 0.766035, 2.046332, 291342},
	{"12/21/2039", "12/20/2038", "12/19/2039", 364, 3.78419, 377382, 0.742243, 2.125804, 280109},
	{"12/20/2040", "12/19/2039", "12/18/2040", 365, 3.78433, 378432, 0.719118, 2.194994, 272137},
}

// SWPM Pay Leg (TIBOR6M, semi-annual) - first few periods
var swpmPayLeg = []SWPMCashflow{
	{"06/20/2036", "12/18/2035", "06/18/2036", 183, 3.40101, -170516, 0.825258, 1.826037, -140720},
	{"12/22/2036", "06/18/2036", "12/18/2036", 183, 3.40101, -170516, 0.814069, 1.865909, -138812},
	{"06/22/2037", "12/18/2036", "06/18/2037", 182, 3.40085, -169576, 0.802313, 1.911406, -136053},
	{"12/22/2037", "06/18/2037", "12/18/2037", 183, 3.40289, -170610, 0.790658, 1.953404, -134894},
	{"06/22/2038", "12/18/2037", "06/18/2038", 182, 3.74576, -186774, 0.778283, 2.001596, -145363},
	{"12/22/2038", "06/18/2038", "12/20/2038", 185, 3.74633, -189882, 0.766035, 2.046332, -145456},
	{"06/22/2039", "12/20/2038", "06/20/2039", 182, 3.74576, -186774, 0.754045, 2.087533, -140836},
	{"12/21/2039", "06/20/2039", "12/19/2039", 182, 3.74576, -186774, 0.742243, 2.125804, -138632},
	{"06/20/2040", "12/19/2039", "06/18/2040", 182, 3.74576, -186774, 0.730626, 2.161447, -136462},
	{"12/20/2040", "06/18/2040", "12/18/2040", 183, 3.74910, -187968, 0.719118, 2.194994, -135171},
}

func main() {
	fmt.Println("====================================================================")
	fmt.Println("SWPM vs molib Row-by-Row Comparison: TONAR vs TIBOR6M (10x30)")
	fmt.Println("====================================================================")
	fmt.Println()
	fmt.Println("SWPM Spread: 57.000 bp")
	fmt.Println("molib Spread: 57.804 bp")
	fmt.Println("Difference: 0.804 bp")
	fmt.Println()

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

	// Generate schedules
	paySchedule, _ := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	recSchedule, _ := swap.GenerateSchedule(trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)

	// Get forward rates
	payFwds, _ := swap.GetForwardRates(trade.PayProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.PayLeg)
	recFwds, _ := swap.GetForwardRates(trade.RecProjCurve, trade.Spec.EffectiveDate, trade.Spec.MaturityDate, trade.Spec.RecLeg)

	// Get discount factors
	payDates := make([]time.Time, len(paySchedule))
	for i, p := range paySchedule {
		payDates[i] = p.PayDate
	}
	recDates := make([]time.Time, len(recSchedule))
	for i, p := range recSchedule {
		recDates[i] = p.PayDate
	}
	payDFs, _ := swap.GetDiscountFactors(trade.DiscountCurve, payDates)
	recDFs, _ := swap.GetDiscountFactors(trade.DiscountCurve, recDates)

	// ===== PAY LEG COMPARISON (TIBOR6M) =====
	fmt.Println("===== PAY LEG (TIBOR6M) - First 10 Periods =====")
	fmt.Println()
	fmt.Printf("%-3s | %-10s | %-10s | %-10s | %-4s | %-4s | %-9s | %-9s | %-8s | %-8s | %-10s | %-10s | %-10s | %-10s\n",
		"#", "PayDate", "AccrStart", "AccrEnd", "Days", "SWPM", "FwdRate%", "SWPM%", "DF", "SWPM DF", "Payment", "SWPM Pmt", "PV", "SWPM PV")
	fmt.Println("----+------------+------------+------------+------+------+-----------+-----------+----------+----------+------------+------------+------------+------------")

	for i := 0; i < min(10, len(paySchedule)); i++ {
		p := paySchedule[i]
		fwd := payFwds[i]
		df := payDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.PayLeg.DayCount))
		payment := -notional * fwd.Rate * yearFrac
		pv := payment * df

		swpmDays := 0
		swpmFwd := 0.0
		swpmDF := 0.0
		swpmPmt := 0.0
		swpmPV := 0.0
		if i < len(swpmPayLeg) {
			swpmDays = swpmPayLeg[i].AccrualDays
			swpmFwd = swpmPayLeg[i].EquivCoupon
			swpmDF = swpmPayLeg[i].Discount
			swpmPmt = swpmPayLeg[i].Payment
			swpmPV = swpmPayLeg[i].PV
		}

		fmt.Printf("%-3d | %s | %s | %s | %4d | %4d | %9.5f | %9.5f | %8.6f | %8.6f | %10.0f | %10.0f | %10.0f | %10.0f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			swpmDays,
			fwd.Rate*100,
			swpmFwd,
			df,
			swpmDF,
			payment,
			swpmPmt,
			pv,
			swpmPV,
		)
	}

	// ===== REC LEG COMPARISON (TONAR) =====
	fmt.Println()
	fmt.Println("===== REC LEG (TONAR) - First 5 Periods =====")
	fmt.Println()
	fmt.Printf("%-3s | %-10s | %-10s | %-10s | %-4s | %-4s | %-9s | %-9s | %-8s | %-8s | %-10s | %-10s | %-10s | %-10s\n",
		"#", "PayDate", "AccrStart", "AccrEnd", "Days", "SWPM", "FwdRate%", "SWPM%", "DF", "SWPM DF", "Payment", "SWPM Pmt", "PV", "SWPM PV")
	fmt.Println("----+------------+------------+------------+------+------+-----------+-----------+----------+----------+------------+------------+------------+------------")

	for i := 0; i < min(5, len(recSchedule)); i++ {
		p := recSchedule[i]
		fwd := recFwds[i]
		df := recDFs[i]
		yearFrac := utils.YearFraction(p.StartDate, p.EndDate, string(trade.Spec.RecLeg.DayCount))
		payment := notional * fwd.Rate * yearFrac
		pv := payment * df

		swpmDays := 0
		swpmFwd := 0.0
		swpmDF := 0.0
		swpmPmt := 0.0
		swpmPV := 0.0
		if i < len(swpmRecLeg) {
			swpmDays = swpmRecLeg[i].AccrualDays
			swpmFwd = swpmRecLeg[i].EquivCoupon
			swpmDF = swpmRecLeg[i].Discount
			swpmPmt = swpmRecLeg[i].Payment
			swpmPV = swpmRecLeg[i].PV
		}

		fmt.Printf("%-3d | %s | %s | %s | %4d | %4d | %9.5f | %9.5f | %8.6f | %8.6f | %10.0f | %10.0f | %10.0f | %10.0f\n",
			i+1,
			p.PayDate.Format("2006-01-02"),
			p.StartDate.Format("2006-01-02"),
			p.EndDate.Format("2006-01-02"),
			p.AccrualDays,
			swpmDays,
			fwd.Rate*100,
			swpmFwd,
			df,
			swpmDF,
			payment,
			swpmPmt,
			pv,
			swpmPV,
		)
	}

	// ===== KEY DIFFERENCES ANALYSIS =====
	fmt.Println()
	fmt.Println("===== KEY DIFFERENCES ANALYSIS =====")
	fmt.Println()

	// Compare first pay leg period in detail
	if len(paySchedule) > 0 && len(swpmPayLeg) > 0 {
		p := paySchedule[0]
		fwd := payFwds[0]
		df := payDFs[0]
		swpm := swpmPayLeg[0]

		fmt.Println("Pay Leg Period 1 Analysis:")
		fmt.Printf("  PayDate:      molib=%s, SWPM=%s\n", p.PayDate.Format("2006-01-02"), swpm.PayDate)
		fmt.Printf("  AccrualDays:  molib=%d, SWPM=%d, diff=%d\n", p.AccrualDays, swpm.AccrualDays, p.AccrualDays-swpm.AccrualDays)
		fmt.Printf("  FwdRate:      molib=%.6f%%, SWPM=%.6f%%, diff=%.6f%%\n", fwd.Rate*100, swpm.EquivCoupon, fwd.Rate*100-swpm.EquivCoupon)
		fmt.Printf("  DF:           molib=%.6f, SWPM=%.6f, diff=%.6f\n", df, swpm.Discount, df-swpm.Discount)
	}

	// Compare first rec leg period in detail
	if len(recSchedule) > 0 && len(swpmRecLeg) > 0 {
		p := recSchedule[0]
		fwd := recFwds[0]
		df := recDFs[0]
		swpm := swpmRecLeg[0]

		fmt.Println()
		fmt.Println("Rec Leg Period 1 Analysis:")
		fmt.Printf("  PayDate:      molib=%s, SWPM=%s\n", p.PayDate.Format("2006-01-02"), swpm.PayDate)
		fmt.Printf("  AccrualDays:  molib=%d, SWPM=%d, diff=%d\n", p.AccrualDays, swpm.AccrualDays, p.AccrualDays-swpm.AccrualDays)
		fmt.Printf("  FwdRate:      molib=%.6f%%, SWPM=%.6f%%, diff=%.6f%%\n", fwd.Rate*100, swpm.EquivCoupon, fwd.Rate*100-swpm.EquivCoupon)
		fmt.Printf("  DF:           molib=%.6f, SWPM=%.6f, diff=%.6f\n", df, swpm.Discount, df-swpm.Discount)
	}

	// Check pay delay settings
	fmt.Println()
	fmt.Println("===== LEG CONVENTIONS =====")
	fmt.Printf("Pay Leg (TIBOR6M):\n")
	fmt.Printf("  PayDelayDays: %d\n", trade.Spec.PayLeg.PayDelayDays)
	fmt.Printf("  DayCount:     %s\n", trade.Spec.PayLeg.DayCount)
	fmt.Printf("  PayFrequency: %s\n", trade.Spec.PayLeg.PayFrequency)
	fmt.Printf("Rec Leg (TONAR):\n")
	fmt.Printf("  PayDelayDays: %d\n", trade.Spec.RecLeg.PayDelayDays)
	fmt.Printf("  DayCount:     %s\n", trade.Spec.RecLeg.DayCount)
	fmt.Printf("  PayFrequency: %s\n", trade.Spec.RecLeg.PayFrequency)

	// Final spread comparison
	spreadBP, pv, _ := trade.SolveParSpread(swap.SpreadTargetRecLeg)
	fmt.Println()
	fmt.Println("===== FINAL SPREAD COMPARISON =====")
	fmt.Printf("molib fair spread: %.6f bp\n", spreadBP)
	fmt.Printf("SWPM fair spread:  57.000000 bp\n")
	fmt.Printf("Difference:        %.6f bp\n", spreadBP-57.0)
	fmt.Printf("NPV at fair spread: %.2f (should be ~0)\n", pv.TotalPV)

	// Analyze DF curve
	fmt.Println()
	fmt.Println("===== DISCOUNT FACTOR CURVE COMPARISON =====")
	spot := calendar.AddBusinessDays(oisLeg.Calendar, tradeDate, 2)
	testDates := []time.Time{
		spot.AddDate(10, 0, 0),  // 10Y
		spot.AddDate(20, 0, 0),  // 20Y
		spot.AddDate(30, 0, 0),  // 30Y
		spot.AddDate(40, 0, 0),  // 40Y
	}
	dfs, _ := swap.GetDiscountFactors(trade.DiscountCurve, testDates)
	for i, d := range testDates {
		fmt.Printf("  DF(%s) = %.6f\n", d.Format("2006-01-02"), dfs[i])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper to suppress unused import error
var _ = math.Abs
