

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

### AUD AONIA OIS

Use `AONIA` (alias `RBACOR`, the Bloomberg index ticker is `RBACOR Index`)
for the AUD overnight cash rate swap. This adds OIS support; AUD BBSW IRS
conventions are not supplied.

Conventions transcribed from the supplied Bloomberg AUD SWAP-OIS screens:

| Setting | Fixed leg | Floating leg |
| --- | --- | --- |
| Day count | ACT/365F | ACT/365F |
| Payment frequency | Annual | Annual |
| Business day adjustment | Modified Following | Modified Following |
| Roll / generation | Backward EOM | Backward EOM |
| Payment delay | 2 AU business days | 2 AU business days |
| Reset | — | Daily, in arrears |
| Fixing lag / rate cutoff | — | 0 / 1 AU business day |
| Calculation / fixing calendar | AU | AU / AU |

Settlement is T+1 AU business day. Both accrual boundaries and payment dates
are adjusted. Spreads are added outside the projected overnight return
(the screen's “Excl Sprd”). Go presets are `swaps.AONIAFixed`,
`swaps.AONIAFloating`, and `swaps.OISAONIA`. As with existing presets,
remove principal flags for a vanilla coupon-only OIS when using the Go API;
the par-rate and OIS NPV CLIs already do so.

`calendar.AU` implements Sydney banking holidays, including the August Bank
Holiday and October Labour Day, and known 2022/2026/2027 exceptional holidays.
Rules: [NSW public holidays](https://www.nsw.gov.au/about-nsw/public-holidays).
[RITS operating days](https://www.rba.gov.au/payments-and-infrastructure/rits/business-hours.html)
are a different calendar. Future one-off holidays require maintenance. This
calendar has not been reconciled against Bloomberg's licensed AU calendar.

Example bootstrap input (illustrative quotes, not a market snapshot):

```json
{
  "curve_date": "2026-11-10",
  "currency": "AUD",
  "index": "AONIA",
  "curve_quotes": {"1M": 4.1, "1Y": 4.2, "2Y": 4.3, "5Y": 4.6895}
}
```

Run `go run ./cmd/bootstrap -input input.json`. The default settlement date
is `2026-11-11`. For `cmd/parswaprate`, set `floating_rate_index` to `AONIA`;
for `cmd/npv ois`, set `ois_index` to `AONIA`.

Validation covers registry settings, holiday/payment/cutoff dates, T+1 trade
construction, bootstrap diagnostics and par-rate solving. The existing engine
projects overnight coupons from period DF ratios; the cutoff is schedule
metadata, not a daily historical fixing/lockout calculation. Its sequential
bootstrap uses the existing delayed-payment approximation; the illustrative
curve has repricing drift below 0.05 bp. Bloomberg price/NPV parity has not
been established from these convention screens alone.

