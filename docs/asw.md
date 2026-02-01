# Asset Swap Spread (ASW)

This document describes two ASW calculation methods supported by `molib`:

| Method | JSON `asw_type` | PV01 Notional | Use Case |
|--------|-----------------|---------------|----------|
| **Par-Par** | `"PAR-PAR"` | Par (face value) | Bloomberg YAS default |
| **MMS** | `"MMS"` | Dirty Price | Matched-Maturity Spread |

> **Core Equation (both methods)**
>
> $$\text{ASW}_{bp} = \frac{P_{OIS} - P_{\text{dirty}}}{PV01_{bp}}$$
>
> The **numerator** is identical; only the **PV01 denominator** differs.

---

## Part 1: Matched-Maturity Spread (MMS)

### 1.1 Overview

MMS uses the **dirty price** as the notional for PV01 calculation. This reflects the actual capital deployed by the investor.

> **MMS Equation**
>
> $$\text{MMS}_{bp} = \frac{P_{OIS} - P_{\text{dirty}}}{PV01^{MMS}_{bp}}$$
>
> where
>
> $$PV01^{MMS}_{bp} = P_{\text{dirty}} \cdot \sum_j \delta_j \cdot D(T_j) \cdot 10^{-4}$$

### 1.2 Notation

- $P_{\text{dirty}}$: dirty price in currency units (e.g., $N \times \text{price\%}$).
- $P_{OIS}$: PV of bond cashflows discounted on the OIS curve.
- $\delta_j$: year fraction for swap period $j$ (e.g., ACT/360).
- $D(T_j)$: discount factor to swap payment date $T_j$.
- $\mathcal{A}_{swap}$: annuity factor, $\sum_j \delta_j \cdot D(T_j)$ (unitless).

### 1.3 MMS PV01 Derivation

The MMS PV01 represents the PV of receiving 1bp on the swap schedule, scaled by the **dirty price** instead of par:

$$PV01^{MMS}_{bp} = P_{\text{dirty}} \cdot \mathcal{A}_{swap} \cdot 10^{-4}$$

### 1.4 Relationship to Par-Par ASW

Since both methods share the same numerator:

$$\text{MMS}_{bp} = \text{ASW}^{ParPar}_{bp} \times \frac{N}{P_{\text{dirty}}}$$

| Bond Price | Ratio $N / P_{\text{dirty}}$ | Effect |
|------------|------------------------------|--------|
| Discount ($P < N$) | $> 1$ | MMS > Par-Par |
| Par ($P = N$) | $= 1$ | MMS = Par-Par |
| Premium ($P > N$) | $< 1$ | MMS < Par-Par |

### 1.5 Example Calculation (MMS)

**Given:**
- Settlement: 2026-01-12
- Notional ($N$): \$1,000,000
- Dirty Price: 98.66211% $\Rightarrow P_{\text{dirty}} = \$986,621.10$
- $P_{OIS}$: \$1,043,007.39
- Annuity Factor ($\mathcal{A}_{swap}$): 11.56

**Step 1: Compute MMS PV01**

$$PV01^{MMS}_{bp} = \$986,621.10 \times 11.56 \times 10^{-4} = \$1,140.45$$

**Step 2: Compute MMS Spread**

$$\text{MMS}_{bp} = \frac{\$1,043,007.39 - \$986,621.10}{\$1,140.45} = \frac{\$56,386.29}{\$1,140.45} = 49.44 \text{ bp}$$

### 1.6 When to Use MMS

- **Per-dollar-invested analysis**: MMS reflects the spread earned on actual capital deployed.
- **Deep discount/premium bonds**: More meaningful spread comparison across bonds with different prices.
- **Bloomberg MMS field**: Matches the "MMS Spread" shown in Bloomberg YASN.

---

## Part 2: Par-Par ASW

### 2.1 Overview

Par-Par ASW uses the **par (face value)** as the notional for PV01 calculation. This is the Bloomberg default and the traditional market convention.

