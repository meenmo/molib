# molib Curve Data Flow

## 📊 Where Does the Curve Data Come From?

### Current Flow (As of 2025-11-27)

```
┌─────────────────────────────────────────────────────────────────┐
│  1. ORIGINAL SOURCE: PostgreSQL Database                        │
│                                                                  │
│  Database: ficc                                                  │
│  Host: 100.127.72.74:1013                                       │
│  Table: marketdata.curves                                       │
│                                                                  │
│  Columns:                                                        │
│  - date: date (e.g., '2025-11-21')                        │
│  - source: text (e.g., 'BGN', 'LCH')                            │
│  - reference_index: text (e.g., 'ESTR', 'EURIBOR3M')            │
│  - quotes: jsonb (e.g., {"1Y": 1.8916, "2Y": 1.912, ...})       │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                    (Manual extraction)
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  2. STATIC FIXTURE FILES (Go code)                              │
│                                                                  │
│  Location: marketdata/                                          │
│                                                                  │
│  Files:                                                          │
│  - fixtures_bgn_euribor.go ← BGN EUR curves (example date)     │
│  - fixtures_lch_euribor.go ← LCH EUR curves (example date)     │
│  - fixtures_bgn_tibor.go   ← BGN TIBOR curves (2025-11-21)     │
│                                                                  │
│  Variables:                                                      │
│  var BGNEstr = map[string]float64{                              │
│      "1W": 1.928,                                               │
│      "1M": 1.931,                                               │
│      "1Y": 1.8916,                                              │
│      "2Y": 1.912,                                               │
│      ...                                                         │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                    (Import in Go code)
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  3. TEST PROGRAMS                                               │
│                                                                  │
│  cmd/basiscalc/main.go:                                         │
│  import "github.com/meenmo/molib/marketdata"                    │
│  import "github.com/meenmo/molib/instruments/swaps"             │
│  import "github.com/meenmo/molib/swap"                          │
│                                                                  │
│  trade, _ := swap.InterestRateSwap(...)                          │
│  spreadBP, _ := trade.SolveParSpread(swap.SpreadTargetRecLeg)    │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                    (Passed to curve builder)
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  4. CURVE BOOTSTRAP (Runtime)                                   │
│                                                                  │
│  swap/curve/curve.go:                                           │
│                                                                  │
│  BuildCurve(settlement, quotes, calendar, freqMonths)           │
│    ↓                                                             │
│  - Parse tenor strings to float years                           │
│  - Generate payment dates (monthly/annual)                      │
│  - Interpolate par rates for all payment dates                  │
│  - Bootstrap discount factors (ONLY quoted pillars) ✅ FIXED    │
│  - Interpolate DFs for non-quoted dates                         │
│  - Calculate zero rates                                          │
│    ↓                                                             │
│  Returns: Curve object with DFs and zero rates                  │
└─────────────────────────────────────────────────────────────────┘
```

## 🗂️ Current Data Files

### BGN EUR (2025-11-21)
**File**: `marketdata/fixtures_bgn_euribor.go`

**Source**: Extracted from database on 2025-11-26
```sql
SELECT quotes FROM marketdata.curves
WHERE date='2025-11-21'
  AND source='BGN'
  AND reference_index='ESTR';
```

**Contains**:
- `BGNEstr` - 51 tenors (1W to 60Y)
- `BGNEuribor3M` - 42 tenors (3M to 50Y)
- `BGNEuribor6M` - 39 tenors (6M to 50Y) ✅ **Fixed in Phase 5**

### LCH EUR (2025-11-20)
**File**: `marketdata/fixtures_lch_euribor.go`

**Source**: Extracted from database on 2025-11-27
```sql
SELECT quotes FROM marketdata.curves
WHERE date='2025-11-20'
  AND source='LCH'
  AND reference_index='ESTR';
```

**Contains**:
- `LCHEstr` - 36 tenors (sparser than BGN)
- `LCHEuribor3M` - 27 tenors
- `LCHEuribor6M` - 25 tenors

### BGN TIBOR (2025-11-21)
**File**: `marketdata/fixtures_bgn_tibor.go`

**Source**: Extracted from database on 2025-11-23

**Contains**:
- `BGNTonar` - TONAR OIS curve
- `BGNTibor3M` - TIBOR 3M curve
- `BGNTibor6M` - TIBOR 6M curve

## 🔄 How to Update Curve Data

### Option 1: Automated Extraction (Recommended)

Use the provided Python script to extract fresh data from the database:

```bash
# Extract curves for a new date
docker exec -u airflow airflow-worker \
  python3 /Users/meenmo/Documents/workspace/molib/scripts/extract_curves_new_date.py 2025-11-25

# Output format (ready to paste into Go file):
var BGNEstr = map[string]float64{
    "1W": 1.928,
    "1M": 1.931,
    ...
}
```

