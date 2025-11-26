package swap

import "github.com/meenmo/molib/marketdata/krx"

// Position describes whether the swap receives or pays the fixed leg.
type Position string

const (
	PositionReceive Position = "REC"
	PositionPay     Position = "PAY"
)

// ParSwapQuotes maps year-based tenors (e.g., 0, 0.25, 1, 5) to quoted par swap rates.
type ParSwapQuotes map[float64]float64

// InterestRateSwap captures the key economic terms for pricing.
type InterestRateSwap struct {
	EffectiveDate   string
	TerminationDate string
	SettlementDate  string
	FixedRate       float64
	Notional        float64
	Direction       Position
	SwapQuotes      ParSwapQuotes
	ReferenceRate   krx.ReferenceRateFeed
}
