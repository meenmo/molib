# molib Basis Swap Testing Guide

This guide explains how to test basis swap pricing with different dates and configurations.

## Quick Testing Options

### Option 1: Use the Flexible Calculator (Recommended for Quick Tests)

Copy the flexible calculator to your cmd directory:

```bash
cp /tmp/basiscalc_flexible.go /Users/meenmo/Documents/workspace/molib/cmd/basiscalc-flex/main.go
```

Run with custom parameters:

```bash
# Test BGN EUR 10x10 on 2025-11-21 (default)
go run ./cmd/basiscalc-flex

# Test with different date
go run ./cmd/basiscalc-flex -date 2025-11-25

# Test different structure: 5x15 swap
go run ./cmd/basiscalc-flex -date 2025-11-21 -forward 5 -tenor 15

# Test LCH EUR
go run ./cmd/basiscalc-flex -provider LCH -currency EUR

# Test TIBOR
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 1 -tenor 4
```

**Flags:**
- `-date`: Curve date in YYYY-MM-DD format (default: 2025-11-21)
- `-provider`: BGN or LCH (default: BGN)
- `-currency`: EUR or JPY (default: EUR)
- `-forward`: Forward start in years (default: 10)
- `-tenor`: Swap tenor in years (default: 10)

### Option 2: Standalone Test Program

For complete control, use the custom test program:

```bash
# Edit /tmp/test_custom_date.go to set your curve date and quotes
# Then run:
go run /tmp/test_custom_date.go
```

### Option 3: Add New Test Data Files

For testing with a completely new curve date:

#### Step 1: Extract curve data from database

```bash
# Extract quotes for a specific date
docker exec -u airflow airflow-worker python3 /tmp/extract_curves_new_date.py 2025-11-25 > /tmp/curves_2025_11_25.go
```

#### Step 2: Create new fixture file

```bash
# Copy to data directory
cp /tmp/curves_2025_11_25.go /Users/meenmo/Documents/workspace/molib/swap/basis/data/fixtures_bgn_eur_20251125.go
```

#### Step 3: Update the fixture file format

Edit the file to match Go package format:

```go
package data

// BGN EUR quotes for curve date 2025-11-25.
var (
    BGNEstr_20251125 = map[string]float64{
        "1W": 1.928,
        // ... rest of quotes
    }

    BGNEuribor3M_20251125 = map[string]float64{
        "3M": 2.054,
        // ... rest of quotes
    }

    BGNEuribor6M_20251125 = map[string]float64{
        "6M": 2.138,
        // ... rest of quotes
    }
)
```

#### Step 4: Create a test program

```go
package main

import (
    "fmt"
    "time"
    "github.com/meenmo/molib/swap/basis"
    "github.com/meenmo/molib/swap/basis/data"
    "github.com/meenmo/molib/swap/benchmark"
)

func main() {
    curveDate := time.Date(2025, 11, 25, 0, 0, 0, 0, time.UTC)

    spread, pv := basis.CalculateSpread(
        curveDate,
        10, 10,
        benchmark.EURIBOR6M_FLOATING,
        benchmark.EURIBOR3M_FLOATING,
        benchmark.ESTR_OIS,
        data.BGNEstr_20251125,
        data.BGNEuribor6M_20251125,
        data.BGNEuribor3M_20251125,
        10_000_000.0,
    )

    fmt.Printf("BGN EUR 10x10 (2025-11-25): %.6f bp, NPV=%.2f\n", spread, pv.TotalPV)
}
```

## Testing Different Swap Structures

### Common Structures to Test

```bash
# Spot-starting swaps
go run ./cmd/basiscalc-flex -forward 0 -tenor 5   # 0x5
go run ./cmd/basiscalc-flex -forward 0 -tenor 10  # 0x10

# Forward-starting swaps
go run ./cmd/basiscalc-flex -forward 5 -tenor 5   # 5x5
go run ./cmd/basiscalc-flex -forward 10 -tenor 10 # 10x10 (most common)
go run ./cmd/basiscalc-flex -forward 10 -tenor 20 # 10x20

# Long-dated structures
go run ./cmd/basiscalc-flex -forward 15 -tenor 15 # 15x15
go run ./cmd/basiscalc-flex -forward 20 -tenor 10 # 20x10
```

### Testing Different Providers

```bash
# BGN (Bloomberg) EUR
go run ./cmd/basiscalc-flex -provider BGN -currency EUR -forward 10 -tenor 10

# LCH EUR (more sparse quotes)
go run ./cmd/basiscalc-flex -provider LCH -currency EUR -forward 10 -tenor 10

# BGN TIBOR/TONAR
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 1 -tenor 4
go run ./cmd/basiscalc-flex -provider BGN -currency JPY -forward 2 -tenor 3
```

