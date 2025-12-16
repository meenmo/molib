# Next Steps: Basis Swap Refinement

**Date:** 2025-12-12
**Context:** 
We have significantly improved the JPY 5x5 Basis Swap (TIBOR6M vs TONAR) pricing. By correcting the JPY TIBOR fixed leg frequency to **Semi-Annual** (from Annual) and implementing a robust OIS bootstrap with correct day counts, we reduced the spread discrepancy against SWPM from **~2.6 bp** to **~2.32 bp**. The system is now numerically stable and aligns structurally with market conventions.

We have prepared a "Phase 6" diagnostic tool (`scripts/20251212_06_diag_jpy_5x5_swpm_dfs.go`) that allows us to bypass our internal curve bootstrapping and price the swap using externally provided Discount Factors. This is crucial to isolate whether the remaining error comes from **Valuation** (how we calculate cashflows, dates, day counts) or **Curve Construction** (how we interpolate and solve for rates).

---

## Prompt for Next Session

Copy and paste the following prompt to resume work:

```markdown
We are investigating the remaining ~2.3bp discrepancy on the JPY 5x5 Basis Swap (TIBOR6M/TONAR). The infrastructure is ready for "Phase 6: Deep Diagnostics".

**Current Status:**
- Model Spread: 55.03 bp
- Target Spread: 57.35 bp
- Residual Gap: ~2.32 bp

**Objective:** Isolate the source of error by injecting exact SWPM Discount Factors.

**Instructions:**
1.  **Data Ingestion:** I will provide (or you should ask for) the exact **TONAR Discount Factors** and **TIBOR6M Pseudo-Discount Factors** (or Zero Rates) from SWPM for the curve date 2025-12-10.
2.  **Execution:** Update the maps `tonarDFs` and `tiborDFs` in `scripts/20251212_06_diag_jpy_5x5_swpm_dfs.go` with this data.
3.  **Analysis:** Run the script (`go run scripts/20251212_06_diag_jpy_5x5_swpm_dfs.go`).
    *   **If PV is near zero:** The error is in **Curve Construction**. Proceed to **Phase 4** and implement **Monotone Convex Spline Interpolation** to smooth the forward curve.
    *   **If PV is significantly non-zero:** The error is in **Valuation Logic**. Investigate `swap/basis/schedule.go`, exact day count alignments (especially Turn-of-Year), and OIS compounding precision.
```
