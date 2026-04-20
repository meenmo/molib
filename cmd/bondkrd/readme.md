# BondKRD — Key Rate Duration Engine

## Overview

`bondkrd`는 Bloomberg Wave 방법론 기반의 Key Rate Duration(KRD) 산출 엔진이다.
Par yield curve → zero curve bootstrapping → cashflow repricing → KRD 계산의 전체 파이프라인을 Go로 구현한다.

---

## 1. Zero Curve Bootstrapping

### 1.1 Input

`pricing.curve_data`에서 제공하는 par yield curve를 사용한다.

| 파라미터 | 예시 |
|---|---|
| `coupon_frequency` | 2 (semi), 1 (annual), 4 (quarterly) |
| `day_count` | ACT/ACT, ACT/365, ACT/360 |
| `curve` | [{0.25, 3.69}, {0.5, 3.71}, ...] |

### 1.2 Day Count Fraction

Day count convention에 따라 tenor를 보정하는 함수:

$$
\text{dcf}(\tau, \text{dc}) = \begin{cases}
\tau \times \frac{365}{360} & \text{if dc = ACT/360} \\
\tau & \text{otherwise (ACT/ACT, ACT/365)}
\end{cases}
$$

SOFR OIS처럼 ACT/360을 사용하는 커브에서는 1년이 360일 기준이므로,
동일 tenor에 대해 실제 이자 발생 기간이 `365/360`배 길어진다.

### 1.3 Bootstrapping Algorithm

Step size: $h = 1 / \text{freq}$ (semi-annual이면 0.5Y, annual이면 1.0Y)

Grid points: $\tau_i = i \times h$, where $i = 1, 2, \ldots, N$

각 grid point에서 par yield $c_i$ (%)로부터 discount factor $D_i$를 순차적으로 구한다:

$$
\text{cpn}_i = \frac{c_i}{100} \times \text{dcf}(h, \text{dc})
$$

$$
D_i = \frac{1 - \text{cpn}_i \times \sum_{j=1}^{i-1} D_j}{1 + \text{cpn}_i}
$$

여기서:
- $\text{cpn}_i$는 각 쿠폰 기간의 이자율 (day count 보정 포함)
- $\sum_{j=1}^{i-1} D_j$는 이전까지 누적된 discount factor의 합
- 첫 번째 grid point ($i=1$)에서는 $\sum = 0$이므로 $D_1 = 1/(1 + \text{cpn}_1)$

### 1.4 Short-End Handling (tenor < step)

Step보다 짧은 tenor의 quote(예: semi-annual 커브에서 3M)는 별도 처리:

- **Bond curve**: $D(\tau) = \exp(-r \cdot \tau)$ (continuous compounding, par yield ≈ zero rate)
- **SWAP curve**: $D(\tau) = \frac{1}{1 + r \cdot \text{dcf}(\tau)}$ (simple discount, money market convention)

### 1.5 Interpolation

Grid point 사이의 discount factor는 **log-linear interpolation**으로 구한다:

$$
\ln D(\tau) = \ln D(\tau_L) + w \cdot [\ln D(\tau_R) - \ln D(\tau_L)]
$$

$$
w = \frac{\tau - \tau_L}{\tau_R - \tau_L}
$$

### 1.6 Par Yield 역산

Bootstrapped zero curve로부터 par yield를 역산하여 검증한다:

$$
c(T) = \text{freq} \times \frac{1 - D(T)}{\sum_{i=1}^{n} D(\tau_i)}
$$

여기서 $\tau_i = i \times h$, $\tau_n = T$

---

## 2. Key Rate Duration (KRD) — Bloomberg Wave Method

### 2.1 왜 KRD가 필요한가

Modified Duration은 yield curve 전체가 동시에 같은 크기만큼 평행이동(parallel shift)한다고 가정한다.
하지만 현실에서 커브는 단기금리만 빠지거나, 장기금리만 오르는 등 비평행적으로 움직인다.

KRD는 이 한계를 극복한다. Yield curve 위에 여러 개의 **key tenor**(예: 1Y, 2Y, 3Y, 5Y, 10Y, ...)를 지정하고, **한 번에 하나의 key tenor만** 움직였을 때 채권 가격이 얼마나 변하는지를 각각 측정한다. 이를 통해 "이 채권은 5Y 금리에 가장 민감하다" 같은 분석이 가능해진다.

### 2.2 Triangle Wave: Bump의 형태

핵심 질문: key tenor 하나를 bump할 때, 나머지 tenor의 금리는 어떻게 되는가?