### Option 2: Manual SQL Query

```sql
-- Extract ESTR quotes
SELECT quotes
FROM marketdata.curves
WHERE date='2025-11-25'
  AND source='BGN'
  AND reference_index='ESTR';

-- Extract EURIBOR3M quotes
SELECT quotes
FROM marketdata.curves
WHERE date='2025-11-25'
  AND source='BGN'
  AND reference_index='EURIBOR3M';

-- Extract EURIBOR6M quotes
SELECT quotes
FROM marketdata.curves
WHERE date='2025-11-25'
  AND source='BGN'
  AND reference_index='EURIBOR6M';
```

Then manually format into Go map syntax.

### Option 3: Dynamic Loading (Future Enhancement)

**Not implemented yet**, but could look like:

```go
// Future: Load directly from database
curves, err := data.LoadCurvesFromDB(
    curveDate,
    source,
    []string{"ESTR", "EURIBOR3M", "EURIBOR6M"},
)
```

**Why not implemented?**
- Static fixtures are faster (no DB queries at runtime)
- Static fixtures work offline
- Static fixtures are easier to test and version control
- Current use case is batch processing, not real-time

## 📝 Data Format

### Database Format (PostgreSQL JSONB)

```json
{
  "1W": 1.928,
  "2W": 1.931,
  "1M": 1.931,
  "3M": 1.92795,
  "6M": 1.91695,
  "1Y": 1.8916,
  "2Y": 1.912,
  "5Y": 2.172,
  "10Y": 2.531,
  "30Y": 2.912
}
```

### Go Format (map[string]float64)

```go
var BGNEstr = map[string]float64{
    "1W":  1.928,
    "2W":  1.931,
    "1M":  1.931,
    "3M":  1.92795,
    "6M":  1.91695,
    "1Y":  1.8916,
    "2Y":  1.912,
    "5Y":  2.172,
    "10Y": 2.531,
    "30Y": 2.912,
}
```

### Internal Format (map[float64]float64)

After parsing in `BuildCurve()`:

```go
parsed := map[float64]float64{
    0.0191781:  1.928,    // 1W = 7/365 years
    0.0833333:  1.931,    // 1M = 1/12 years
    0.25:       1.92795,  // 3M = 3/12 years
    0.5:        1.91695,  // 6M = 6/12 years
    1.0:        1.8916,   // 1Y
    2.0:        1.912,    // 2Y
    5.0:        2.172,    // 5Y
    10.0:       2.531,    // 10Y
    30.0:       2.912,    // 30Y
}
```

## 🔍 Verification

To verify where your data came from:

### Check Database

```bash
# Check what dates are available
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT date
FROM marketdata.curves
WHERE source='BGN' AND reference_index='ESTR'
ORDER BY date DESC
LIMIT 10;
"

# Check what sources are available
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT source, reference_index
FROM marketdata.curves
WHERE date='2025-11-21'
ORDER BY source, reference_index;
"
```

### Compare Fixture vs Database

```bash
# Extract from database
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT quotes->'10Y' as ten_year_rate
FROM marketdata.curves
WHERE date='2025-11-21'
  AND source='BGN'
  AND reference_index='ESTR';
" -t

# Check fixture file
grep '"10Y"' marketdata/fixtures_bgn_euribor.go
```

Should match: `2.531`

## 📅 Date Handling

**Important**: The curve date in the fixture file is **metadata only** - it's in the comment:

```go
// BGN EUR quotes for curve date 2025-11-21.  ← Just a comment
var (
    BGNEstr = map[string]float64{
        ...
    }
)
```

The **actual curve date** used for calculations comes from the test program:

```go
curveDate := time.Date(2025, 11, 21, 0, 0, 0, 0, time.UTC)  ← This matters
```

**Why?** The curve quotes themselves are just rates - they don't have timestamps. The curve date is used for:
1. Calculating spot date (date + 2 business days)
2. Generating payment schedules
3. Date adjustments (business day conventions)

## 🎯 Summary

**Current State**:
- ✅ Curve data is **static** (hardcoded in Go files)
- ✅ Extracted **manually** from PostgreSQL database
- ✅ For **specific dates**: BGN EUR (2025-11-21), LCH EUR (2025-11-20), BGN TIBOR (2025-11-21)
- ✅ Updated in **Phase 5** to fix EURIBOR6M bug

**To test different dates**:
1. Extract new data from database using provided script
2. Create new fixture file (or update existing)
3. Import and use in test program

**Database Table**: `marketdata.curves`
- **Host**: 100.127.72.74:1013
- **Database**: ficc
- **Schema**: marketdata
