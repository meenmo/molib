# molib Basis Swap Testing - Quick Start

## 🎯 Quick Commands (Ready to Use Now!)

All files are now in your molib project directory. You can use these commands from anywhere:

### Test with Different Swap Structures

```bash
cd /Users/meenmo/Documents/workspace/molib

# Default: BGN EUR 10x10
go run ./cmd/basiscalc-flex

# Different forward start and tenor
go run ./cmd/basiscalc-flex -forward 5 -tenor 15

# Different provider
go run ./cmd/basiscalc-flex -provider LCH -forward 10 -tenor 20

# Japanese TIBOR
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 1 -tenor 4
```

### Available Flags

- **`-date`**: Curve date in YYYY-MM-DD format (default: "2025-11-21")
- **`-provider`**: BGN or LCH (default: "BGN")
- **`-currency`**: EUR or JPY (default: "EUR")
- **`-forward`**: Forward start in years (default: 10)
- **`-tenor`**: Swap tenor in years (default: 10)

### Examples

```bash
# 5Y forward-starting 15Y swap
go run ./cmd/basiscalc-flex -forward 5 -tenor 15

# Spot-starting 10Y swap
go run ./cmd/basiscalc-flex -forward 0 -tenor 10

# 20Y forward-starting 10Y swap
go run ./cmd/basiscalc-flex -forward 20 -tenor 10
```

## 📁 File Locations in Your Project

```
/Users/meenmo/Documents/workspace/molib/
├── cmd/
│   ├── basiscalc/          # Original test suite (all test cases)
│   │   └── main.go
│   └── basiscalc-flex/     # NEW: Flexible calculator
│       └── main.go
├── instruments/
│   └── swaps/              # Leg conventions + presets
│       └── conventions.go
├── docs/
│   ├── QUICK_START.md      # This file
│   └── TESTING_GUIDE.md    # Complete testing guide
├── marketdata/             # Embedded curve fixtures
│   ├── fixtures_bgn_euribor.go
│   ├── fixtures_lch_euribor.go
│   └── fixtures_bgn_tibor.go
├── scripts/
│   └── extract_curves_new_date.py  # Extract DB data for new dates
└── swap/
    ├── api.go              # Unified trade builder API
    ├── common.go           # NPV, schedules, spread solver
    ├── curve/              # Curve construction (OIS + dual-curve)
    ├── market/             # Primitive types (legs/spec)
    └── clearinghouse/
        └── krx/            # KRX-specific legacy engine
```

## 🔄 Testing with a New Curve Date

### Step 1: Extract Data from Database

The Python script needs to run **inside the Docker container** to access the database:

```bash
# Extract curves for a new date (e.g., 2025-11-25)
docker exec -u airflow airflow-worker \
  python3 /Users/meenmo/Documents/workspace/molib/scripts/extract_curves_new_date.py 2025-11-25 \
  > /tmp/curves_20251125.txt

# View the extracted data
cat /tmp/curves_20251125.txt
```

**Note**: The script file is on your local machine, but Docker can access it because your workspace is mounted in the container.

### Step 2: Create New Fixture File

```bash
# Create a new fixture file
cat > marketdata/fixtures_bgn_eur_20251125.go << 'EOF'
package marketdata

// BGN EUR quotes for curve date 2025-11-25
var (
    BGNEstr_20251125 = map[string]float64{
        // Paste quotes from Step 1 here
    }

    BGNEuribor3M_20251125 = map[string]float64{
        // Paste quotes from Step 1 here
    }

    BGNEuribor6M_20251125 = map[string]float64{
        // Paste quotes from Step 1 here
    }
)
EOF
```

### Step 3: Create Test Program