> **Par-Par Equation**
>
> $$\text{ASW}^{ParPar}_{bp} = \frac{P_{OIS} - P_{\text{dirty}}}{PV01^{ParPar}_{bp}}$$
>
> where
>
> $$PV01^{ParPar}_{bp} = N \cdot \sum_j \delta_j \cdot D(T_j) \cdot 10^{-4}$$

### 2.2 The Upfront Term and Amortization

In a **Par-Par** asset swap, there is an **upfront payment** at inception because the swap notional ($N$) differs from the dirty price ($P_{\text{dirty}}$):

$$\text{Upfront}^{ParPar} = N - P_{\text{dirty}}$$

**Cashflow at inception:**

| Cashflow | Amount |
|----------|--------|
| Investor pays for bond | $-P_{\text{dirty}}$ |
| Swap initial exchange | $+N$ (receive par) |
| **Net at inception** | $N - P_{\text{dirty}}$ |

**Cashflow at maturity:**

| Cashflow | Amount |
|----------|--------|
| Bond principal received | $+N$ |
| Swap final exchange | $-N$ (pay par) |
| **Net at maturity** | **0** |

**Key insight:** The upfront payment ($N - P_{\text{dirty}}$) must be **amortized** into the running spread.

#### Algebraic Derivation: How the Upfront Gets Amortized

**Step 1: Full Valuation Equation**

At inception, the asset swap package value is:

$$V = \underbrace{P_{OIS}}_{\substack{\text{Bond PV} \\ \text{(OIS discounted)}}} - \underbrace{P_{\text{dirty}}}_{\substack{\text{Price} \\ \text{paid}}} + \underbrace{(N - P_{\text{dirty}})}_{\substack{\text{Upfront} \\ \text{received}}} + \underbrace{s \cdot N \cdot \mathcal{A}_{swap}}_{\substack{\text{Spread stream} \\ \text{PV}}}$$

**Step 2: Set Fair Value ($V = 0$) and Rearrange**

$$0 = P_{OIS} - P_{\text{dirty}} + (N - P_{\text{dirty}}) + s \cdot N \cdot \mathcal{A}_{swap}$$

Move the spread term to the left:

$$-s \cdot N \cdot \mathcal{A}_{swap} = P_{OIS} - P_{\text{dirty}} + N - P_{\text{dirty}}$$

$$-s \cdot N \cdot \mathcal{A}_{swap} = P_{OIS} + N - 2P_{\text{dirty}}$$

**Step 3: Observe the Cancellation**

Notice that the upfront term $(N - P_{\text{dirty}})$ partially cancels with $-P_{\text{dirty}}$:

$$P_{OIS} - P_{\text{dirty}} + (N - P_{\text{dirty}}) = P_{OIS} - P_{\text{dirty}} + N - P_{\text{dirty}}$$

Rearranging:

$$= P_{OIS} - 2P_{\text{dirty}} + N$$

This doesn't simplify nicely... **unless** we recognize that the standard Par-Par convention already **nets** the upfront into the formula.

**Step 4: The Standard Par-Par Convention**

In practice, the Par-Par ASW is quoted as:

$$s \cdot N \cdot \mathcal{A}_{swap} = P_{OIS} - P_{\text{dirty}}$$

This is equivalent to saying: **the upfront payment is already embedded in the numerator**.

We can decompose the numerator to see both components:

$$P_{OIS} - P_{\text{dirty}} = \underbrace{(P_{OIS} - N)}_{\substack{\text{Bond richness/} \\ \text{cheapness vs par}}} + \underbrace{(N - P_{\text{dirty}})}_{\substack{\text{Upfront} \\ \text{amortized}}}$$

**Step 5: The Two-Component Interpretation**

Solving for $s$:

$$s = \frac{P_{OIS} - P_{\text{dirty}}}{N \cdot \mathcal{A}_{swap}} = \frac{(P_{OIS} - N) + (N - P_{\text{dirty}})}{N \cdot \mathcal{A}_{swap}}$$

$$\boxed{s = \underbrace{\frac{P_{OIS} - N}{N \cdot \mathcal{A}_{swap}}}_{\text{Pure spread}} + \underbrace{\frac{N - P_{\text{dirty}}}{N \cdot \mathcal{A}_{swap}}}_{\text{Amortized upfront}}}$$

