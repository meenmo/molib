# Plain Vanilla Interest Rate Swap Pricing Examples

This command demonstrates swap structure specifications for plain vanilla (fixed vs floating) interest rate swaps across 8 major reference rates.

## Usage

```bash
go run ./cmd/swapprice
```

## Reference Rates Covered

### 1. **EURIBOR 3M** (EUR)
- Fixed 2.50% vs EURIBOR 3M
- Fixed: Annual, 30/360
- Float: Quarterly, ACT/360
- Discount: ESTR OIS curve

### 2. **EURIBOR 6M** (EUR)
- Fixed 2.55% vs EURIBOR 6M
- Fixed: Semi-annual, 30/360
- Float: Semi-annual, ACT/360
- Discount: ESTR OIS curve

### 3. **TIBOR 3M** (JPY)
- Fixed 0.80% vs TIBOR 3M
- Fixed: Semi-annual, ACT/365F
- Float: Quarterly, ACT/365F
- Discount: TONAR OIS curve

### 4. **TIBOR 6M** (JPY)
- Fixed 0.85% vs TIBOR 6M
- Fixed: Semi-annual, ACT/365F
- Float: Semi-annual, ACT/365F
- Discount: TONAR OIS curve

### 5. **ESTR** (EUR OIS)
- Fixed 2.30% vs ESTR
- Fixed: Annual, ACT/360
- Float: Daily compounded, annual payment
- Discount: ESTR (single curve)

### 6. **TONAR** (JPY OIS)
- Fixed 0.50% vs TONAR
- Fixed: Annual, ACT/365F
- Float: Daily compounded, annual payment
- Discount: TONAR (single curve)

### 7. **SOFR** (USD OIS)
- Fixed 4.50% vs SOFR
- Fixed: Annual, ACT/360
- Float: Daily compounded, annual payment
- Discount: SOFR (single curve)

### 8. **CD 91-day** (KRW)
- Fixed 3.20% vs CD 91-day
- Fixed: Quarterly, ACT/365
- Float: Quarterly, ACT/365
- Discount: KOFR OIS curve

## Swap Conventions

Each example demonstrates:
- **Trade Date**: T+2 spot convention (T+1 for KRW)
- **Tenor**: 10 years
- **Notional**: Market-standard amounts
- **Day Count**: Market-standard conventions
- **Payment Frequency**: Market-standard frequencies
- **Business Day Adjustment**: Modified Following
- **Dual-Curve Framework**: OIS discounting with IBOR projection

## Implementation Status

This program currently demonstrates swap **structure specification** using the `swap/benchmark` package conventions.

To calculate actual NPV values, you would need to:
1. Build OIS discount curves from market data
2. Build IBOR projection curves (for non-OIS swaps)
3. Generate cash flow schedules
4. Apply dual-curve pricing methodology

See `cmd/basiscalc` for a working example of dual-curve pricing for basis swaps.

## Architecture

The program uses:
- `swap/benchmark/LegConvention`: Convention-based leg specifications
- `swap/benchmark/SwapSpec`: Complete swap specification
- `calendar` package: Business day calendars (TARGET, JPN, USD, KRW)
- Market-standard conventions from `swap/benchmark/presets.go`

## Related Tools

- `cmd/basiscalc`: Basis swap pricing with actual market data
- `scripts/basis_swap.sh`: Generate curve fixtures from database
- `swap/basis`: Dual-curve pricing engine
