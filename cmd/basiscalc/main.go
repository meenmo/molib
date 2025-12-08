package main

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	"github.com/meenmo/molib/db"
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

	// Connect to database
	dbConn, err := db.Connect(db.DefaultConfig())
	if err != nil {
		fmt.Printf("Warning: Could not connect to database: %v\n", err)
		fmt.Println("Running without database comparison")
		dbConn = nil
	} else {
		defer dbConn.Close()
	}

	runBGNEUR(curveDate, dbConn)
	runBGNTibor(curveDate, dbConn)
	runLCHEUR(curveDate, dbConn)
}

func runBGNEUR(curveDate time.Time, dbConn *sql.DB) {
	tenorPairs := [][2]int{{10, 10}, {10, 20}}
	for _, tp := range tenorPairs {
		spread, _ := basis.CalculateSpread(
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

		// Query database for expected spread
		output := fmt.Sprintf("BGN EURIBOR3M/EURIBOR6M %dx%d computed=%.6f bp", tp[0], tp[1], spread)
		if dbConn != nil {
			params := db.BasisSwapParams{
				ValuationDate:     curveDate,
				ForwardTenor:      tp[0],
				SwapTenor:         tp[1],
				RecLegIndex:       "EURIBOR3M",
				PayLegIndex:       "EURIBOR6M",
				PayLegSource:      "BGN",
				RecLegSource:      "BGN",
				DiscountingSource: "BGN",
			}
			if dbSpread, err := db.GetBasisSwapSpread(dbConn, params); err == nil {
				diff := spread - dbSpread
				output += fmt.Sprintf(" | database=%.6f bp | diff=%.6f bp", dbSpread, diff)
			}
		}
		fmt.Println(output)
	}
}

func runBGNTibor(curveDate time.Time, dbConn *sql.DB) {
	tenorPairs := [][2]int{{1, 4}, {2, 3}}
	for _, tp := range tenorPairs {
		spread, _ := basis.CalculateSpread(
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

		// Query database for expected spread
		output := fmt.Sprintf("BGNS TIBOR3M/TIBOR6M %dx%d computed=%.6f bp", tp[0], tp[1], spread)
		if dbConn != nil {
			params := db.BasisSwapParams{
				ValuationDate:     curveDate,
				ForwardTenor:      tp[0],
				SwapTenor:         tp[1],
				RecLegIndex:       "TIBOR3M",
				PayLegIndex:       "TIBOR6M",
				PayLegSource:      "BGNS",
				RecLegSource:      "BGNS",
				DiscountingSource: "BGN",
			}
			if dbSpread, err := db.GetBasisSwapSpread(dbConn, params); err == nil {
				diff := spread - dbSpread
				output += fmt.Sprintf(" | database=%.6f bp | diff=%.6f bp", dbSpread, diff)
			}
		}
		fmt.Println(output)
	}
}

func runLCHEUR(curveDate time.Time, dbConn *sql.DB) {
	tenorPairs := [][2]int{{10, 10}, {10, 20}}
	for _, tp := range tenorPairs {
		spread, _ := basis.CalculateSpread(
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

		// Query database for expected spread
		output := fmt.Sprintf("LCH EURIBOR3M/EURIBOR6M %dx%d computed=%.6f bp", tp[0], tp[1], spread)
		if dbConn != nil {
			params := db.BasisSwapParams{
				ValuationDate:     curveDate,
				ForwardTenor:      tp[0],
				SwapTenor:         tp[1],
				RecLegIndex:       "EURIBOR3M",
				PayLegIndex:       "EURIBOR6M",
				PayLegSource:      "LCH",
				RecLegSource:      "LCH",
				DiscountingSource: "LCH",
			}
			if dbSpread, err := db.GetBasisSwapSpread(dbConn, params); err == nil {
				diff := spread - dbSpread
				output += fmt.Sprintf(" | database=%.6f bp | diff=%.6f bp", dbSpread, diff)
			}
		}
		fmt.Println(output)
	}
}