| Component | Meaning |
|-----------|---------|
| $(P_{OIS} - N)$ | How much the bond's OIS PV differs from par (credit/liquidity) |
| $(N - P_{\text{dirty}})$ | The upfront payment converted to a running spread |

#### Numerical Example

**Given:**
- $N = \$1,000,000$
- $P_{\text{dirty}} = \$986,621$ (discount bond)
- $P_{OIS} = \$1,043,007$
- $\mathcal{A}_{swap} = 11.56$

**Decomposition:**

$$\text{Pure spread} = \frac{\$1,043,007 - \$1,000,000}{\$1,000,000 \times 11.56} = \frac{\$43,007}{\$11,560,000} = 37.2 \text{ bp}$$

$$\text{Amortized upfront} = \frac{\$1,000,000 - \$986,621}{\$1,000,000 \times 11.56} = \frac{\$13,379}{\$11,560,000} = 11.6 \text{ bp}$$

$$\text{Total ASW} = 37.2 + 11.6 = 48.8 \text{ bp}$$

**Interpretation:** Of the 48.8 bp Par-Par ASW spread:
- **37.2 bp** reflects the bond trading rich to OIS (credit/liquidity)
- **11.6 bp** is the amortized upfront payment (price discount from par)

### 2.3 Setup and Notation

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

### 2.4 The Valuation Components

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

### 2.5 Deriving the Solution

The "PV gap to running spread" logic

Under the market quoting convention used here, ASW is defined as the constant spread (in bp) on the swap schedule that converts the PV gap between the bond's OIS PV and its market dirty price into a running spread:

$$
P_{OIS} - P_{\text{dirty}} = \text{ASW}_{bp}\cdot PV01_{bp}
$$

Equivalently (decimal units):

$$s_{\text{dec}} \cdot \mathcal{A}_{swap} = P_{OIS} - P_{\text{dirty}}$$

$$s_{\text{dec}} = \frac{P_{OIS} - P_{\text{dirty}}}{\mathcal{A}_{swap}}$$

and $\text{ASW}_{bp} = 10^{4}\,s_{\text{dec}}$.

---

### 2.6 The Expanded Analytical Solution

Substituting the definitions from Section 2.4 back into the equation, we get the fully expanded formula used in pricing engines:

$$s_{\text{dec}} = \frac{ \left[ \sum_{i=1}^{m} cN\alpha_i D(t_i) + ND(T) \right] - P_{\text{dirty}} }{ \sum_{j=1}^{M} N\delta_j D(T_j) }$$

- **Numerator:** Uses **Bond** Schedule ($t_i$). Represents the "Upfront Value Gap."

- **Denominator:** Uses **Swap** Schedule ($T_j$). Represents the "Exchange Rate" to convert that gap into a running spread.


---

### 2.7 Example Calculation (Par-Par)

**Given:** (same as MMS example)
- Settlement: 2026-01-12
- Notional ($N$): \$1,000,000
- Dirty Price: 98.66211% $\Rightarrow P_{\text{dirty}} = \$986,621.10$
- $P_{OIS}$: \$1,043,007.39
- Annuity Factor ($\mathcal{A}_{swap}$): 11.56

**Step 1: Compute Par-Par PV01**

$$PV01^{ParPar}_{bp} = \$1,000,000 \times 11.56 \times 10^{-4} = \$1,155.92$$

**Step 2: Compute Par-Par Spread**

$$\text{ASW}^{ParPar}_{bp} = \frac{\$1,043,007.39 - \$986,621.10}{\$1,155.92} = \frac{\$56,386.29}{\$1,155.92} = 48.78 \text{ bp}$$

---

### 2.8 Intuitive Interpretation (The Trader's View)

Think of the Asset Swap Spread ($s$) as an **Amortized Discount**.

1. **The Instant Profit:** If you buy a bond at $90$ ($P_{\text{dirty}}$) that is theoretically worth $100$ ($P_{OIS}$), you have generated **$10 of Instant Value**.

2. **The Mechanism:** Instead of taking that \$10 cash immediately, the Asset Swap spreads it out over the life of the trade.

