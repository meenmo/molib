package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	swaps "github.com/meenmo/molib/instruments/swaps"
	basisdata "github.com/meenmo/molib/marketdata"
	"github.com/meenmo/molib/swap"
	"github.com/meenmo/molib/swap/curve"
)

func main() {
	curveDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)

	// Use JPN calendar for TONAR / TIBOR curves.
	cal := calendar.JPN

	// Build TONAR OIS curve (discounting) and TIBOR6M dual curve.
	discCurve := curve.BuildCurve(curveDate, basisdata.BGNTonar, cal, 1)

	tiborLeg := swaps.TIBOR6MFloat
	projTibor6M := curve.BuildProjectionCurve(curveDate, tiborLeg, basisdata.BGNSTibor6M, discCurve)

	fmt.Println("=== Curve probe: JPY TONAR OIS and TIBOR6M on 2025-12-10 ===")
	fmt.Printf("Curve date: %s\n\n", curveDate.Format("2006-01-02"))

	// Key dates from the SWPM 5x5 export (pay / receive leg payment dates).
	keyDates := []time.Time{
		time.Date(2030, 12, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2031, 6, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2031, 12, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2032, 6, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2032, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2033, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2033, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2034, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2034, 12, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2035, 6, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2035, 12, 19, 0, 0, 0, 0, time.UTC),
	}

	fmt.Println("[TONAR OIS curve: discount factors and zero rates]")
	fmt.Println("Date, DF, ZeroRatePct")
	for _, d := range keyDates {
		df := discCurve.DF(d)
		z := discCurve.ZeroRateAt(d)
		fmt.Printf("%s,%.9f,%.9f\n", d.Format("01/02/2006"), df, z)
	}
	fmt.Println()

	// Now probe TIBOR6M forwards on the same 5x5 structure:
	// - Trade date: 2025-12-12
	// - Spot: T+2 on JPN calendar
	// - Effective: spot + 5Y, maturity: +5Y
	tradeDate := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	_, eff, mat := swap.SpotEffectiveMaturity(tradeDate, cal, 5, 5)
	payLeg := tiborLeg

	payPeriods, err := swap.GenerateSchedule(eff, mat, payLeg)
	if err != nil {
		panic(err)
	}
	payFwds, err := swap.GetForwardRates(projTibor6M, eff, mat, payLeg)
	if err != nil {
		panic(err)
	}

	fmt.Println("[TIBOR6M projection curve: forwards on 5x5 pay periods]")
	fmt.Println("ResetDate,AccrualStart,AccrualEnd,AccrualDays,FwdRatePct")
	for i, p := range payPeriods {
		fmt.Printf("%s,%s,%s,%.0f,%.5f\n",
			p.FixingDate.Format("01/02/2006"),
			p.StartDate.Format("01/02/2006"),
			p.EndDate.Format("01/02/2006"),
			float64(p.AccrualDays),
			payFwds[i].Rate*100.0,
		)
	}
}
