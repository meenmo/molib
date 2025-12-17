# Fixture Generation Guide

This guide explains how to generate Go fixture files from database curve data for testing with different dates.

## 📋 Prerequisites

**Option A: Local Python (Recommended)**
- Python virtual environment at `.venv/`
- Install dependencies: `uv pip install psycopg2-binary`

**Option B: Docker Container**
- Docker container with access to the PostgreSQL database
- Python 3 with `psycopg2` already installed in airflow-worker

## 🚀 Quick Start

### Generate Fixtures Using Local Python (Recommended)

```bash
cd /Users/meenmo/Documents/workspace/molib

# Install dependencies (first time only)
uv pip install psycopg2-binary

# Generate fixtures
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251125 \
  --source BGN \
  --currency EUR
```

### Generate Fixtures Using Docker

```bash
# BGN EUR curves for 2025-11-25
docker exec -u airflow airflow-worker \
  python3 /opt/airflow/dags/generate_fixtures.py \
  --date 20251125 \
  --source BGN \
  --currency EUR
```

## 📝 Usage Examples

### Example 1: Generate BGN EUR for a New Date (Local Python)

```bash
cd /Users/meenmo/Documents/workspace/molib

# Generate fixture file
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251201 \
  --source BGN \
  --currency EUR

# Output:
# ✅ Generated: marketdata/fixtures_bgn_euribor.go
#    - BGNEstr: 51 tenors
#    - BGNEuribor3M: 42 tenors
#    - BGNEuribor6M: 39 tenors
```

### Example 2: Generate All Available Curves for a Date

```bash
# Using local Python
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251125 \
  --all

# Or using Docker
docker exec -u airflow airflow-worker \
  python3 /opt/airflow/dags/generate_fixtures.py \
  --date 20251125 \
  --all
```

### Example 3: Generate LCH EUR and BGN TIBOR

```bash
# LCH EUR
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251125 \
  --source LCH \
  --currency EUR

# BGN TIBOR
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251125 \
  --source BGN \
  --currency JPY
```

### Example 4: Use Generated Fixtures in Your Code

The generated file is immediately ready to use:

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
    // Use the generated fixtures
    curveDate := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

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
        OISQuotes:         marketdata.BGNEstr,
        PayLegQuotes:      marketdata.BGNEuribor6M,
        RecLegQuotes:      marketdata.BGNEuribor3M,
    })
    if err != nil {
        panic(err)
    }

    spread, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Spread: %.6f bp\n", spread)
    fmt.Printf("NPV: %.2f\n", pv.TotalPV)
}
```

## 🎛️ Command-Line Options

### Required Arguments

- **`--date`**: Curve date in YYYYMMDD format
  - Example: `--date 20251125`

### Optional Arguments (one of the following is required)

**Option A: Specify source and currency**
- **`--source`**: Data source (BGN, LCH, etc.)
- **`--currency`**: Currency code (EUR, JPY, USD, GBP)

**Option B: Generate all**
- **`--all`**: Generate fixtures for all available curves on the date

### Other Options

- **`--output-dir`**: Output directory (default: `marketdata`)
  - Example: `--output-dir /tmp/my_fixtures`

## 📊 Supported Currencies

The script supports the following currency/index combinations:

| Currency | OIS Index | 3M IBOR | 6M IBOR |
|----------|-----------|---------|---------|
| EUR | ESTR | EURIBOR3M | EURIBOR6M |
| JPY | TONAR | TIBOR3M | TIBOR6M |
| USD | SOFR | USD_LIBOR_3M | USD_LIBOR_6M |
| GBP | SONIA | GBP_LIBOR_3M | GBP_LIBOR_6M |

**Note**: USD and GBP are configured but may not have data in your database.

### Tenor Filtering

The script automatically filters tenors to include only the allowed tenors for each index:

- **ESTR**: 32 standard tenors (1W, 2W, 1M-11M, 1Y, 18M, 2Y-12Y, 15Y, 20Y, 25Y, 30Y, 40Y, 50Y)
- **EURIBOR3M**: 19 standard tenors (3M, 1Y, 2Y-12Y, 15Y, 20Y, 25Y, 30Y, 40Y, 50Y)
- **EURIBOR6M**: 20 standard tenors (6M, 1Y, 18M, 2Y-12Y, 15Y, 20Y, 25Y, 30Y, 40Y, 50Y)
- **TONAR**: 34 standard tenors (similar structure to ESTR)
- **TIBOR3M/6M**: 17 standard tenors each

This ensures that only liquid, commonly-traded tenors are included in the generated fixtures.

## 🔍 Finding Available Dates

Check what dates are available in the database:

```bash
# List recent dates with BGN EUR data
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT date
FROM marketdata.curves
WHERE source='BGN' AND reference_index='ESTR'
ORDER BY date DESC
LIMIT 20;
"

# List all sources for a specific date
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT source, reference_index
FROM marketdata.curves
WHERE date='2025-11-25'
ORDER BY source, reference_index;
"
```

## 📁 Output Format

### File Naming Convention

Generated files overwrite stable, source+family filenames:
```
fixtures_{source}_{family}.go
```

Examples:
- `fixtures_bgn_euribor.go`
- `fixtures_lch_euribor.go`
- `fixtures_bgn_tibor.go`

### Generated Variable Names

Variable names use stable, date-less identifiers:
```
{SOURCE}{Index}
```

Examples:
- `BGNEstr`
- `BGNEuribor3M`
- `BGNEuribor6M`
- `LCHEstr`

### File Structure

```go
package marketdata

