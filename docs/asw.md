# ASW Spread (OIS Version)

> **The Equation**
> 
> $$\text{ASW}_{bp} \approx \frac{P_{OIS} - P_{\text{dirty}}}{PV01_{bp}}$$
> 
> **Market convention in this note (and in `molib`):**
> 
> ASW spread is positive when the bond is “cheap” versus the OIS curve, i.e., when $P_{OIS} > P_{\text{dirty}}$.

---

### 1. Setup and Notation

We consider a package where an investor buys a fixed-rate bond and enters an asset swap to exchange the fixed coupons for OIS floating + spread ($s$).

**Crucial Market Detail**:

The bond pays on its own schedule (e.g., Annual, ACT/360), while the spread $s$ is paid on the swap schedule (e.g., Quarterly, Act/360). We must distinguish between these two timelines.

**Notation:**

- $N$: notional / face value (e.g., 100 or 1,000,000).
    
- **Bond Leg:** $t_i$ (dates) and $\alpha_i$ (accrual factors).
    
- **Swap Leg:** $T_j$ (dates) and $\delta_j$ (accrual factors).
    
- $D(t)$: OIS discount factor from settlement date to $t$.
    
- $P_{\text{dirty}}$: dirty price paid for the bond **in currency units** (e.g., $P_{\text{dirty}} = N \times \text{price\%}$).
    
- $P_{OIS}$: PV of the bond cashflows discounted on the OIS curve (also in currency units).
    
- $\mathcal{A}_{swap}$: annuity PV for the **swap** schedule, $\sum_j N\delta_j D(T_j)$ (currency per 1.00 of rate).
- $PV01_{bp}$: PV of **receiving 1bp** on the swap schedule:
  $$ PV01_{bp} = 10^{-4}\,\mathcal{A}_{swap} = \sum_j N\delta_j\cdot 10^{-4}\cdot D(T_j) $$
    

---

### 2. The Valuation Components

A. The "Fair" Price of the Bond ($P_{OIS}$)

First, we value the bond's fixed coupons and principal using the OIS curve. This is what the bond is theoretically worth in a risk-free world.

$$P_{OIS} = \sum_{i=1}^{m} (c \cdot N \cdot \alpha_i) \cdot D(t_i) + N \cdot D(T)$$

where $T$ is the maturity date.

B. The Value of the Spread Stream ($PV_{spread}$)

The spread is paid on the swap schedule ($T_j$). If the spread is quoted in **decimal rate units** ($s_{\text{dec}}$, e.g., 10bp = 0.0010):

$$PV_{spread}(s_{\text{dec}}) = \sum_{j=1}^{M} (s_{\text{dec}} \cdot N \cdot \delta_j) \cdot D(T_j)$$

We factor out $s$ to isolate the Swap Annuity ($\mathcal{A}_{swap}$):

$$PV_{spread}(s_{\text{dec}}) = s_{\text{dec}} \cdot \underbrace{\sum_{j=1}^{M} (N \cdot \delta_j) \cdot D(T_j)}_{\mathcal{A}_{swap}}$$

So, we can write:

$$PV_{spread}(s_{\text{dec}}) = s_{\text{dec}} \cdot \mathcal{A}_{swap}$$

---

### 3. Deriving the Solution

The "PV gap to running spread" logic

Under the market quoting convention used here, ASW is defined as the constant spread (in bp) on the swap schedule that converts the PV gap between the bond’s OIS PV and its market dirty price into a running spread:

$$
P_{OIS} - P_{\text{dirty}} = \text{ASW}_{bp}\cdot PV01_{bp}
$$

Equivalently (decimal units):

$$s_{\text{dec}} \cdot \mathcal{A}_{swap} = P_{OIS} - P_{\text{dirty}}$$

$$s_{\text{dec}} = \frac{P_{OIS} - P_{\text{dirty}}}{\mathcal{A}_{swap}}$$

and $\text{ASW}_{bp} = 10^{4}\,s_{\text{dec}}$.

