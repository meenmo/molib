---

# 📘 Par Swap Spread — Solver Note

> **Master Equation**
>
> Find a constant spread $s$ (added to a target leg) such that:
>
> $$\text{NPV}(s) = 0$$

---

### 1. Setup

We have a two-leg swap and want to solve for a **running spread** on one leg (the “target leg”) so the whole trade is fair at inception.

- Let $\text{NPV}_0$ be the NPV when the target-leg spread is zero.
- A constant spread adds a linear PV term because each coupon is proportional to $s$.

---

### 2. Linearization

Use decimal spread $s$ (e.g., 10bp = 0.0010).

For each target-leg period $j$:

$$\Delta CF_j = N \cdot \delta_j \cdot s$$

PV contribution:

$$\Delta PV_j = (N\delta_j s)\,D(T_j)$$

Sum over periods:

$$\Delta PV(s) = s \cdot \sum_{j} N\delta_j D(T_j)$$

Define the **PV01 per unit decimal rate** as the derivative of NPV with respect to the target-leg spread:

$$PV01_{\text{dec}} := \frac{\partial\,\text{NPV}}{\partial s}$$

For a plain running spread added to coupon cashflows, this reduces to:

- target = **receive** leg: \(PV01_{dec} = +\sum_j N\delta_j D(T_j)\)
- target = **pay** leg: \(PV01_{dec} = -\sum_j N\delta_j D(T_j)\)

So:

$$\text{NPV}(s) = \text{NPV}_0 + s \cdot PV01_{\text{dec}}$$

Solve $\text{NPV}(s)=0$:

$$s = -\frac{\text{NPV}_0}{PV01_{\text{dec}}}$$

If you want the spread in **bp**:

$$s_{\text{bp}} = \frac{s}{10^{-4}} = -\frac{\text{NPV}_0}{PV01_{\text{bp}}}$$

where:

$$PV01_{\text{bp}} := PV01_{\text{dec}}\cdot 10^{-4}$$

---

### 3. OIS Basis Special Case (molib)

For OIS basis swaps where both legs are the **same overnight index** but come from different venues/curves, `molib` can compute basis as a **par-rate difference** instead of doing cross-curve NPV root-finding.

---

### 4. molib Implementation Links (Code Map)

- **Core solver:** `swap.SolveParSpread(...)` in [`swap/common.go`](../../swap/common.go)
  - PV01 calc: `pv01TargetLegPerDec(...)` in [`swap/common.go`](../../swap/common.go)
- **Trade wrapper:** `(*swap.SwapTrade).SolveParSpread(...)` in [`swap/api.go`](../../swap/api.go)
- **OIS basis helper:** `swap.SolveOISBasisSpread(...)` and `ComputeOISParRateWithDiscount(...)` in [`swap/common.go`](../../swap/common.go)
- **CLI runner:** [`cmd/parswapspread/main.go`](../../cmd/parswapspread/main.go)
  - Sample input: `cmd/parswapspread/testdata/input.json`
  - Run: `go run ./cmd/parswapspread/main.go -input cmd/parswapspread/testdata/input.json`
