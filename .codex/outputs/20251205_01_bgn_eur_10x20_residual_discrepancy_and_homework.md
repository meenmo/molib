BGN EUR 10x20 residual discrepancy – diagnosis and homework
===========================================================

Scope and context
-----------------

This note records where the remaining differences come from between:

- Bloomberg SWPM (via pricing.basis_swap in the database),
- ficclib or Airflow implementation, and
- molib (Go) implementation

for BGN EURIBOR3M or EURIBOR6M 10x10 and 10x20 basis swaps, focusing on the stubborn case:

- valuation or curve date 2025-01-29,
- BGN EUR 10x20 (forward 10y, swap 20y),
- receive 3M, pay 6M, discounting ESTR OIS.

All observations below respect the constraint of not modifying anything under airflow or ficclib.


1. Current accuracy snapshot
----------------------------

After the changes already implemented in molib, the status is:

- 2025-08-27:
  - BGN EUR 10x10: molib and DB differ by about -0.005 bp
  - BGN EUR 10x20: molib and DB differ by about +0.009 bp
  - BGNS TIBOR 1x4, 2x3: differences about 0.02–0.06 bp
  - LCH EUR 10x10, 10x20: differences about 0.0–0.014 bp

These are already in the “noise” regime.

- 2025-01-29 (after fixes described below):
  - BGN EUR 10x10: molib -10.722199 bp vs DB -10.691710 bp ⇒ diff about -0.031 bp
  - BGN EUR 10x20: molib -11.659573 bp vs DB -11.573420 bp ⇒ diff about -0.086 bp
  - BGNS TIBOR 1x4, 2x3: differences about ±0.006 bp

Database values have been checked against Bloomberg SWPM; ficclib or Airflow is effectively “on top” of SWPM. The only meaningful discrepancy that was left was BGN EUR 10x20 on 2025-01-29, which started off at about -3.36 bp and is now down to about -0.09 bp.


2. Key changes already made in molib
------------------------------------

All changes are restricted to molib, with airflow and ficclib treated as the reference.

2.1 Schedule generation (previous iteration)

File: swap or basis or schedule.go

- buildSchedule now uses an end-of-month aware stepping when the leg’s RollConvention is BACKWARD_EOM:
  - end = AddMonth(start, months) (EOM aware),
  - then apply TARGET or JPN business day adjustment.
- For other roll conventions, simple AddDate(0, months, 0) is used.

This brought molib’s forward-floating schedules very close to ficclib’s build_schedule, especially for long and forward-starting swaps. This fix largely eliminated the big errors (several basis points) seen on 2025-08-27 in both EUR and JPY structures.

2.2 Dual-curve IBOR bootstrap fixed-leg daycount (previous iteration)

File: swap or basis or curve.go

- In the IBOR pseudo-discount-factor bootstrap, the fixed leg was originally using an implicit “years” accrual with Days divided by 365 as a time axis, independent of currency.
- It now chooses the fixed-leg daycount convention consistently with the currency:
  - TARGET (EUR): ACT/360,
  - JPN (JPY): ACT/365F.

The bootstrap still uses synthetic annual fixed legs, but at least the daycount is aligned with the conventions used in swap pricing and in ficclib.

2.3 Dual-curve IBOR bootstrap floating-leg daycount (this iteration)

File: swap or basis or curve.go, function evalIBORSwapNPV

- The IBOR floating leg used in the calibration was using accrual = Days / 365 for its internal synthetic swap equations.
- It now uses the same daycount convention as the eventual pricing legs:
  - TARGET (EUR): ACT/360,
  - JPN (JPY): ACT/365F.

This change is mostly a conceptual tidy-up; it has only a small numerical impact but makes the bootstrap more consistent with valuation.

2.4 Effective and maturity date alignment with Airflow or ficclib

File: swap or basis or pricing.go

Before these changes, molib used:

- spot = T+2 adjusted by Modified Following on the OIS calendar,
- effective = AddYearsWithRoll(spot, forward_tenor),
- maturity = AddYearsWithRoll(effective, swap_tenor).

AddYearsWithRoll implements a sort of backward end-of-month adjustment followed by Modified Following. For 2025-01-29 and 10x20:

- DB or Airflow effective date: 2035-01-31,
- DB or Airflow maturity date: 2055-02-01,
- original molib maturity date: 2055-01-29.

