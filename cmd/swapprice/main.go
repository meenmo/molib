package main

import (
	"fmt"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/benchmark"
)

func main() {
	// Common trade date
	tradeDate := time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC)

	fmt.Println("================================================================================")
	fmt.Println("PLAIN VANILLA INTEREST RATE SWAP NPV EXAMPLES")
	fmt.Println("================================================================================")
	fmt.Println("Trade Date:", tradeDate.Format("2006-01-02"))
	fmt.Println()

	// Example 1: EUR - Fixed vs EURIBOR 3M
	fmt.Println("1. EUR SWAP: Fixed 2.50% vs EURIBOR 3M")
	fmt.Println("   Tenor: 10Y | Notional: EUR 10,000,000")
	priceEURSwap_EURIBOR3M(tradeDate)

	// Example 2: EUR - Fixed vs EURIBOR 6M
	fmt.Println("\n2. EUR SWAP: Fixed 2.55% vs EURIBOR 6M")
	fmt.Println("   Tenor: 10Y | Notional: EUR 10,000,000")
	priceEURSwap_EURIBOR6M(tradeDate)

	// Example 3: JPY - Fixed vs TIBOR 3M
	fmt.Println("\n3. JPY SWAP: Fixed 0.80% vs TIBOR 3M")
	fmt.Println("   Tenor: 10Y | Notional: JPY 1,000,000,000")
	priceJPYSwap_TIBOR3M(tradeDate)

	// Example 4: JPY - Fixed vs TIBOR 6M
	fmt.Println("\n4. JPY SWAP: Fixed 0.85% vs TIBOR 6M")
	fmt.Println("   Tenor: 10Y | Notional: JPY 1,000,000,000")
	priceJPYSwap_TIBOR6M(tradeDate)

	// Example 5: EUR - Fixed vs ESTR (OIS)
	fmt.Println("\n5. EUR OIS SWAP: Fixed 2.30% vs ESTR")
	fmt.Println("   Tenor: 10Y | Notional: EUR 10,000,000")
	priceEURSwap_ESTR(tradeDate)

	// Example 6: JPY - Fixed vs TONAR (OIS)
	fmt.Println("\n6. JPY OIS SWAP: Fixed 0.50% vs TONAR")
	fmt.Println("   Tenor: 10Y | Notional: JPY 1,000,000,000")
	priceJPYSwap_TONAR(tradeDate)

	// Example 7: USD - Fixed vs SOFR (OIS)
	fmt.Println("\n7. USD OIS SWAP: Fixed 4.50% vs SOFR")
	fmt.Println("   Tenor: 10Y | Notional: USD 10,000,000")
	priceUSDSwap_SOFR(tradeDate)

	// Example 8: KRW - Fixed vs CD 91-day
	fmt.Println("\n8. KRW SWAP: Fixed 3.20% vs CD 91-day")
	fmt.Println("   Tenor: 10Y | Notional: KRW 10,000,000,000")
	priceKRWSwap_CD(tradeDate)

	fmt.Println("\n================================================================================")
}