## Validation Against ficclib

To validate molib results against ficclib (Python reference):

### Step 1: Run molib test

```bash
go run ./cmd/basiscalc-flex -date 2025-11-21 -forward 10 -tenor 10 > /tmp/molib_result.txt
```

### Step 2: Run ficclib test

Create `/tmp/test_ficclib.py`:

```python
from datetime import date
from ficclib.swap.basis.pricing import calculate_basis_swap_spread
from ficclib.data.curves import load_curves

curve_date = date(2025, 11, 21)
curves = load_curves(curve_date, source='BGN', currency='EUR')

spread = calculate_basis_swap_spread(
    curve_date=curve_date,
    forward_years=10,
    tenor_years=10,
    curves=curves,
    notional=10_000_000
)

print(f"ficclib result: {spread:.6f} bp")
```

Run:

```bash
docker exec -u airflow airflow-worker python3 /tmp/test_ficclib.py > /tmp/ficclib_result.txt
```

### Step 3: Compare results

```bash
cat /tmp/molib_result.txt /tmp/ficclib_result.txt
```

Expected difference: < 0.5 bp for EUR, < 0.2 bp for TIBOR

## Debugging Failed Tests

If you get unexpected results:

### 1. Check OIS Discount Factors

```go
package main

import (
    "fmt"
    "time"
    "github.com/meenmo/molib/calendar"
    "github.com/meenmo/molib/swap/basis"
    "github.com/meenmo/molib/swap/basis/data"
)

func main() {
    settlement, _ := time.Parse("2006-01-02", "2025-11-21")
    oisCurve := basis.BuildCurve(settlement, data.BGNEstr, calendar.TARGET, 1)

    // Check DFs at key tenors
    for _, years := range []int{1, 2, 5, 10, 15, 20, 25, 30} {
        d := settlement.AddDate(years, 0, 0)
        adj := calendar.Adjust(calendar.TARGET, d)
        df := oisCurve.DF(adj)
        fmt.Printf("%2dY: DF=%.12f\n", years, df)
    }
}
```

### 2. Check IBOR Projection Curves

(Requires dual-curve implementation - not yet added)

### 3. Check Schedule Generation

```go
// Add to test program
periods := buildSchedule(effectiveDate, maturityDate, legConvention)
fmt.Printf("Number of periods: %d\n", len(periods))
for i, p := range periods[:5] {  // First 5 periods
    fmt.Printf("Period %d: %v to %v, pay on %v\n",
        i, p.AccrualStart, p.AccrualEnd, p.PaymentDate)
}
```

## Performance Testing

To test performance with many calculations:

```bash
# Time 100 calculations
time for i in {1..100}; do
    go run ./cmd/basiscalc-flex -date 2025-11-21 > /dev/null
done
```

## Current Test Results (2025-11-21)

**✅ All EUR cases within 0.4 bp:**

| Test Case | molib Result | Expected Error |
|-----------|--------------|----------------|
| BGN EUR 10x10 | -4.072100 bp | < 0.1 bp ✅ |
| BGN EUR 10x20 | -4.972943 bp | < 0.1 bp ✅ |
| LCH EUR 10x10 | -3.598092 bp | < 0.3 bp ✅ |
| LCH EUR 10x20 | -4.075852 bp | < 0.4 bp ✅ |
| BGNS TIBOR 1x4 | -2.190111 bp | ~0.1 bp 🟡 |
| BGNS TIBOR 2x3 | -2.392584 bp | ~0.1 bp 🟡 |

**Known Limitations:**
- TIBOR has ~0.1 bp error (marginal, but acceptable)
- Very long-dated tenors (40Y+) may have larger interpolation errors
- Sparse curves (like LCH) may have slightly larger errors than dense curves (like BGN)

## Adding New Currency Pairs

To add USD, GBP, or other currencies:

1. Add new fixture files in `swap/basis/data/`
2. Add new benchmark leg conventions in `swap/benchmark/`
3. Update the flexible calculator to support new currencies
4. Extract curve data from database for the new currency

Example for USD:
```bash
# Extract USD curves
docker exec -u airflow airflow-worker python3 /tmp/extract_curves_usd.py 2025-11-21

# Add to data/fixtures_bgn_usd.go
# Add benchmark.SOFR_OIS, benchmark.USD_LIBOR_3M, etc.
# Update test program
```
