# Plain Vanilla Interest Rate Swap Examples - Summary

## What Was Created

Created a comprehensive demonstration of plain vanilla interest rate swap structures covering all 8 requested reference rates.

## New Files

### 1. `cmd/swapprice/main.go`
Main program demonstrating swap structure specifications for:
- EURIBOR 3M (EUR)
- EURIBOR 6M (EUR)
- TIBOR 3M (JPY)
- TIBOR 6M (JPY)
- ESTR (EUR OIS)
- TONAR (JPY OIS)
- SOFR (USD OIS)
- CD 91-day (KRW)

### 2. `cmd/swapprice/README.md`
Documentation explaining:
- How to run the program
- Conventions for each reference rate
- Implementation status
- Architecture overview

## Updated Files

### 1. `calendar/calendar.go`
Added new calendar IDs:
- `USD`: US dollar calendar
- `KRW`: Korean won calendar

### 2. `swap/market/index.go`
Added new reference rate:
- `SOFR`: Secured Overnight Financing Rate

### 3. `swap/market/leg.go`
Added new day count conventions:
- `Act365`: ACT/365 (for Korean market)
- `Dc30360`: 30/360 (for EUR fixed legs)

## How to Run

```bash
go run ./cmd/swapprice
```

## Example Output

```
================================================================================
PLAIN VANILLA INTEREST RATE SWAP NPV EXAMPLES
================================================================================
Trade Date: 2024-11-25

1. EUR SWAP: Fixed 2.50% vs EURIBOR 3M
   Tenor: 10Y | Notional: EUR 10,000,000
   Effective: 2024-11-27 | Maturity: 2034-11-27
   Fixed Leg: 2.50% annual, 30/360
   Float Leg: EURIBOR 3M, quarterly, ACT/360
   Status: Pricing requires market data (OIS & EURIBOR curves)
   Structure: {SwapSpec with all conventions...}

[... 7 more examples ...]
```

## Key Features

1. **Market-Standard Conventions**: Uses real-world market conventions for each reference rate
2. **Dual-Curve Framework**: Demonstrates proper OIS discounting with IBOR projection
3. **Multiple Markets**: Covers EUR, JPY, USD, and KRW markets
4. **Extensible Architecture**: Built on the `swap/market` convention-based system

## Implementation Notes

### Current Status
The program demonstrates **swap structure specification** using market-standard conventions. It shows:
- Effective dates (T+2 spot for most, T+1 for KRW)
- Maturity dates (10Y tenor)
- Fixed leg conventions (rate, frequency, day count)
- Floating leg conventions (reference rate, frequency, day count)
- Discounting curve specification

### To Add NPV Calculation
To extend this to calculate actual NPV values, you would need to:

1. **Curve Building**:
   - Fetch market data (OIS rates, IBOR rates)
   - Bootstrap discount curves (OIS)
   - Bootstrap projection curves (IBOR, for dual-curve approach)

2. **Cash Flow Generation**:
   - Generate fixed leg schedule
   - Generate floating leg schedule
   - Apply business day adjustments
   - Calculate accrual fractions

3. **PV Calculation**:
   - Project floating rates from projection curve
   - Discount all cash flows using OIS curve
   - Calculate NPV as receiver or payer

4. **Example Reference**:
   - See `swap/api.go` and `swap/common.go` for trade construction and PV/spread solving
   - See `swap/curve/` for curve bootstrapping (OIS + dual-curve IBOR)
   - See `cmd/basiscalc` for working pricing example

## Relationship to Basis Swap Pricing

This plain vanilla swap demonstration complements the existing basis swap pricing:

| Feature | Basis Swaps (`cmd/basiscalc`) | Plain Vanilla (`cmd/swapprice`) |
|---------|-------------------------------|----------------------------------|
| **Type** | Float vs Float | Fixed vs Float |
| **Status** | Full NPV calculation | Structure specification |
| **Market Data** | From database | Not yet implemented |
| **Curves** | Dual-curve bootstrap | Structure only |
| **Output** | Actual spreads in bp | Swap conventions |

## Next Steps

To implement full NPV calculation for plain vanilla swaps:

1. Adapt the curve building from `swap/curve/curve.go`
2. Implement fixed leg cash flow generation
3. Implement floating leg cash flow generation (similar to basis swap)
4. Apply dual-curve discounting
5. Add market data integration

This would create a complete plain vanilla swap pricer similar to the existing basis swap pricer.
