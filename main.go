package main

import (
	"fmt"

	"github.com/meenmo/molib/marketdata/krx"
	"github.com/meenmo/molib/swap"
)

func main() {
	quotes := swap.ParSwapQuotes{
		0:    2.5524458035,
		0.25: 2.7600000000,
		0.5:  2.7225000000,
		0.75: 2.7225000000,
		1:    2.7225000000,
		1.5:  2.7571428571,
		2:    2.8075000000,
		3:    2.8882142857,
		4:    2.9596428571,
		5:    3.0189285714,
		6:    3.0614285714,
		7:    3.0889285714,
		8:    3.1153571429,
		9:    3.1357142857,
		10:   3.1578571429,
		12:   3.1910714286,
		15:   3.1757142857,
		20:   3.0946428571,
	}

	trade := swap.InterestRateSwap{
		EffectiveDate:   "2024-01-25",
		TerminationDate: "2044-01-25",
		SettlementDate:  "2025-11-21",
		FixedRate:       3.24,
		Notional:        10000000000,
		Direction:       swap.PositionReceive,
		SwapQuotes:      quotes,
		ReferenceRate:   krx.DefaultReferenceFeed(),
	}

	curve := swap.BootstrapCurve(trade.SettlementDate, trade.SwapQuotes)
	fixedPV, floatPV := trade.PVByLeg(curve)

	fmt.Printf("Fixed PV: %.2f\n", fixedPV)
	fmt.Printf("Floating PV: %.2f\n", floatPV)
	fmt.Printf("NPV: %.2f\n", trade.NPV(curve))
}