여러 방법이 가능하다:

1. **Step function**: 5Y 한 점만 -1bp, 4.99Y와 5.01Y는 0bp → 커브에 불연속(점프)이 생겨 bootstrapping 결과가 불안정
2. **Box function**: 4Y~6Y 전체를 -1bp → KRD 영역이 서로 겹쳐서 $\sum \text{KRD} \neq \text{Effective Duration}$
3. **Gaussian bump**: 5Y 중심으로 완만하게 퍼지되 4Y에서 0.3bp 정도 남음 → 역시 영역이 겹침

Bloomberg Wave 방법론은 **인접 key tenor에서 정확히 0이 되는 triangle wave**를 선택한다. 이것은 자명한 선택이 아니라, $\sum \text{KRD} = \text{Effective Duration}$을 성립시키기 위한 설계 결정이다:
- **5Y에서 bump의 최대치**, 양쪽 인접 key tenor(4Y, 6Y)에서 **정확히 0**
- 그 사이를 **직선으로 연결** → 자연스럽게 삼각형(triangle)이 된다

예를 들어 key tenor가 [4Y, 5Y, 6Y]이고 5Y를 **-1bp** bump하면:

```
bump
(bp)
 0.0 |------╲--------╱--------
     |       ╲      ╱
     |        ╲    ╱
     |         ╲  ╱
-1.0 |          ╲╱
     |     4Y   5Y    6Y
```

- **5Y 지점**: 정확히 -1bp (bump의 최대치)
- **4Y 지점**: 0bp (bump이 여기서 끝남)
- **6Y 지점**: 0bp (bump이 여기서 끝남)
- **4Y~5Y 사이**: 0bp에서 -1bp까지 직선으로 내려감
- **5Y~6Y 사이**: -1bp에서 0bp까지 직선으로 올라옴
- **4Y 이전, 6Y 이후**: 0bp (영향 없음)

이 설계의 핵심 성질:
- 각 key tenor의 bump 영역은 **서로 겹치지 않는다** (4Y bump의 우측 경계 = 5Y bump의 좌측 경계)
- 모든 key tenor를 동시에 같은 크기로 bump하면, 삼각형들이 빈틈없이 이어붙여져서 **parallel shift와 동일**하다
- 따라서 $\sum \text{KRD}(n) = \text{Effective Duration}$ 이 **정확히** 성립한다

### 2.3 Wave Shift Function — 수식 정의

Key tenor 배열을 $\{\tau_0, \tau_1, \ldots, \tau_N\}$ 이라 하자 (예: 0.25, 0.5, 1, 2, 3, ..., 30).
$n$번째 key tenor $\tau_n$에 bump size $\Delta$ (bp)를 적용할 때, 커브 위 임의의 tenor $\tau$에서의 shift:

**일반적인 경우 (중간 key tenor, $0 < n < N$):**

$$
w(\tau;\, n) = \begin{cases}
\Delta \times \dfrac{\tau - \tau_{n-1}}{\tau_n - \tau_{n-1}} & \text{if } \tau_{n-1} \leq \tau \leq \tau_n \quad \text{(좌측: 0에서 Δ까지 증가)} \\[8pt]
\Delta \times \dfrac{\tau_{n+1} - \tau}{\tau_{n+1} - \tau_n} & \text{if } \tau_n < \tau \leq \tau_{n+1} \quad \text{(우측: Δ에서 0까지 감소)} \\[8pt]
0 & \text{otherwise} \quad \text{(영향 범위 밖)}
\end{cases}
$$

$\tau = \tau_n$ (정확히 key tenor 위)에서: $w = \Delta$ (최대치)
$\tau = \tau_{n-1}$ 또는 $\tau = \tau_{n+1}$ (인접 key tenor)에서: $w = 0$

**첫 번째 key tenor ($n = 0$):**

좌측에 인접 key tenor가 없으므로, $\tau < \tau_0$ 구간에서는 원점(0)에서 $\tau_0$까지 ramp up:

$$
w(\tau;\, 0) = \begin{cases}
\Delta \times \dfrac{\tau}{\tau_0} & \text{if } 0 \leq \tau \leq \tau_0 \\[8pt]
\Delta \times \dfrac{\tau_1 - \tau}{\tau_1 - \tau_0} & \text{if } \tau_0 < \tau \leq \tau_1 \\[8pt]
0 & \text{otherwise}
\end{cases}
$$

**마지막 key tenor ($n = N$):**

