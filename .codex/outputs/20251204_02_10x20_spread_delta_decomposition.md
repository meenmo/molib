10x20 Basis Spread – ficclib vs molib Closed Form and Delta
===========================================================

This note fixes a single valuation date $t_0$ and derives the closed-form receive-leg spread for the BGN EUR 10x20 basis swap in each stack:

- ficclib (Airflow or database path),
- molib (Go).

We then express the spread difference in terms of present value components.

The trade is:

- Pay: 6M EURIBOR (no spread),
- Receive: 3M EURIBOR (unknown spread $s$ in basis points),
- Discounting: ESTR OIS (BGN),
- Notional: $N$.

We work in present value per unit notional, dividing everything by $N$.

------------------------------------------------------------

1. Common notation
------------------

For a given implementation, denoted by an asterisk superscript $(*)$ with

- $(*) = \text{F}$ for ficclib,
- $(*) = \text{M}$ for molib,

we define:

- Pay leg (6M) payment dates: $T^{(*),6}_i$, for $i = 1,\dots,n_6$,
- Rec leg (3M) payment dates: $T^{(*),3}_j$, for $j = 1,\dots,n_3$,
- Accrual fractions:
  - $\delta^{(*),6}_i$ for the pay leg,
  - $\delta^{(*),3}_j$ for the rec leg,
- IBOR forwards:
  - $f^{(*),6}_i$ (6M forwards),
  - $f^{(*),3}_j$ (3M forwards),
- OIS discount factors:
  - $D^{(*)}_6(T^{(*),6}_i)$ for pay-leg payments,
  - $D^{(*)}_3(T^{(*),3}_j)$ for rec-leg payments,
  - in practice both come from the same OIS curve $D^{(*)}(t)$ evaluated at different dates.

Principal present values:

- $PV_{\text{prin}}^{(*),\text{pay}}$: pay-leg principal PV (per unit notional),
- $PV_{\text{prin}}^{(*),\text{rec}}$: rec-leg principal PV (per unit notional).

The receive-leg spread is quoted in basis points $s$, and we define the decimal form
$$
s_{\text{dec}} = \frac{s}{10{,}000}.
$$

------------------------------------------------------------

2. General NPV equation and closed form for $s$
-----------------------------------------------

For each implementation $(*)$, the unit-notional pay-leg present value is
$$
PV_{\text{pay}}^{(*)}
  = -\sum_{i=1}^{n_6} \delta^{(*),6}_i\ f^{(*),6}_i\ D^{(*)}_6\!\bigl(T^{(*),6}_i\bigr)
    + PV_{\text{prin}}^{(*),\text{pay}},
$$
and the rec-leg PV with spread $s$ is
$$
PV_{\text{rec}}^{(*)}(s)
  = \sum_{j=1}^{n_3} \delta^{(*),3}_j\ \bigl(f^{(*),3}_j + s_{\text{dec}}\bigr)\ D^{(*)}_3\!\bigl(T^{(*),3}_j\bigr)
    + PV_{\text{prin}}^{(*),\text{rec}}.
$$

The zero-NPV condition per unit notional is
$$
0 = PV_{\text{pay}}^{(*)} + PV_{\text{rec}}^{(*)}(s).
$$

Introduce the following aggregates:
$$
A^{(*)} = \sum_{j=1}^{n_3} \delta^{(*),3}_j\ D^{(*)}_3\!\bigl(T^{(*),3}_j\bigr),
$$
$$
B^{(*)} = \sum_{j=1}^{n_3} \delta^{(*),3}_j\ f^{(*),3}_j\ D^{(*)}_3\!\bigl(T^{(*),3}_j\bigr),
$$
$$
P^{(*)} = PV_{\text{pay}}^{(*)},
$$
$$
R_{\text{prin}}^{(*)} = PV_{\text{prin}}^{(*),\text{rec}}.
$$

Then we can rewrite
$$
PV_{\text{rec}}^{(*)}(s)
  = B^{(*)} + s_{\text{dec}} A^{(*)} + R_{\text{prin}}^{(*)},
$$
and the NPV equation becomes
$$
0 = P^{(*)} + B^{(*)} + s_{\text{dec}} A^{(*)} + R_{\text{prin}}^{(*)}.
$$

Solving for $s_{\text{dec}}$ yields
$$
s_{\text{dec}}^{(*)}
  = -\frac{P^{(*)} + B^{(*)} + R_{\text{prin}}^{(*)}}{A^{(*)}},
$$
and the receive-leg spread in basis points is
$$
s^{(*)}
  = 10{,}000 \times s_{\text{dec}}^{(*)}
  = -10{,}000 \times \frac{P^{(*)} + B^{(*)} + R_{\text{prin}}^{(*)}}{A^{(*)}}.
$$

This formula is common to both stacks; they differ only in how the ingredients $P^{(*)}, A^{(*)}, B^{(*)}, R_{\text{prin}}^{(*)}$ are computed.

------------------------------------------------------------

3. ficclib closed form for BGN EUR 10x20
----------------------------------------

For ficclib we denote the 10x20 spread by $s^{\text{F}}$. It is given by
$$
s^{\text{F}}
  = -10{,}000 \times
    \frac{P^{\text{F}} + B^{\text{F}} + R_{\text{prin}}^{\text{F}}}{A^{\text{F}}}.
$$

