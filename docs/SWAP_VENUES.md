# Swap Venues and Pricing Engines

This document summarizes how molib prices swaps across different data sources and clearing houses.

## OTC Swaps (BGN / LCH Data Sources)

### Engine

- Trade construction: swap/InterestRateSwap (swap/api.go)
- Scheduling: swap/GenerateSchedule (swap/common.go)
- Valuation: swap/NPV and swap/PVByLeg (swap/common.go)
- Par spread solving: swap/SolveParSpread (swap/common.go)

### Curves

- OIS discount curve: swap/curve/BuildCurve
- IBOR projection curve: swap/curve/BuildProjectionCurve
  - Overnight indices project directly off the OIS curve
  - IBOR indices build a dual curve bootstrapped using OIS discounting

### Conventions / Presets

- Primitive leg/spec types: swap/market
- Market-standard leg conventions and common trade presets: instruments/swaps

### Data Sources and Clearing Houses

- **Data Sources**: BGN (Bloomberg Generic), LCH (London Clearing House), Tradition, etc.
  - Identifies where conventions and market quotes originate
- **Clearing Houses**: OTC (generic bilateral), LCH, EUREX, CME, etc.
  - Determines venue-specific rules (e.g., spot lag)

### Clearing House Defaults

- Spot lag default:
  - OTC: T+2 (override via InterestRateSwapParams.SpotLagDays)
  - KRX: T+1

## KRX (KRW swaps)

KRX-style KRW swaps use a dedicated, legacy engine.

- Package: swap/clearinghouse/krx
- Curve bootstrap: single-curve par-swap bootstrap from KRW swap quotes
- Cashflows/NPV: clearing-house-specific implementation (not the generic OIS+IBOR engine)

Clearing house defaults:
- Spot lag is typically T+1 (handled in examples)

