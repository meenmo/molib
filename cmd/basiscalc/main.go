package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/meenmo/molib/swap/basis"
	"github.com/meenmo/molib/swap/basis/data"
	"github.com/meenmo/molib/swap/benchmark"
)

func main() {
	// Parse command line arguments
	dateStr := flag.String("date", "", "Curve date in YYYYMMDD format (e.g., 20251121)")
	flag.Parse()

	// Parse date if provided, otherwise use default
	var curveDate time.Time
	if *dateStr != "" {
		parsedDate, err := time.Parse("20060102", *dateStr)
		if err != nil {
			fmt.Printf("Error parsing date '%s': %v\n", *dateStr, err)
			fmt.Println("Date must be in YYYYMMDD format (e.g., 20251121)")
			return
		}
		curveDate = parsedDate
	} else {
		// Default date if not specified
		curveDate = time.Date(2025, 11, 21, 0, 0, 0, 0, time.UTC)
	}

	runBGNEUR(curveDate)
	runBGNTibor(curveDate)
	runLCHEUR(curveDate)
}

func runBGNEUR(curveDate time.Time) {
	tenorPairs := [][2]int{{10, 10}, {10, 20}}
	for _, tp := range tenorPairs {
		spread, pv := basis.CalculateSpread(
			curveDate,
			tp[0],
			tp[1],
			benchmark.EURIBOR6MFloat,
			benchmark.EURIBOR3MFloat,
			benchmark.ESTRFloat,
			data.BGNEstr,
			data.BGNEuribor6M,
			data.BGNEuribor3M,
			10_000_000.0,
		)
		fmt.Printf("BGN %dx%d spread=%.6f bp, NPV=%.2f\n", tp[0], tp[1], spread, pv.TotalPV)
	}
}

func runBGNTibor(curveDate time.Time) {
	tenorPairs := [][2]int{{1, 4}, {2, 3}}
	for _, tp := range tenorPairs {
		spread, pv := basis.CalculateSpread(
			curveDate,
			tp[0],
			tp[1],
			benchmark.TIBOR6MFloat,
			benchmark.TIBOR3MFloat,
			benchmark.TONARFloat,
			data.BGNTonar,
			data.BGNTibor6M,
			data.BGNTibor3M,
			10_000_000.0,
		)
		fmt.Printf("BGNS %dx%d spread=%.6f bp, NPV=%.2f\n", tp[0], tp[1], spread, pv.TotalPV)
	}
}

func runLCHEUR(curveDate time.Time) {
	tenorPairs := [][2]int{{10, 10}, {10, 20}}
	for _, tp := range tenorPairs {
		spread, pv := basis.CalculateSpread(
			curveDate,
			tp[0],
			tp[1],
			benchmark.EURIBOR6MFloat,
			benchmark.EURIBOR3MFloat,
			benchmark.ESTRFloat,
			data.LCHEstr,
			data.LCHEuribor6M,
			data.LCHEuribor3M,
			10_000_000.0,
		)
		fmt.Printf("LCH %dx%d spread=%.6f bp, NPV=%.2f\n", tp[0], tp[1], spread, pv.TotalPV)
	}
}