This mismatch on the long-end anchor (two days earlier, and different month handling) was a major driver of the large 10x20 discrepancy.

Now molib mirrors the Airflow basis_swap.pricing logic:

- trade_date = curve_date
- spot_date = calendar.add_business_days(trade_date, 2) on the OIS calendar (TARGET for EUR),
- unadjusted_effective = spot_date plus forward_tenor_years (plain year add),
- effective_date = AdjustFollowing(unadjusted_effective),
- unadjusted_maturity = effective_date plus swap_tenor_years,
- maturity_date = AdjustFollowing(unadjusted_maturity).

The new calendar.AdjustFollowing function is a pure Following convention:

- move forward one day at a time until the date is a business day,
- no month-preservation step (unlike Modified Following).

Result for 2025-01-29, BGN EUR 10x20:

- effective_date = 2035-01-31,
- maturity_date = 2055-02-01,

matching pricing.basis_swap and ficclib’s logic.

2.5 Diagnostic script for BGN EUR 10x20 on 2025-01-29

File: scripts or 20251204_01_diag_bgn_eur_10x20_20250129.go

This diagnostic is purely in molib or scripts, so it does not affect production.

It:

- Rebuilds BGN ESTR, EURIBOR3M, EURIBOR6M curves for curve_date = 2025-01-29 from the fixtures used by basis_swap.sh.
- Recomputes spot, effective and maturity dates using the same logic as CalculateSpread.
- Builds pay and receive leg schedules using the same buildSchedule logic as pricing.
- Computes:
  - pay leg PV at zero spread (including principal),
  - rec leg floating PV B_rec = sum delta_j f_j D_j,
  - rec leg discounting mass A_rec = sum delta_j D_j,
  - rec leg principal PV R_prin_rec (receive notional at effective, pay back at maturity),
  - and the implied spread from the closed-form formula
    s_dec = -(P_pay + B_rec + R_prin_rec) divided by A_rec.
- Confirms that this algebraic spread matches the numeric root from CalculateSpread within numerical precision.
- Buckets A_rec and B_rec into:
  - 10–15y, 15–30y, and “other” buckets based on payment time from curve date (in years).

Diagnostic output for 2025-01-29 after the maturity fix:

- Spread:
  - molib: -11.659573 bp,
  - database: -11.573420 bp,
  - difference: about -0.086 bp.

- Aggregates (per unit notional):
  - P_pay ≈ -0.002473,
  - B_rec ≈ 0.287994,
  - A_rec ≈ 12.734512,
  - R_prin_rec ≈ -0.270674.

The implied spread is:

- s_dec ≈ -(P_pay + B_rec + R_prin_rec) divided by A_rec ≈ -0.00116596,
- s_bp = 10 000 times s_dec ≈ -11.659573 bp,

which matches CalculateSpread exactly.

Bucketed rec leg contributions (per unit notional):

- 10–15 years:
  - A_10_15 ≈ 3.550529 (about 27.88 percent of A_rec),
  - B_10_15 ≈ 0.098577 (about 34.23 percent of B_rec).

- 15–30 years:
  - A_15_30 ≈ 9.053261 (about 71.09 percent of A_rec),
  - B_15_30 ≈ 0.187156 (about 64.99 percent of B_rec).

- other (pre-10y tail):
  - A_other ≈ 0.130722 (about 1.03 percent of A_rec),
  - B_other ≈ 0.002261 (about 0.79 percent of B_rec).

So, approximately:

- about 70 percent of the discounted accrual mass A_rec sits in 15–30y,
- about 28 percent sits in 10–15y,
- the rest is negligible for this particular structure.

This matters when translating a PV difference between stacks into a spread difference:

- A residual spread difference of about 0.086 bp, given A_rec ≈ 12.73, corresponds to a per-unit-notional PV discrepancy of roughly
  - Delta B_rec ≈ - Delta s_dec times A_rec ≈ 1.1 times 10 to the power of -4,
  - which is about 0.00011 or roughly 0.04 percent of B_rec.
- Given the bucket weights, any remaining curve-shape mismatch that causes this tiny PV discrepancy is almost entirely driven by 10–30y forwards, with roughly a 2:1 split between 15–30y and 10–15y.

