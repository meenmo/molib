# Basis Swap Spread Refinement Plan

**Date:** 2025-12-12
**Status:** In Progress
**Objective:** Reduce the residual spread discrepancy on JPY 5x5 Basis Swaps (TIBOR6M vs TONAR) from ~2.3 bp to < 1.0 bp against SWPM/ficclib benchmarks.

```go run examples/basis_swap_spread_solver.go```
---

## 1. Current State

We have successfully refactored the `molib` curve engine to be currency-aware and structurally closer to market conventions.

### Achievements
*   **OIS Bootstrap:**
    *   Moved from generic par-swap logic to a realistic Fixed Leg schedule (Annual coupons).
    *   Implemented `curveDayCount` awareness (ACT/365F for JPY).
    *   Added a Newton-Raphson solver for discount factors, robust to irregular quote spacing.
*   **IBOR Bootstrap:**
    *   Updated `evalIBORSwapNPV` to use currency-specific fixed leg frequencies.
    *   **Crucial Fix:** JPY TIBOR calibration now correctly uses **Semi-Annual (6M)** fixed legs (ACT/365F), aligning with standard JPY IRS market conventions. TARGET remains Annual (12M).
*   **Results:**
    *   The fair spread for the 5x5 JPY basis swap improved from **54.75 bp** to **55.03 bp**.
    *   Target (SWPM): **57.35 bp**.
    *   Remaining Gap: **~2.32 bp**.
    *   PV error at target spread reduced by ~65%.
    *   **SWPM Cashflow (at 57.351 bp):**
        *   Pay Leg PV: **-245,513** (for 10MM notional)
        *   Receive Leg PV: **+255,332** (for 10MM notional)
        *   Total PV: **+9,819** (for 10MM notional)

---

## 2. Hypothesis for Remaining Discrepancy

The remaining ~2.3 bp gap is likely due to second-order effects in curve construction or specific convention mismatches.

### H1: Interpolation Method (High Probability)
*   **Current:** Log-Linear on Discount Factors (or piecewise log-linear). This effectively assumes constant forward rates between pillars.
*   **Market Standard:** Monotone Convex Splines (Hagan-West) on Zero Rates or Log-Linear on Zero Rates.
*   **Impact:** TIBOR quotes have large gaps (e.g., 2Y -> 3Y). A "constant forward" assumption creates "sawtooth" forwards. If SWPM uses splines, their forward curve will be smooth. Since the 5x5 swap depends heavily on the 5Y-10Y forward sector, differences in forward curve shape (even with identical par swap prices) can shift the valuation of the floating leg significantly.

### H2: OIS Floating Leg Precision (Medium Probability)
*   **Current:** The OIS bootstrap solves $1 = PV_{fixed} + D(T)$. This implicitly assumes the floating leg is valued exactly at par (1.0).
*   **Reality:** OIS floating legs drift slightly from par due to daily compounding vs. simple rate accrual, especially if there's a spread or if the compounding convention (geometric vs arithmetic) isn't perfectly netted.
*   **Action:** We may need to explicitly model the OIS floating leg (daily compounding) during the bootstrap, rather than assuming it equals 1.0.

### H3: Exact Day Count / Calendar Alignment (Low-Medium Probability)
*   **Current:** We use standard JPN calendar logic.
*   **Risk:** "Turn-of-year" effects in Japan are significant. If SWPM's grid includes specific end-of-year dates that we interpolate over (or vice versa), or if their holiday file differs slightly (e.g., banking holidays vs exchange holidays), accrual fractions will diverge.

### H4: Quote Input Mismatch (Low Probability)
*   **Check:** Confirm that the 20Y, 30Y, 40Y quotes we use match the exact instruments SWPM uses. Sometimes "20Y" means "20Y swap starting today" vs "20Y swap starting spot". We assume Spot start.

---

## 3. Work Plan

### Phase 4: Interpolation Upgrade
1.  **Refactor `Curve` struct:** Abstract the interpolation logic.
2.  **Implement Monotone Convex Spline:** Add a spline interpolator for Zero Rates.
3.  **Test:** Re-run the JPY 5x5 probe. If the forwards smooth out, the spread should move closer to 57 bp.

### Phase 5: OIS Bootstrap Refinement
1.  **Explicit Floating Leg:** Modify `bootstrapDiscountFactors` to calculate $PV_{float}$ using daily compounding steps (approximated or exact) instead of assuming $PV_{float} = 1.0$.
2.  **Impact:** This typically adds fractions of a basis point, but for spread trades, precision matters.

### Phase 6: Deep Diagnostics (Fixture Mode)
1.  **DF Injection:** Create a `NewCurveFromDFs` constructor.
2.  **SWPM Dump:** Manually extract the exact Discount Factors and Forward Rates from SWPM for the curve date.
3.  **Injection Test:** Feed SWPM DFs into `molib`. If the pricing matches exactly, the Valuation engine is perfect, and the error is purely in Bootstrap. If it still differs, the error is in the Schedule/DayCount logic.

## 4. Next Actions
*   **Immediate:** Proceed to **Phase 6 (Deep Diagnostics)**. Before writing complex spline logic, we must confirm that *if* we had the correct DFs, we would match the price. This isolates "Construction Error" from "Valuation Error".