우측에 인접 key tenor가 없으므로, $\tau > \tau_N$ 구간에서는 bump이 $\Delta$ 그대로 유지(flat):

$$
w(\tau;\, N) = \begin{cases}
\Delta \times \dfrac{\tau - \tau_{N-1}}{\tau_N - \tau_{N-1}} & \text{if } \tau_{N-1} \leq \tau \leq \tau_N \\[8pt]
\Delta & \text{if } \tau > \tau_N \quad \text{(flat 유지)}\\[8pt]
0 & \text{otherwise}
\end{cases}
$$

### 2.4 구체적 예시

Key tenors = [4Y, 5Y, 6Y], bump size $\Delta$ = -1bp, **5Y를 bump**하는 경우:

| Tenor | 구간 | 계산 | Shift |
|---|---|---|---|
| 3.0Y | 범위 밖 | — | 0 bp |
| 4.0Y | 좌측 경계 | $-1 \times \frac{4-4}{5-4} = 0$ | 0 bp |
| 4.5Y | 좌측 (4Y→5Y) | $-1 \times \frac{4.5-4}{5-4} = -0.5$ | -0.5 bp |
| 5.0Y | **bump 중심** | $-1 \times \frac{5-4}{5-4} = -1$ | **-1.0 bp** |
| 5.5Y | 우측 (5Y→6Y) | $-1 \times \frac{6-5.5}{6-5} = -0.5$ | -0.5 bp |
| 6.0Y | 우측 경계 | $-1 \times \frac{6-6}{6-5} = 0$ | 0 bp |
| 7.0Y | 범위 밖 | — | 0 bp |

### 2.5 Shifted Curve Construction

각 key tenor $n$에 대해 **up shift**와 **down shift** 두 개의 shifted curve를 만든다:

**Step 1.** Par yield curve의 모든 point에 wave shift를 적용:

$$
c'_i = c_i + w(\tau_i;\, n)  \quad \text{(up shift: +Δ)}
$$

$$
c''_i = c_i - w(\tau_i;\, n)  \quad \text{(down shift: -Δ)}
$$

여기서 $c_i$는 tenor $\tau_i$에서의 원래 par yield (%)

**Step 2.** Shifted par yield $c'_i$, $c''_i$ 각각에 대해 Section 1의 bootstrapping을 수행하여 shifted zero curve를 얻는다.

Key tenor가 $N$개이면, up/down 합쳐 총 $2N$개의 zero curve를 만든다. 이 과정은 서로 독립적이므로 병렬 처리가 가능하다.

### 2.6 Bond Repricing

채권의 미래 cashflow를 $\{(t_1, CF_1), (t_2, CF_2), \ldots, (t_M, CF_M)\}$ 이라 하자.
$t_k$는 valuation date로부터의 잔존기간 (년 단위), $CF_k$는 해당 시점의 현금흐름 (쿠폰 + 원금).

Zero curve의 discount factor $D(t)$를 사용하여 채권의 이론가를 구한다:

$$
P = \sum_{k=1}^{M} CF_k \times D(t_k)
$$

이 계산을 base curve와 각 shifted curve에 대해 수행:

- $P_{\text{base}}$ : 원래 zero curve로 계산한 이론가
- $P^{+}(n)$ : key tenor $n$을 $+\Delta$ bump한 zero curve로 계산한 이론가
- $P^{-}(n)$ : key tenor $n$을 $-\Delta$ bump한 zero curve로 계산한 이론가

금리가 올라가면 ($+\Delta$) 할인이 더 강해지므로 $P^{+}(n) < P_{\text{base}}$,
금리가 내려가면 ($-\Delta$) 할인이 약해지므로 $P^{-}(n) > P_{\text{base}}$.

### 2.7 KRD 계산

**Central difference (중앙차분)** 방식으로 금리 변화에 대한 가격 민감도를 구한다:

$$
\text{KRD}(n) = \frac{P^{-}(n) - P^{+}(n)}{2 \times \frac{\Delta}{100} \times P_{\text{dirty}}} \times 100
$$

각 항의 의미:

| 항 | 의미 |
|---|---|
| $P^{-}(n) - P^{+}(n)$ | 금리 하락 시 가격 - 금리 상승 시 가격 = 가격 변화폭 |
| $2 \times \frac{\Delta}{100}$ | 총 금리 변화폭 (down→up = $2\Delta$ bp, %로 변환하여 $\div 100$) |
| $P_{\text{dirty}}$ | 시장 dirty price로 나누어 정규화 |
| $\times 100$ | 가격이 face 100 기준이므로 % → 절대값 보정 |

