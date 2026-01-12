

# 📘 KRX CD91 IRS (Clearinghouse) — Pricing Note

> **Master Equation (NPV)**
>
> For direction = **REC** (receive fixed / pay float):
>
> $$\text{NPV} = PV_{\text{fixed}} - PV_{\text{float}}$$
>
> For direction = **PAY**:
>
> $$\text{NPV} = PV_{\text{float}} - PV_{\text{fixed}}$$

---

### 1. Schedule and Day Count (KRX Rulebook Style)

- Payment frequency: **quarterly** (every 3 months)
- Calendar: **KR**
- Day count: **ACT/365F** (implemented as `days/365`)
- End-of-month rule:
  - If the effective date is end-of-month, coupon dates are the **last business day of month**.
  - Otherwise, coupon dates are business-day adjusted normally.

---

### 2. Curve Bootstrap (Par Swap Quotes → DF → Zero)

Let the curve settlement date be $S$.

We work on quarterly dates $t_0=S, t_1, t_2, \dots$ with:

$$\delta_k = \frac{\text{days}(t_{k-1}, t_k)}{365}$$

For maturity $t_n$, the quoted par swap rate is $R_n$ (decimal).

The par swap condition used for bootstrap is:

$$R_n \sum_{k=1}^{n} \delta_k DF(t_k) = 1 - DF(t_n)$$

Solve recursively:

$$DF(t_n) = \frac{1 - R_n\sum_{k=1}^{n-1}\delta_k DF(t_k)}{1 + R_n\delta_n}$$

In `molib`, if a tenor is not explicitly quoted, par swap rates are linearly interpolated between adjacent quoted nodes before bootstrap.

Zero rate (continuously-compounded, in %):

$$z(t) = -\frac{\ln DF(t)}{\tau(t)} \cdot 100,\qquad \tau(t)=\frac{\text{days}(S,t)}{365}$$

Discounting to an arbitrary date uses the interpolated zero rate:

$$DF(t)=\exp\left(-\tau(t)\cdot\frac{z(t)}{100}\right)$$

---

### 3. KRX Floating Cashflows (First Fixing + Implied Forwards)

Coupon dates: $T_1, T_2, \dots, T_M$ (quarterly).

Accrual for period $j$:

$$\delta_j = \frac{\text{days}(T_{j-1},T_j)}{365}$$

**First period rate**: taken from a reference fixing feed (CD91) on:

$$\text{fixing date} = \text{(prior coupon date)} - 1\text{ business day}$$

**Subsequent periods** use the curve-implied forward:

$$F_j = \frac{DF(T_{j-1})/DF(T_j) - 1}{\delta_j}$$

Floating coupon:

$$CF^{flt}_j = N\cdot \delta_j \cdot F_j$$

---

### 4. PV by Leg

Fixed coupon (fixed rate $K$, decimal):

$$CF^{fix}_j = N\cdot \delta_j \cdot K$$

Leg PVs:

$$PV_{\text{fixed}} = \sum_j CF^{fix}_j \cdot DF(T_j)$$

$$PV_{\text{float}} = \sum_j CF^{flt}_j \cdot DF(T_j)$$

---

### 5. molib Implementation Links (Code Map)

- **KRX curve bootstrap:** [`swap/clearinghouse/krx/curve.go`](../../../swap/clearinghouse/krx/curve.go) (`BootstrapCurve`, `DF`, `ZeroRateAt`)
- **KRX IRS cashflows + NPV:** [`swap/clearinghouse/krx/cashflow.go`](../../../swap/clearinghouse/krx/cashflow.go) (`legCashflows`, `PVByLeg`, `NPV`)
- **KRX schedule helpers:** [`swap/clearinghouse/krx/helpers.go`](../../../swap/clearinghouse/krx/helpers.go)
- **Reference rate feed (CD91):** [`calendar/korea.go`](../../../calendar/korea.go) (`ReferenceIndex Feed`, `DefaultReferenceFeed`)
- **CLI (NPV runner):** `cmd/npv` (subcommand: `krx-irs`) in [`cmd/npv/main.go`](../../../cmd/npv/main.go)
  - Sample input: `cmd/npv/testdata/krx-irs.json`