```go
package main

import (
    "fmt"
    "time"
    swaps "github.com/meenmo/molib/instruments/swaps"
    "github.com/meenmo/molib/marketdata"
    "github.com/meenmo/molib/swap"
)

func main() {
    curveDate := time.Date(2025, 11, 25, 0, 0, 0, 0, time.UTC)

    trade, err := swap.InterestRateSwap(swap.InterestRateSwapParams{
        DataSource:        swap.DataSourceBGN,
        ClearingHouse:     swap.ClearingHouseOTC,
        CurveDate:         curveDate,
        TradeDate:         curveDate,
        ValuationDate:     curveDate,
        ForwardTenorYears: 10,
        SwapTenorYears:    10,
        Notional:          10_000_000.0,
        PayLeg:            swaps.EURIBOR6MFloat,
        RecLeg:            swaps.EURIBOR3MFloat,
        DiscountingOIS:    swaps.ESTRFloat,
        OISQuotes:         marketdata.BGNEstr_20251125,
        PayLegQuotes:      marketdata.BGNEuribor6M_20251125,
        RecLegQuotes:      marketdata.BGNEuribor3M_20251125,
    })
    if err != nil {
        panic(err)
    }

    spread, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %.6f bp\n", spread)
    fmt.Printf("NPV: %.2f\n", pv.TotalPV)
}
```

## ✅ Current Test Results (2025-11-21)

After Phase 8 OIS bootstrap fix:

| Test Case | Result | Expected | Error | Status |
|-----------|--------|----------|-------|--------|
| BGN EUR 10x10 | -4.072100 bp | -4.023073 bp | 0.05 bp | ✅ Excellent |
| BGN EUR 10x20 | -4.972943 bp | -4.941096 bp | 0.03 bp | ✅ Excellent |
| LCH EUR 10x10 | -3.598092 bp | -3.872002 bp | 0.27 bp | ✅ Great |
| LCH EUR 10x20 | -4.075852 bp | -4.426513 bp | 0.35 bp | ✅ Great |
| TIBOR 1x4 | -2.190111 bp | -2.285949 bp | 0.10 bp | 🟡 Acceptable |
| TIBOR 2x3 | -2.392584 bp | -2.513270 bp | 0.12 bp | 🟡 Acceptable |

All EUR cases achieve **< 0.4 bp error** ✅

## 🐛 Debugging

### Check OIS Discount Factors

```go
package main

import (
    "fmt"
    "time"
    "github.com/meenmo/molib/calendar"
    "github.com/meenmo/molib/swap/curve"
    "github.com/meenmo/molib/marketdata"
)

func main() {
    settlement, _ := time.Parse("2006-01-02", "2025-11-21")
    oisCurve := curve.BuildCurve(settlement, marketdata.BGNEstr, calendar.TARGET, 1)

    for _, years := range []int{1, 5, 10, 15, 20, 30} {
        d := settlement.AddDate(years, 0, 0)
        adj := calendar.Adjust(calendar.TARGET, d)
        df := oisCurve.DF(adj)
        fmt.Printf("%2dY: DF=%.12f\n", years, df)
    }
}
```

### Compare with ficclib (Python)

```python
from datetime import date
from ficclib.swap.basis.pricing import calculate_basis_swap_spread

spread = calculate_basis_swap_spread(
    date=date(2025, 11, 21),
    forward_years=10,
    tenor_years=10,
    source='BGN',
    currency='EUR',
    notional=10_000_000
)
print(f"ficclib: {spread:.6f} bp")
```

## 📚 More Information

See `docs/TESTING_GUIDE.md` for:
- Complete testing workflows
- Performance testing
- Adding new currency pairs
- Debugging failed tests
- Validation against ficclib

## 🚀 Quick Validation

Run all test cases:

```bash
# Run full test suite
go run ./cmd/basiscalc

# Or test specific cases
go run ./cmd/basiscalc-flex -provider BGN -currency EUR -forward 10 -tenor 10
go run ./cmd/basiscalc-flex -provider BGN -currency EUR -forward 10 -tenor 20
go run ./cmd/basiscalc-flex -provider LCH -currency EUR -forward 10 -tenor 10
go run ./cmd/basiscalc-flex -provider LCH -currency EUR -forward 10 -tenor 20
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 1 -tenor 4
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 2 -tenor 3
```