$P_{\text{dirty}}$는 시장에서 관측된 dirty price (clean price + 경과이자)를 사용한다.
이론가 $P_{\text{base}}$가 아닌 시장가를 분모에 쓰는 이유는, Duration의 정의가 "현재 가치 대비 민감도"이기 때문이다.

**결과 해석:**
- KRD(n) = 2.5 → key tenor $n$의 금리가 1bp 상승하면 채권 가격이 약 0.025% 하락
- KRD의 단위는 **years** (Duration과 동일)
- KRD(n) > 0 → 해당 tenor 금리 상승 시 가격 하락 (일반적)
- KRD(n) < 0 → 해당 tenor 금리 상승 시 가격 상승 (이전 tenor의 쿠폰 할인 효과)

### 2.8 왜 Σ KRD = Effective Duration인가

모든 key tenor를 동시에 $+\Delta$만큼 bump하면, wave shift의 합이 전 구간에서 정확히 $\Delta$가 된다:

$$
\sum_{n=0}^{N} w(\tau;\, n) = \Delta \quad \text{for all } \tau
$$

이는 삼각파들이 빈틈없이 이어붙여지기 때문이다 (좌측 삼각형의 끝 = 우측 삼각형의 시작).
따라서 모든 key tenor의 KRD를 합하면 전체 커브를 parallel shift한 것과 같고:

$$
\sum_{n=0}^{N} \text{KRD}(n) = \frac{P_{\text{parallel}}^{-} - P_{\text{parallel}}^{+}}{2 \times \frac{\Delta}{100} \times P_{\text{dirty}}} \times 100 = \text{Effective Duration}
$$

이 성질은 구현의 정합성 검증에도 유용하다. 실제 계산에서 $\sum \text{KRD}$와 Effective Duration의 차이가 machine epsilon ($\sim 10^{-15}$) 수준임을 확인했다.

---

## 3. Implementation Details

### 3.1 Key Tenors

```
[0.25, 0.5, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 20, 25, 30]
```

추가 tenor (40, 50)는 커브에 존재할 경우 포함.

### 3.2 Face Value Convention

| 구분 | Price 기준 | Cashflow 기준 | 변환 |
|---|---|---|---|
| KRW | face 10,000 | face 10,000 | 없음 |
| 외화 | face 100 | face 1,000,000 | cashflow / 10,000 |

엔진은 face value를 모른다. Input 단계에서 price와 cashflow의 스케일을 맞추면 된다.

### 3.3 Parallelization

- Phase 1: $2N$개 shifted curve bootstrapping → goroutine 병렬
- Phase 2: bond repricing → `runtime.NumCPU()` worker pool

### 3.4 CLI Usage

```bash
go run ./cmd/bondkrd/ --input input.json > output.json
```

### 3.5 Input JSON Schema

```json
{
  "valuation_date": "2026-04-17",
  "bump_bp": 1,
  "coupon_frequency": 2,
  "day_count": "ACT/ACT",
  "curve": [
    {"tenor": 0.25, "par_yield": 3.69},
    {"tenor": 0.5,  "par_yield": 3.71},
    ...
  ],
  "bonds": [
    {
      "isin": "US912810TA60",
      "dirty_price": 67.82,
      "cashflows": [
        {"date": "2026-08-15", "amount": 0.875},
        {"date": "2027-02-15", "amount": 0.875},
        ...
      ]
    },
    {
      "isin": "US912810TA60",
      "dirty_price": 67.82,
      "cashflows": [
        {"date": "2026-08-15", "amount": 0.875},
        {"date": "2027-02-15", "amount": 0.875},
        ...
      ]
    }
  ]
}
```

### 3.6 Output JSON Schema

```json
{
  "valuation_date": "2026-04-17",
  "bump_bp": 1,
  "results": [
    {
      "isin": "US912810TA60",
      "dirty_price": 67.82,
      "base_price": 67.15,
      "effective_duration": 12.95,
      "key_rate_deltas": [
        {"tenor": 0.25, "krd": 0.003, "price_down": 67.16, "price_up": 67.14, ...},
        ...
      ]
    },
    {
      "isin": "US912810TA60",
      "dirty_price": 67.82,
      "base_price": 67.15,
      "effective_duration": 12.95,
      "key_rate_deltas": [
        {"tenor": 0.25, "krd": 0.003, "price_down": 67.16, "price_up": 67.14, ...},
        ...
      ]
    }
  ]
}
```