In other words, the “big” 3.3 bp error on 2025-01-29 10x20 was dominated by the maturity-date convention mismatch and is now largely gone. What remains is a tiny long-end curve-shape difference and not a structural convention bug.


3. Diagnosis summary
--------------------

From the perspective of the algebraic PV equation, molib and ficclib or Airflow now agree:

- Both have:
  - pay leg: 6M EURIBOR, ACT/360, BACKWARD_EOM schedule, OIS discounting, principal exchanged,
  - rec leg: 3M EURIBOR, ACT/360, same conventions, plus an unknown spread s in basis points.
- Spread solves:
  - P_pay + PV_rec(s) = 0, where PV_rec(s) = B_rec + s_dec times A_rec + R_prin_rec.

The earlier large discrepancies were driven by:

1. Schedule generation mismatch:
   - Ignoring BACKWARD_EOM and using simple AddDate in molib,
   - now fixed by using AddMonth plus Modified Following, mirroring ficclib’s effective EOM behavior for floating legs.

2. Date-anchor mismatch for 10x20 on 2025-01-29:
   - DB or Airflow maturity at 2055-02-01 (Following),
   - molib maturity at 2055-01-29 (Modified Following with month preservation),
   - now fixed by adopting the same “spot plus years, then Following” convention as basis_swap.pricing in Airflow.

3. Minor differences in IBOR bootstrap daycounts:
   - Floating leg calibration previously using Days divided by 365,
   - now using ACT/360 for EUR and ACT/365F for JPY, consistent with pricing legs.

After these changes:

- For all the benchmark dates and tenors the differences between molib and pricing.basis_swap are roughly:
  - less than 0.06 bp on 2025-08-27 and 2025-10-31,
  - less than 0.1 bp on 2025-01-29 (including the previously problematic BGN EUR 10x20).

The remaining small discrepancies are consistent with the intentional simplifications in molib’s curve construction:

- OIS curve built from a synthetic monthly par-swap grid with interpolation in rate space and Days divided by 365 axis,
- IBOR curves built from synthetic 3M and 6M swaps with annual fixed legs and simplified instruments,
- not using QuantLib calendars directly.


4. Homework and possible enhancements
-------------------------------------

If the goal is to drive molib even closer to SWPM or ficclib (for example, to try to get all benchmark tenors within a few hundredths of a basis point), there are several potential enhancements. They are ordered roughly from “high value per complexity” to “full QuantLib clone”.

4.1 OIS curve bootstrap refinements

Target: swap or basis or curve.go, BuildCurve.

Ideas:

1. Time axis:
   - Move from Days divided by 365 to ACT/365F year fractions everywhere in the OIS bootstrap.
   - This would better match ficclib’s OISBootstrapper, which uses ACT/365F and log-discount-factor interpolation on that axis.

2. Instrument structure:
   - Instead of a generic monthly par-swap grid, consider a minimal set of realistic OIS instruments:
     - ESTR par swaps with annual fixed and daily compounded floating legs,
     - consistent with ficclib’s use of compute_maturity and QuantLib OIS instruments.
   - Even a sparse set of realistic pillars (eg 1y, 2y, 3y, 5y, 7y, 10y, 15y, 20y, 30y) interpolated on ACT/365F could further reduce differences in the 10–30y region.

3. Interpolation:
   - Explore using log-discount-factor or log-forward interpolation instead of linear in par rate across tenors.
   - This would better mimic how QuantLib-based bootstraps behave between pillars.

These changes will be most visible in long-end OIS discount factors and will directly impact both legs’ PVs through D(t).

4.2 IBOR curve construction refinements

Target: swap or basis or curve.go, BuildDualCurve and evalIBORSwapNPV.

Ideas:

1. Instrument set:
   - Move away from purely synthetic IBOR swaps on a fixed 3M or 6M grid with annual fixed legs, and towards:
     - a deposit strip for the very short end,
     - a proper set of EURIBOR3M and EURIBOR6M swaps using:
       - fixed legs with ACT/360 daycount and appropriate annual or semi-annual payments,
       - floating legs aligned with the same BACKWARD_EOM schedule as pricing.
   - This would mirror ficclib’s IborCurveBuilder, which calibrates to realistic IBOR instruments using QuantLib schedules and ACT/360.