---

### 4. The Expanded Analytical Solution

Substituting the definitions from Section 2 back into the equation, we get the fully expanded formula used in pricing engines:

$$s_{\text{dec}} = \frac{ \left[ \sum_{i=1}^{m} cN\alpha_i D(t_i) + ND(T) \right] - P_{\text{dirty}} }{ \sum_{j=1}^{M} N\delta_j D(T_j) }$$

- **Numerator:** Uses **Bond** Schedule ($t_i$). Represents the "Upfront Value Gap."
    
- **Denominator:** Uses **Swap** Schedule ($T_j$). Represents the "Exchange Rate" to convert that gap into a running spread.
    

---

### 5. Intuitive Interpretation (The Trader's View)

Think of the Asset Swap Spread ($s$) as an **Amortized Discount**.

1. **The Instant Profit:** If you buy a bond at $90$ ($P_{\text{dirty}}$) that is theoretically worth $100$ ($P_{OIS}$), you have generated **$10 of Instant Value**.
    
2. **The Mechanism:** Instead of taking that \$10 cash immediately, the Asset Swap spreads it out over the life of the trade.
    
3. **The Result:** The ASW is that \$10 lump sum divided by the swap’s PV01 (or annuity).
    

$$Spread = \frac{\text{The Discount You Found (PV)}}{\text{The Duration of the Trade (Annuity)}}$$

---

### 6. Practical Notes (What’s *not* in this formula)

This closed-form relationship is the core intuition, but production ASW calculators typically have additional details (depending on market and system):

- Settlement conventions (spot lag, calendars) and ex-dividend handling
- Stub conventions (both bond and swap schedules)
- Multi-curve effects (projection curve vs discount curve)
- Day count variants (e.g., 30/360 vs 30E/360 vs 30U/360)

---

### 7. molib Implementation Links (Code Map)

This note maps directly to the `molib` implementation:

- **ASW calculator (core):** `bond.ComputeASWSpread(...)` in [`bond/asw.go`](../bond/asw.go)
  - Computes `PVBondRF` by discounting the provided bond cashflows (`bond.Cashflow`) using the provided `swap.DiscountCurve` (`DF(date)`).
  - Computes `PV01` as the PV of receiving **1bp** on the floating leg using the **swap schedule** (not the bond schedule).
- **Bond cashflow type:** [`bond/types.go`](../bond/types.go) (`bond.Cashflow`)
- **Swap schedule generation (for PV01):** `swap.GenerateSchedule(...)` in [`swap/common.go`](../swap/common.go)
  - Uses `market.LegConvention` pay frequency / roll / calendar rules; includes OIS-specific chaining logic.
- **Discount curve interface:** [`swap/types.go`](../swap/types.go) (`swap.DiscountCurve`)
- **Leg convention presets used by fixtures/inputs:** [`instruments/swaps/conventions.go`](../instruments/swaps/conventions.go)
  - Examples: `swaps.ESTRFloating`, `swaps.EURIBOR6MFloating`, `swaps.KRXCD91DFloating`
- **Example curve bootstraps used by fixture runners:**
  - EUR simple curve builders: [`swap/curve/curve.go`](../swap/curve/curve.go) (`curve.BuildCurve`, `curve.BuildIBORDiscountCurve`)
  - KRX CD IRS curve (KRW): [`swap/clearinghouse/krx/curve.go`](../swap/clearinghouse/krx/curve.go) (`krx.BootstrapCurve`)
- **Official runner (CLI):** [`cmd/aswspread/main.go`](../cmd/aswspread/main.go)
  - Sample inputs (fixtures): `bond/testdata/input_asw_spread_ois.json`, `bond/testdata/input_asw_spread_irs.json`, `bond/testdata/input_asw_spread_krx.json`
- **Fixture-driven regression runner:** [`bond/asw_test.go`](../bond/asw_test.go) (`TestComputeASW_FromFixture`)
