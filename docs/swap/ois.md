

# 📘 OIS (Overnight Index Swap) — Pricing Note

> **Master Equation (NPV)**
>
> $$\text{NPV} = PV_{\text{rec}} - PV_{\text{pay}}$$
>
> An OIS is “par” when NPV = 0.

---

### 1. Setup and Notation

We consider a vanilla OIS:

- One leg pays **fixed** at rate $K$.
- The other leg pays **overnight floating** (e.g., €STR, TONAR) compounded in arrears.
- Discounting is typically on the same overnight curve.

**Notation**

- $N$: notional
- $t_i$: fixed-leg payment dates, $i=1,\dots,m$
- $\alpha_i$: fixed-leg accrual factor
- $T_j$: floating-leg payment dates, $j=1,\dots,M$
- $\delta_j$: floating-leg accrual factor
- $D(\cdot)$: OIS discount factor
- $K$: fixed rate (decimal)

---

### 2. PV Formulas (Standard View)

**Fixed leg**

$$PV_{fix} = \sum_{i=1}^{m} \left(N\alpha_iK\right)\,D(t_i)$$

**Overnight floating leg (conceptual)**

Over the accrual period $[T_{j-1}, T_j]$ the compounded overnight return is:

$$\Pi_j = \prod_{k \in \text{days in }[T_{j-1},T_j]} \left(1 + r_k\,\Delta_k\right)$$

A standard OIS floating coupon can be expressed as:

$$CF^{ois}_j = N\left(\Pi_j - 1\right)$$

Discounted PV:

$$PV_{ois} = \sum_{j=1}^{M} CF^{ois}_j\,D(T_j)$$

In a single-curve OIS framework, the floating leg PV is commonly summarized by:

$$PV_{ois} \approx N\left(D(T_0) - D(T_M)\right)$$

---

### 3. Telescoping Form (Single-Curve, Idealized)

If the floating coupon for period $[T_{j-1},T_j]$ is represented using the same curve used for discounting (and ignoring payment lags / convexity details), a common identity is:

$$
CF^{ois}_j \approx N\left(\frac{D(T_{j-1})}{D(T_j)} - 1\right)
$$

Then the discounted PV of that coupon is:

$$
CF^{ois}_j\,D(T_j) \approx N\left(D(T_{j-1}) - D(T_j)\right)
$$

Summing across periods telescopes:

$$
PV_{ois} \approx \sum_{j=1}^{M} N\left(D(T_{j-1}) - D(T_j)\right) = N\left(D(T_0) - D(T_M)\right)
$$

This is the intuition behind the “float leg PV = DF(start) − DF(end)” shortcut.

---

### 4. Par Fixed Rate

Define the fixed-leg annuity:

$$\mathcal{A}_{fix} := \sum_{i=1}^{m} \alpha_i D(t_i)$$

Then:

$$K_{par} = \frac{PV_{ois}}{N\mathcal{A}_{fix}} \approx \frac{D(T_0)-D(T_M)}{\mathcal{A}_{fix}}$$

---

### 5. molib Implementation Links (Code Map)

- **Implementation note:** `molib` prices the overnight floating leg using simple forwards inferred from the OIS curve for each coupon period (via `forwardRate(...)`), which matches the telescoping intuition under idealized conditions but will differ from full daily-compounding implementations when conventions (lags/calendars/day counts) differ.
- **Trade builder (curve + dates):** `swap.InterestRateSwap(...)` in [`swap/api.go`](../../swap/api.go)
- **Swap PV decomposition:** `swap.PVByLeg(...)` / `swap.NPV(...)` in [`swap/common.go`](../../swap/common.go)
  - Schedule generation: `swap.GenerateSchedule(...)` in [`swap/common.go`](../../swap/common.go)
  - Forward used in code path: `forwardRate(...)` in [`swap/common.go`](../../swap/common.go)
- **OIS curve builder:** `curve.BuildCurve(...)` in [`swap/curve/curve.go`](../../swap/curve/curve.go)
- **Conventions/presets:** [`instruments/swaps/conventions.go`](../../instruments/swaps/conventions.go)
- **CLI (NPV runner):** `cmd/npv` (subcommand: `ois`) in [`cmd/npv/main.go`](../../cmd/npv/main.go)
  - Sample input: `cmd/npv/testdata/ois.json`