// BGN EUR quotes for curve date 2025-11-25.
// Generated by generate_fixtures.py on 2025-11-27 14:30:00
var (
    // ESTR OIS curve (51 tenors)
    BGNEstr = map[string]float64{
        "1W": 1.928,
        "2W": 1.931,
        "1M": 1.931,
        // ... more tenors ...
        "30Y": 2.912,
    }

    // EURIBOR3M curve (42 tenors)
    BGNEuribor3M = map[string]float64{
        "3M": 2.054,
        "6M": 2.052,
        // ... more tenors ...
    }

    // EURIBOR6M curve (39 tenors)
    BGNEuribor6M = map[string]float64{
        "6M": 2.138,
        "1Y": 2.123,
        // ... more tenors ...
    }
)
```

## 🐛 Troubleshooting

### Issue: "ModuleNotFoundError: No module named 'psycopg2'"

**Solution**: Install the required package:

```bash
cd /Users/meenmo/Documents/workspace/molib
uv pip install psycopg2-binary
```

### Issue: "No data found"

```bash
❌ No data found for BGN EUR on 2025-11-25
```

**Solution**: Check if data exists for that date in the database:

```bash
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT date, source, reference_index, jsonb_array_length(quotes) as num_quotes
FROM marketdata.curves
WHERE date='2025-11-25'
  AND source='BGN';
"
```

### Issue: Database connection error

```bash
Error retrieving BGN ESTR: connection refused
```

**Solution**:
1. Check database is accessible: `ping 100.127.72.74`
2. Verify database credentials in the script (currently hardcoded)
3. Check if your IP is allowed to connect to the database

### Issue: Missing some curves

```bash
⚠️  Warning: No EURIBOR6M curve found
```

**Solution**: This is informational - the script will generate a fixture with empty map for missing curves. You can:
1. Check if the curve exists in the database for that date
2. Continue with partial data if acceptable
3. Choose a different date that has complete data

## 🔄 Complete Workflow Example

Here's a complete workflow for testing with a new date:

```bash
cd /Users/meenmo/Documents/workspace/molib

# 1. Find available dates
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT date
FROM marketdata.curves
WHERE source='BGN' AND date >= '2025-11-01'
ORDER BY date DESC
LIMIT 10;
"

# 2. Generate fixtures for chosen date (e.g., 2025-11-27)
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251127 \
  --source BGN \
  --currency EUR

# 3. Generated file is already in the correct location!
# marketdata/fixtures_bgn_euribor.go

# 4. Create test program
cat > /tmp/test_20251127.go << 'EOF'
package main

import (
    "fmt"
    "time"
    swaps "github.com/meenmo/molib/instruments/swaps"
    "github.com/meenmo/molib/marketdata"
    "github.com/meenmo/molib/swap"
)

func main() {
    curveDate := time.Date(2025, 11, 27, 0, 0, 0, 0, time.UTC)

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
        OISQuotes:         marketdata.BGNEstr,
        PayLegQuotes:      marketdata.BGNEuribor6M,
        RecLegQuotes:      marketdata.BGNEuribor3M,
    })
    if err != nil {
        panic(err)
    }

    spread, pv, err := trade.SolveParSpread(swap.SpreadTargetRecLeg)
    if err != nil {
        panic(err)
    }

    fmt.Printf("BGN EUR 10x10 (2025-11-27): %.6f bp\n", spread)
    fmt.Printf("NPV: %.2f\n", pv.TotalPV)
}
EOF

# 5. Run test
go run /tmp/test_20251127.go
```

## 🔧 Advanced Usage

### Generate Fixtures with Custom Output Path

```bash
# Generate to a custom directory
.venv/bin/python3 scripts/generate_fixtures.py \
  --date 20251125 \
  --source BGN \
  --currency EUR \
  --output-dir /tmp/custom_fixtures
```

### Batch Generate Multiple Dates

```bash
# Generate fixtures for multiple dates
for date in 20251125 20251126 20251127; do
    echo "Generating fixtures for $date..."
    .venv/bin/python3 scripts/generate_fixtures.py \
      --date $date \
      --source BGN \
      --currency EUR
done
```

### Updating the Default Fixtures

Running generate_fixtures.py overwrites the stable fixture files in marketdata/ (for example, marketdata/fixtures_bgn_euribor.go) with standard variable names (for example, BGNEstr).

## ✅ Validation

After generating fixtures, validate they work:

```bash
# Quick validation - check file compiles
go build ./marketdata

# Full validation - run pricing test
go run ./cmd/basiscalc
```

## 🔐 Security Note

The database credentials are currently hardcoded in the script:
```python
DB_CONFIG = {
    'host': '100.127.72.74',
    'port': 1013,
    'user': 'meenmo',
    'password': '04201',
    'database': 'ficc'
}
```

For production use, consider:
- Using environment variables
- Using a configuration file
- Using a secrets manager

## 📚 Related Documentation

- **Quick Start**: `docs/QUICK_START.md` - Basic testing commands
- **Data Flow**: `docs/DATA_FLOW.md` - Understanding where data comes from
- **Testing Guide**: `docs/TESTING_GUIDE.md` - Comprehensive testing guide

## ⚡ Performance Tips

**Local Python vs Docker**:
- **Local Python**: Faster startup, no Docker overhead
- **Docker**: Already configured, no setup needed

**Recommendation**: Use local Python (`.venv/bin/python3`) for faster iteration during development.