2. Time axis and interpolation:
   - Use the same ACT/365F year fraction axis for calibration and log-discount-factor interpolation between IBOR pillars.
   - Keep the explicit floating daycount ACT/360 in both calibration and valuation, as already done in this iteration.

3. Integration with pricing schedules:
   - Ensure that the accrual and payment dates used in calibration are consistent with those used in actual pricing when computing forwards, so that:
     - P_3(t) and P_6(t) are “seen” at the same dates in both calibration and valuation.

These refinements mainly affect forward rates f_3 and f_6 in the 10–30y region, where the bucketed diagnostic shows most of the sensitivity.

4.3 Calendar alignment with QuantLib

Target: molib or calendar.

Currently:

- molib uses hand-coded TARGET and JPN holiday lists and weekend logic,
- ficclib delegates calendars to QuantLib (TARGET and Japan).

Possible enhancements:

1. Introduce a small wrapper over QuantLib’s TARGET and Japan calendars in Go:
   - Or, if direct QuantLib bindings are not desired in molib, auto-generate holiday tables from ficclib’s calendars.
2. Ensure that AddBusinessDays, Adjust (Modified Following) and AdjustFollowing behave identically to ficclib’s calendar operations for the relevant date range.

This is already “almost right” thanks to the hard-coded holiday sets; any remaining difference would be subtle and mostly around long-dated edges.

4.4 Schedule generation convergence

Target: swap or basis or schedule.go.

The current buildSchedule logic in molib already emulates the ficclib build_schedule behavior quite closely for floating legs, using:

- effective and maturity dates anchored correctly,
- step forward by PayFrequency months,
- BACKWARD_EOM via AddMonth for EUR and JPY floating legs,
- Modified Following business day adjustments.

For perfection, one could:

1. Mirror the exact logic of ficclib’s forward schedule generation:
   - Preserve end-of-month behavior only if the effective date itself is end-of-month.
   - Otherwise, keep the day-of-month constant across periods, as ficclib does in its forward generation branch for floating legs.

2. Add a small unit test suite inside molib (not relying on airflow) that compares schedule dates for a few canonical forward-starting swaps (1x4, 2x3, 10x10, 10x20) against a precomputed set of dates taken from ficclib or from the database.

Given current discrepancies are already below 0.1 bp, the schedule differences are now likely second-order; still, this would close a methodological gap.

4.5 Regression harness around pricing.basis_swap

Even without changing the core methods further, it would be valuable to build a small regression harness in molib that:

1. Reads a sample of rows from pricing.basis_swap for:
   - multiple curve dates (e.g. 2025-01-29, 2025-08-27, 2025-10-31),
   - all reference pairs of interest (BGN EUR 10x10, 10x20; BGNS TIBOR 1x4, 2x3; and possibly more).

2. For each row:
   - rebuilds the appropriate fixtures into the data package via generate_fixtures.py,
   - runs CalculateSpread using the correct conventions and sources,
   - records the spread difference and the implied PV difference in a small report.

3. Enforces a simple threshold, for example:
   - a “green” region for absolute difference below 0.1 bp,
   - a “yellow” region for 0.1–0.5 bp,
   - a “red” region above 0.5 bp.

This would give a quick daily or on-demand check that molib and pricing.basis_swap remain aligned after any further changes.


5. Practical recommendation
---------------------------

Right now, for the key BGN EUR and BGNS TIBOR structures and dates investigated:

- molib and Airflow or ficclib agree within a few hundredths of a basis point for nearly all cases, and within about 0.09 bp even for the previously problematic BGN EUR 10x20 on 2025-01-29.

The main structural issues (schedules and date anchors) are resolved. What remains are tiny curve-shape differences driven by the deliberate simplifications in molib’s OIS and IBOR bootstraps.

Therefore:

- For risk and production pricing comparisons against the database, the current molib implementation looks “good enough” and methodologically consistent.
- If the goal is to build a near bit-for-bit SWPM clone in Go, the homework above provides a roadmap:
  - gradually replace the synthetic OIS and IBOR bootstraps with more realistic instrument sets and ACT/365F time axes,
  - converge the schedule and calendar logic fully to QuantLib’s behavior,
  - and surround everything with a light regression harness anchored on pricing.basis_swap.