func priceEURSwap_EURIBOR3M(tradeDate time.Time) {
	// Spot date: T+2
	spotDate := calendar.AddBusinessDays(calendar.TARGET, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0) // 10Y
	maturity = calendar.AdjustFollowing(calendar.TARGET, maturity)

	notional := 10_000_000.0
	fixedRate := 2.50 // Fixed rate in %

	// Create fixed leg convention
	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Dc30360,
		PayFrequency:          benchmark.FreqAnnual,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.TARGET,
	}

	// Create floating leg convention (EURIBOR 3M)
	floatingLeg := benchmark.EURIBOR3MFloat // Using preset from swap/benchmark/presets.go

	// Create swap specification
	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.ESTRFloat, // Discount with ESTR OIS curve
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% annual, 30/360\n", fixedRate)
	fmt.Printf("   Float Leg: EURIBOR 3M, quarterly, ACT/360\n")
	fmt.Printf("   Status: Pricing requires market data (OIS & EURIBOR curves)\n")

	// Note: Actual NPV calculation would require:
	// 1. ESTR OIS curve for discounting
	// 2. EURIBOR 3M projection curve
	// 3. Cash flow generation
	// 4. PV calculation with dual-curve approach
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceEURSwap_EURIBOR6M(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.TARGET, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.TARGET, maturity)

	notional := 10_000_000.0
	fixedRate := 2.55

	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Dc30360,
		PayFrequency:          benchmark.FreqSemi, // Semi-annual for 6M swap
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.TARGET,
	}

	floatingLeg := benchmark.EURIBOR6MFloat

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.ESTRFloat,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% semi-annual, 30/360\n", fixedRate)
	fmt.Printf("   Float Leg: EURIBOR 6M, semi-annual, ACT/360\n")
	fmt.Printf("   Status: Pricing requires market data (OIS & EURIBOR curves)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceJPYSwap_TIBOR3M(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.JPN, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.JPN, maturity)

	notional := 1_000_000_000.0
	fixedRate := 0.80

	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act365F,
		PayFrequency:          benchmark.FreqSemi,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.JPN,
	}

	floatingLeg := benchmark.TIBOR3MFloat

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.TONARFloat,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% semi-annual, ACT/365F\n", fixedRate)
	fmt.Printf("   Float Leg: TIBOR 3M, quarterly, ACT/365F\n")
	fmt.Printf("   Status: Pricing requires market data (TONAR & TIBOR curves)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceJPYSwap_TIBOR6M(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.JPN, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.JPN, maturity)

	notional := 1_000_000_000.0
	fixedRate := 0.85

	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act365F,
		PayFrequency:          benchmark.FreqSemi,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.JPN,
	}

	floatingLeg := benchmark.TIBOR6MFloat

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.TONARFloat,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% semi-annual, ACT/365F\n", fixedRate)
	fmt.Printf("   Float Leg: TIBOR 6M, semi-annual, ACT/365F\n")
	fmt.Printf("   Status: Pricing requires market data (TONAR & TIBOR curves)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceEURSwap_ESTR(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.TARGET, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.TARGET, maturity)

	notional := 10_000_000.0
	fixedRate := 2.30

	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act360,
		PayFrequency:          benchmark.FreqAnnual,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.TARGET,
	}

	floatingLeg := benchmark.ESTRFloat

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.ESTRFloat, // Single curve for OIS
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% annual, ACT/360\n", fixedRate)
	fmt.Printf("   Float Leg: ESTR daily compounded, annual payment\n")
	fmt.Printf("   Status: Pricing requires ESTR OIS curve (single curve)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceJPYSwap_TONAR(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.JPN, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.JPN, maturity)

	notional := 1_000_000_000.0
	fixedRate := 0.50

	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act365F,
		PayFrequency:          benchmark.FreqAnnual,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.JPN,
	}

	floatingLeg := benchmark.TONARFloat

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: benchmark.TONARFloat,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% annual, ACT/365F\n", fixedRate)
	fmt.Printf("   Float Leg: TONAR daily compounded, annual payment\n")
	fmt.Printf("   Status: Pricing requires TONAR OIS curve (single curve)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceUSDSwap_SOFR(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.USD, tradeDate, 2)
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.USD, maturity)

	notional := 10_000_000.0
	fixedRate := 4.50

	// SOFR OIS convention
	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act360,
		PayFrequency:          benchmark.FreqAnnual,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.USD,
	}

	// SOFR floating leg (daily compounded, annual payment)
	floatingLeg := benchmark.LegConvention{
		LegType:                 benchmark.LegFloating,
		ReferenceRate:           benchmark.SOFR,
		DayCount:                benchmark.Act360,
		ResetFrequency:          benchmark.FreqDaily,
		PayFrequency:            benchmark.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            2,
		BusinessDayAdjustment:   benchmark.ModifiedFollowing,
		RollConvention:          benchmark.BackwardEOM,
		Calendar:                calendar.USD,
		ResetPosition:           benchmark.ResetInArrears,
		RateCutoffDays:          2,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: floatingLeg,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% annual, ACT/360\n", fixedRate)
	fmt.Printf("   Float Leg: SOFR daily compounded, annual payment\n")
	fmt.Printf("   Status: Pricing requires SOFR OIS curve (single curve)\n")
	fmt.Printf("   Structure: %+v\n", swap)
}

func priceKRWSwap_CD(tradeDate time.Time) {
	spotDate := calendar.AddBusinessDays(calendar.KRW, tradeDate, 1) // T+1 for KRW
	maturity := spotDate.AddDate(10, 0, 0)
	maturity = calendar.AdjustFollowing(calendar.KRW, maturity)

	notional := 10_000_000_000.0
	fixedRate := 3.20

	// Fixed leg convention for KRW
	fixedLeg := benchmark.LegConvention{
		LegType:               benchmark.LegFixed,
		DayCount:              benchmark.Act365,
		PayFrequency:          benchmark.FreqQuarterly,
		BusinessDayAdjustment: benchmark.ModifiedFollowing,
		Calendar:              calendar.KRW,
	}

	// CD 91-day floating leg
	floatingLeg := benchmark.LegConvention{
		LegType:                 benchmark.LegFloating,
		ReferenceRate:           benchmark.CD91,
		DayCount:                benchmark.Act365,
		ResetFrequency:          benchmark.FreqQuarterly,
		PayFrequency:            benchmark.FreqQuarterly,
		FixingLagDays:           2,
		PayDelayDays:            0,
		BusinessDayAdjustment:   benchmark.ModifiedFollowing,
		RollConvention:          benchmark.BackwardEOM,
		Calendar:                calendar.KRW,
		ResetPosition:           benchmark.ResetInAdvance,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	// For discounting, would use KOFR (Korean overnight rate)
	discountLeg := benchmark.LegConvention{
		LegType:                 benchmark.LegFloating,
		ReferenceRate:           "KOFR", // Korean Overnight Financing Repo Rate
		DayCount:                benchmark.Act365,
		ResetFrequency:          benchmark.FreqDaily,
		PayFrequency:            benchmark.FreqAnnual,
		FixingLagDays:           0,
		PayDelayDays:            1,
		BusinessDayAdjustment:   benchmark.ModifiedFollowing,
		RollConvention:          benchmark.BackwardEOM,
		Calendar:                calendar.KRW,
		ResetPosition:           benchmark.ResetInArrears,
		RateCutoffDays:          1,
		IncludeInitialPrincipal: true,
		IncludeFinalPrincipal:   true,
	}

	swap := benchmark.SwapSpec{
		Notional:       notional,
		EffectiveDate:  spotDate,
		MaturityDate:   maturity,
		PayLeg:         fixedLeg,
		RecLeg:         floatingLeg,
		DiscountingOIS: discountLeg,
	}

	fmt.Printf("   Effective: %s | Maturity: %s\n",
		spotDate.Format("2006-01-02"), maturity.Format("2006-01-02"))
	fmt.Printf("   Fixed Leg: %.2f%% quarterly, ACT/365\n", fixedRate)
	fmt.Printf("   Float Leg: CD 91-day, quarterly, ACT/365\n")
	fmt.Printf("   Status: Pricing requires KOFR & CD curves\n")
	fmt.Printf("   Structure: %+v\n", swap)
}
