package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/curve"
)

func main() {
	// This diagnostic is intended to be a "DF injection harness":
	// - Inject discount factors from an external source (SWPM/ficclib),
	// - Inject projection pseudo-DFs (or a dense enough grid) similarly,
	// - Re-price a specific swap and compare.
	//
	// The current version is a scaffold: replace the DF maps with full exports
	// to make it a meaningful validation.

	// 1) Setup dates
	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)

	// 2) JPY 5x5 details
	cal := calendar.JPN
	spotDate := calendar.AddBusinessDays(cal, tradeDate, 2)
	effective := calendar.AdjustFollowing(cal, spotDate.AddDate(5, 0, 0))
	mature := calendar.AdjustFollowing(cal, effective.AddDate(5, 0, 0))

	fmt.Printf("Diagnosis: JPY 5x5 Swap (Effective: %s, Maturity: %s)\n", effective.Format("2006-01-02"), mature.Format("2006-01-02"))

	// 3) Define external discount factors (placeholder)
	// Replace with exact SWPM DFs at cashflow payment dates (and ideally accrual boundaries).

	tonarDFs := map[time.Time]float64{
		curveDate: 1.0,
		// Dummy node examples only.
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC): 0.93,
		time.Date(2035, 12, 17, 0, 0, 0, 0, time.UTC): 0.84,
	}

	// 4) Define external projection pseudo-DFs (placeholder)
	// In dual-curve valuation, this curve is used only through DF ratios to compute forwards:
	// forward = (P(start)/P(end) - 1) / alpha.
	//
	// For a meaningful test, provide pseudo-DFs at all accrual boundaries in the swap schedule,
	// or use a dense enough grid and let interpolation fill the gaps.
	tiborDFs := map[time.Time]float64{
		curveDate: 1.0,
		// Dummy node examples only.
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC): 1.0,
		time.Date(2031, 6, 16, 0, 0, 0, 0, time.UTC):  0.99,
		time.Date(2035, 12, 17, 0, 0, 0, 0, time.UTC): 0.92,
	}

	// 5) Build curves from injected DFs
	discCurve := curve.NewCurveFromDFs(curveDate, tonarDFs, cal, 0)
	projCurve := curve.NewCurveFromDFs(curveDate, tiborDFs, cal, 0)

	// 6) Minimal sanity prints
	fmt.Printf("Injected DF @ effective: %.9f\n", discCurve.DF(effective))
	fmt.Printf("Injected DF @ maturity:  %.9f\n", discCurve.DF(mature))
	fmt.Printf("Example projection DF ratio (effective->maturity): %.9f\n", projCurve.DF(effective)/projCurve.DF(mature))

	fmt.Println()
	fmt.Println("Next step: replace the DF maps with full SWPM exports and re-run pricing diagnostics.")
}
