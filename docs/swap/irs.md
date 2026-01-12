

# 📘 IRS (Fixed vs IBOR Float) — Pricing Note

> **Master Equation (NPV)**
>
> $$\text{NPV} = PV_{\text{rec}} - PV_{\text{pay}}$$
>
> A swap is “par” when NPV = 0.

---

### 1. Setup and Notation

We consider a vanilla interest rate swap (IRS):

- One leg pays **fixed** at rate $K$.
- The other leg pays **floating** (e.g., EURIBOR 3M/6M) plus an optional spread $s$.
- Discounting is done on a discount curve (typically OIS post-2020), and floating forwards come from a projection curve (multi-curve).

**Notation**

- $N$: notional
- $t_i$: fixed-leg payment dates, $i=1,\dots,m$
- $\alpha_i$: fixed-leg accrual factor for $[t_{i-1}, t_i]$
- $T_j$: floating-leg payment dates, $j=1,\dots,M$
- $\delta_j$: floating-leg accrual factor for $[T_{j-1}, T_j]$
- $D(\cdot)$: discount factor from the **discount curve**
- $P^{proj}(\cdot)$: discount factor from the **projection curve** (used to infer forwards)
- $K$: fixed rate (decimal)
- $s$: floating spread (decimal)

---

### 2. Cashflows and PV

**Fixed leg (pay/receive fixed coupons)**

Coupon at $t_i$:

$$CF^{fix}_i = N \cdot \alpha_i \cdot K$$

Present value:

$$PV_{fix} = \sum_{i=1}^{m} \left(N\alpha_iK\right)\,D(t_i)$$

**Floating leg (pay/receive IBOR + spread)**

The model uses a **simple forward** over each accrual period:

$$F_j = \frac{\frac{P^{proj}(T_{j-1})}{P^{proj}(T_j)} - 1}{\delta_j}$$

Coupon at $T_j$:

$$CF^{flt}_j = N \cdot \delta_j \cdot (F_j + s)$$

Present value:

$$PV_{flt} = \sum_{j=1}^{M} \left(N\delta_j(F_j+s)\right)\,D(T_j)$$

---

### 3. Spread PV (Linear in s)

Because the spread enters the coupon linearly, the floating leg PV can be decomposed as:

$$
PV_{flt}(s) = PV_{flt}(0) + s \cdot \underbrace{\sum_{j=1}^{M} N\delta_j D(T_j)}_{\mathcal{A}_{flt}}
$$

where $\mathcal{A}_{flt}$ is the “floating-leg annuity” (currency per 1.00 of spread).

If you solve for the spread $s$ that makes the swap par (NPV = 0), a common rearrangement is:

$$
0 = PV_{rec} - PV_{pay} \quad\Rightarrow\quad s = \frac{PV_{fix} - PV_{flt}(0)}{\mathcal{A}_{flt}}
$$

This assumes the spread is applied to the floating leg and $PV_{flt}$ is taken with the same sign as that leg; if the floating leg is the pay leg, the sign convention flips.

---

### 4. Par Fixed Rate (Single-Curve Intuition)

In a single-curve world (projection = discount), the floating leg PV is often written:

$$PV_{flt} \approx N\left(D(T_0) - D(T_M)\right)$$

Define the fixed-leg annuity:

$$\mathcal{A}_{fix} := \sum_{i=1}^{m} \alpha_i D(t_i)$$

The **par fixed rate** is then:

$$K_{par} = \frac{PV_{flt}}{N\mathcal{A}_{fix}} \approx \frac{D(T_0)-D(T_M)}{\mathcal{A}_{fix}}$$

---

### 5. molib Implementation Links (Code Map)

- **Trade builder (curves + dates):** `swap.InterestRateSwap(...)` in [`swap/api.go`](../../swap/api.go)
- **Swap PV decomposition:** `swap.PVByLeg(...)` / `swap.NPV(...)` in [`swap/common.go`](../../swap/common.go)
  - Forward definition used: `forwardRate(...)` in [`swap/common.go`](../../swap/common.go)
  - Schedule generation: `swap.GenerateSchedule(...)` in [`swap/common.go`](../../swap/common.go)
- **Curve builders:**
  - OIS curve: `curve.BuildCurve(...)` in [`swap/curve/curve.go`](../../swap/curve/curve.go)
  - IBOR projection curve (dual-curve): `curve.BuildProjectionCurve(...)` in [`swap/curve/projection.go`](../../swap/curve/projection.go)
- **Conventions/presets:** [`instruments/swaps/conventions.go`](../../instruments/swaps/conventions.go)
- **CLI (NPV runner):** `cmd/npv` (subcommand: `irs`) in [`cmd/npv/main.go`](../../cmd/npv/main.go)
  - Sample input: `cmd/npv/testdata/irs.json`