Here:

- $P^{\text{F}}$ is the pay-leg PV per unit notional using:
  - Accruals $\delta^{\text{F},6}_i = \text{ACT/360}(A^{\text{F},6}_i, B^{\text{F},6}_i)$,
  - Forwards $f^{\text{F},6}_i$ from the ficclib dual-curve 6M projection curve with SWPM-like tenor-end dates,
  - Discounts $D^{\text{F}}(T^{\text{F},6}_i)$ from the QuantLib-based ESTR OIS curve,
  - Principal PVs at effective and maturity as implemented in ficclib.

- $A^{\text{F}}$ and $B^{\text{F}}$ use:
  - Accruals $\delta^{\text{F},3}_j = \text{ACT/360}(A^{\text{F},3}_j, B^{\text{F},3}_j)$,
  - Forwards $f^{\text{F},3}_j$ from the 3M projection curve,
  - Discounts $D^{\text{F}}(T^{\text{F},3}_j)$.

- $R_{\text{prin}}^{\text{F}}$ is the rec-leg principal PV as defined in ficclib (initial and final exchanges with SWPM-aligned sign convention).

The database table pricing.basis_swap stores spreads generated from exactly these ingredients for the Airflow 10x20 runs.

------------------------------------------------------------

4. molib closed form for BGN EUR 10x20
--------------------------------------

For molib we denote the 10x20 spread by $s^{\text{M}}$. It satisfies the same structural formula
$$
s^{\text{M}}
  = -10{,}000 \times
    \frac{P^{\text{M}} + B^{\text{M}} + R_{\text{prin}}^{\text{M}}}{A^{\text{M}}}.
$$

Here:

- $P^{\text{M}}$ is the pay-leg PV per unit notional based on:
  - Accruals $\delta^{\text{M},6}_i = \text{YearFraction}(A^{\text{M},6}_i, B^{\text{M},6}_i, \text{"ACT/360"})$,
  - Forwards $f^{\text{M},6}_i$ from BuildDualCurve’s 6M projection curve, where tenor ends are constructed as Adjust(start + 6M) and accruals use YearFraction,
  - Discounts $D^{\text{M}}(T^{\text{M},6}_i)$ from the synthetic ESTR curve BuildCurve,
  - Principal PVs at effective and maturity as implemented in molib’s priceLeg function.

- $A^{\text{M}}, B^{\text{M}}$ use:
  - Accruals $\delta^{\text{M},3}_j = \text{YearFraction}(A^{\text{M},3}_j, B^{\text{M},3}_j, \text{"ACT/360"})$,
  - Forwards $f^{\text{M},3}_j$ from the 3M projection curve built by BuildDualCurve (synthetic IBOR swaps on a 3M grid),
  - Discounts $D^{\text{M}}(T^{\text{M},3}_j)$.

- $R_{\text{prin}}^{\text{M}}$ is the rec-leg principal PV: receive notional at effective, pay back at maturity, discounted with $D^{\text{M}}$.

Thus molib implements the same algebra as ficclib, but with different curves and schedules inside the aggregates.

------------------------------------------------------------

5. Algebraic spread difference
------------------------------

The exact spread difference is
$$
s^{\text{M}} - s^{\text{F}}
  = -10{,}000 \left(
      \frac{P^{\text{M}} + B^{\text{M}} + R_{\text{prin}}^{\text{M}}}{A^{\text{M}}}
      - \frac{P^{\text{F}} + B^{\text{F}} + R_{\text{prin}}^{\text{F}}}{A^{\text{F}}}
    \right).
$$

To gain intuition, consider a first-order approximation around the ficclib solution. Define differences
$$
P^{\Delta} = P^{\text{M}} - P^{\text{F}},\quad
B^{\Delta} = B^{\text{M}} - B^{\text{F}},\quad
R_{\text{prin}}^{\Delta} = R_{\text{prin}}^{\text{M}} - R_{\text{prin}}^{\text{F}},
$$
$$
A^{\Delta} = A^{\text{M}} - A^{\text{F}}.
$$

At the ficclib solution for 10x20, by definition
$$
P^{\text{F}} + B^{\text{F}} + R_{\text{prin}}^{\text{F}} = 0.
$$

Using this, a linearized expression for the spread difference is
$$
s^{\text{M}} - s^{\text{F}}
  \approx -10{,}000 \times \frac{P^{\Delta} + B^{\Delta} + R_{\text{prin}}^{\Delta}}{A^{\text{F}}}.
$$

Interpretation:

- $P^{\Delta}$ captures the difference in pay-leg PV due to:
  - Different OIS curves $D^{\text{M}}$ vs $D^{\text{F}}$,
  - Different 6M projection curves,
  - Different schedules and accruals.

- $B^{\Delta}$ captures similar differences on the rec leg’s floating PV.

- $R_{\text{prin}}^{\Delta}$ captures any difference in principal timing and discounting (usually very small in practice).

- $A^{\text{F}}$ is essentially a PV-weighted duration of the receive leg, equal to the sum of discounted accruals on the 3M leg under ficclib’s curves and schedules.

Because $A^{\text{F}}$ for a 10x20 EUR basis swap is quite large in magnitude, modest PV differences (for example, from curve-shape or schedule differences) translate into relatively small spread differences, typically on the order of a few tenths of a basis point for the BGN EUR 10x20 case.

