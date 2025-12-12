package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/basis"
	"github.com/meenmo/molib/swap/benchmark"
	"github.com/meenmo/molib/utils"
)

func main() {
	// 1. Setup Dates
	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	
	// JPY 5x5 details
	cal := calendar.JPN
	spotDate := calendar.AddBusinessDays(cal, tradeDate, 2)
	effective := calendar.AdjustFollowing(cal, spotDate.AddDate(5, 0, 0)) // 2030-12-16
	mature := calendar.AdjustFollowing(cal, effective.AddDate(5, 0, 0)) // 2035-12-17

	fmt.Printf("Diagnosis: JPY 5x5 Swap (Effective: %s, Maturity: %s)\n", effective.Format("2006-01-02"), mature.Format("2006-01-02"))

	// 2. Define SWPM Discount Factors (Placeholder - Replace with EXACT dump from SWPM)
	// These should include the exact dates required for valuation (reset/pay dates) if possible,
	// or a dense enough grid.
	// For this test, we use the ones we observed from our "improved" bootstrap as a baseline,
	// but the idea is to plug in the *external* DFs here.
	
	// Example: TONAR DFs (Discount Curve)

tonarDFs := map[time.Time]float64{
	curveDate: 1.0,
		// ... Add key pillars ...
		time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC): 0.995, // Dummy
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC): 0.933368552, // From our latest run
		time.Date(2035, 12, 17, 0, 0, 0, 0, time.UTC): 0.837722,    // From our latest run
	}

	// Example: TIBOR6M Pseudo-DFs (Projection Curve)
	// In dual curve, this curve is used to calculate forwards: F = (P_start/P_end - 1) / alpha
	tiborDFs := map[time.Time]float64{
	curveDate: 1.0,
		// ... Add key pillars ...
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC): 0.88, // Dummy
		time.Date(2035, 12, 17, 0, 0, 0, 0, time.UTC): 0.75, // Dummy
	}

	// 3. Build Curves from DFs
	discCurve := basis.NewCurveFromDFs(curveDate, tonarDFs, cal, 1)
	projCurve := basis.NewCurveFromDFs(curveDate, tiborDFs, cal, 6)

	// 4. Price the Swap
	// Pay Leg: TIBOR 6M (Float)
	// Rec Leg: TONAR (Float + Spread)
	// Notional: 10MM
	
	// Re-construct the trade legs
	payLegPreset := benchmark.TIBOR6MFloat
	
	// We need a helper to price it. We can reuse the logic from the other diag scripts
	// or simplified here. Let's do a simplified check of the Forward Rate at start.
	
fwd := forwardRate(projCurve, effective, payLegPreset)
	fmt.Printf("Forward Rate @ Effective (%s): %.6f%%\n", effective.Format("2006-01-02"), fwd*100)
	
dfStart := discCurve.DF(effective)
	fmt.Printf("Discount Factor @ Effective: %.9f\n", dfStart)
	
	fmt.Println("\nTo use this script effectively, update 'tonarDFs' and 'tiborDFs' maps with full SWPM export data.")
}

func forwardRate(c *basis.Curve, start time.Time, leg benchmark.LegConvention) float64 {
	end := calendar.Adjust(leg.Calendar, start.AddDate(0, int(leg.PayFrequency), 0))
	df1 := c.DF(start)
	df2 := c.DF(end)
	acc := utils.YearFraction(start, end, string(leg.DayCount))
	return (df1/df2 - 1) / acc
}
