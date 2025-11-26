Project state:
- Shared calendars in calendar/ with TARGET and JPN holiday sets (2000–2060), business-day logic (Modified Following, add business days, add years with EOM).
- Benchmarks in swap/benchmark: reference rates (ESTR, EURIBOR3M/6M, TONAR, TIBOR3M/6M), leg conventions (daycount, frequencies, fixing/pay lags, roll, calendars), and SwapSpec.
- Basis module in swap/basis: basic curve bootstrap (BuildCurve), schedule builder, valuation (floating legs + principals, OIS discounting), spread solver (bisection), tenor parsing, fixtures in swap/basis/data (BGN EUR, BGNS TIBOR, LCH EUR).
- CLI runner cmd/basiscalc to compute spreads:
  - BGN EUR (curve date 2025-11-21): 10x10, 10x20
  - BGNS TIBOR (curve date 2025-11-21): 1x4, 2x3
  - LCH EUR (curve date 2025-11-20): 10x10, 10x20

Current output (go run ./cmd/basiscalc):
- BGN EUR: -0.42 bp (10x10), -0.28 bp (10x20)
- BGNS TIBOR: -2.70 bp (1x4), -2.98 bp (2x3)
- LCH EUR: -4.59 bp (10x10), -5.11 bp (10x20)
Targets from Airflow logs: 
- BGN EUR 
  10x10: 4.023073, 10x20: -4.941096 
- LCH EURIBOR
  10x10: -3.872002, 10x20: -4.426513.
- BGNS TIBOR
  1x4: -2.285949, 2x3: -2.513270

How to test: from /Users/meenmo/Documents/workspace/molib, run `go run ./cmd/basiscalc` and compare spreads to targets.

Next steps:
- Align bootstrap with ficclib: proper OIS bootstrap (spot lag, accrual), projection curves from par quotes, interpolation matching ficclib.
- Verify schedule conventions (reset/pay lags, roll, daycount).

Airflow DAG context (dags/pricing/basis_swap.py):
- DAG 'pricing_basis_swap_bgn': tasks 'tibor_basis_swap' (pay TIBOR6M, rec TIBOR3M, ois TONAR, tenor pairs 1x4, 2x3, curve source BGNS, discounting BGN) and 'euribor_basis_swap' (pay EURIBOR6M, rec EURIBOR3M, ois ESTR, tenor pairs 10x10, 10x20, source BGN). Trade_date = curve_date from logical_date.
- DAG 'pricing_basis_swap_lch': pay EURIBOR6M, rec EURIBOR3M, ois ESTR, tenor pairs 10x10, 10x20, source LCH; curve_date parsed from LCH file (2025-11-20 in log).
- Tasks call include/pricing/basis_swap.calculate_spreads -> load curves from marketdata.curves, build curve set, compute spreads, store to pricing.basis_swap.