3. **The Result:** The ASW is that \$10 lump sum divided by the swap's PV01 (or annuity).


$$Spread = \frac{\text{The Discount You Found (PV)}}{\text{The Duration of the Trade (Annuity)}}$$

---

## Part 3: Practical Notes

### 3.1 Comparison Summary

| Aspect | Par-Par ASW | MMS ASW |
|--------|-------------|---------|
| PV01 Notional | $N$ (par) | $P_{\text{dirty}}$ |
| PV01 Formula | $N \cdot \mathcal{A} \cdot 10^{-4}$ | $P_{\text{dirty}} \cdot \mathcal{A} \cdot 10^{-4}$ |
| Discount Bond | Lower spread | Higher spread |
| Premium Bond | Higher spread | Lower spread |
| At Par | Equal | Equal |

### 3.2 What's *not* in these formulas

This closed-form relationship is the core intuition, but production ASW calculators typically have additional details (depending on market and system):

- Settlement conventions (spot lag, calendars) and ex-dividend handling
- Stub conventions (both bond and swap schedules)
- Multi-curve effects (projection curve vs discount curve)
- Day count variants (e.g., 30/360 vs 30E/360 vs 30U/360)

---

## Part 4: molib Implementation

### 4.1 Code Map

- **ASW calculator (core):** `bond.ComputeASWSpread(...)` in [`bond/asw.go`](../bond/asw.go)
  - Accepts `ASWType` field: `bond.ASWTypeParPar` or `bond.ASWTypeMMS`
  - Computes `PVBondRF` by discounting bond cashflows using the provided `swap.DiscountCurve`
  - Computes `PV01` using the selected notional (par or dirty price)
- **ASW type constants:** [`bond/asw.go`](../bond/asw.go)
  - `ASWTypeParPar = "PAR-PAR"`
  - `ASWTypeMMS = "MMS"`
- **Bond cashflow type:** [`bond/types.go`](../bond/types.go) (`bond.Cashflow`)
- **Swap schedule generation (for PV01):** `swap.GenerateSchedule(...)` in [`swap/common.go`](../swap/common.go)
  - Uses `market.LegConvention` pay frequency / roll / calendar rules; includes OIS-specific chaining logic.
- **Discount curve interface:** [`swap/types.go`](../swap/types.go) (`swap.DiscountCurve`)
- **Leg convention presets used by fixtures/inputs:** [`instruments/swaps/conventions.go`](../instruments/swaps/conventions.go)
  - Examples: `swaps.ESTRFloating`, `swaps.EURIBOR6MFloating`, `swaps.SOFRFloating`, `swaps.KRXCD91DFloating`

### 4.2 CLI Usage

**Command:**
```bash
go run ./cmd/aswspread/main.go -input /path/to/input.json
```

**Input JSON Schema:**
```json
{
  "asw_type": "MMS",
  "curve_date": "2026-01-09",
  "curve_type": "OIS",
  "floating_swap_leg": "SOFRFloating",
  "curve_settlement_lag_days": 1,
  "curve_quotes": [...],
  "bonds": [...]
}
```

| Field | Values | Description |
|-------|--------|-------------|
| `asw_type` | `"PAR-PAR"`, `"MMS"` | Selects ASW calculation method |

### 4.3 Test Fixtures

- `bond/testdata/input_asw_spread_ois.json` - OIS curve (ESTR)
- `bond/testdata/input_asw_spread_irs.json` - IRS curve (EURIBOR)
- `bond/testdata/input_asw_spread_krx.json` - KRX CD IRS curve (KRW)
- `cmd/aswspread/testdata/sample_input.json` - SOFR OIS (USD)

### 4.4 Fixture-Driven Tests

- [`bond/asw_test.go`](../bond/asw_test.go) - `TestComputeASW_FromFixture`
- [`bond/asw_test.go`](../bond/asw_test.go) - `TestASW_MMS_RatioRelationship` (verifies MMS = ASW * N/P)
- [`bond/asw_test.go`](../bond/asw_test.go) - `TestASW_MMS_AtPar` (verifies MMS = ASW when P = N)
