# 국채선물 명세 및 이론가·리스크 지표 산출 방법론

## 1. 상품 개요

### 1.1 기본 속성

| 항목 | 내용 |
|------|------|
| 상장 만기(tenor) | 3Y, 5Y, 10Y, 30Y |
| 결제월(Maturity month) | 3 / 6 / 9 / 12 월 |
| 결제일(Expiry date) | 결제월의 **세 번째 화요일** (한국 영업일 roll 적용) |
| 동시 상장 계약 | 근월물 + 원월물 총 2개 |
| 기초자산 | **2~3개의 고정 바스켓 채권** |
| 최종 결제 | **현금결제**(cash settlement) |
| 호가 단위 | 가격 0.01 |

### 1.2 기초자산 바스켓

각 선물 계약은 **상장 시점에 KRX가 지정한 2~3개 국고채권**으로 구성된 고정 바스켓을 가진다. 이론가는 바스켓 채권 각각의 선도수익률을 **산술평균**하여 산출한다.

| 만기(tenor) | 바스켓 채권 수 |
|-------------|----------------|
| 3Y | 3 종목 |
| 5Y | 2 종목 |
| 10Y | 2 종목 |
| 30Y | 2 종목 |


---

## 2. 최종결제가격(Final Settlement Price) 산정

결제일(3 째주 화요일)의 **10:00, 10:30, 11:00, 11:30** 네 시점에 채권 시장의 유통수익률을 스냅샷하여 산출한다.

### 2.1 산정 절차

1. **각 시점에서** 바스켓 채권 별 최종 유통수익률의 **산술평균** (소수점 넷째 자리 반올림)
2. 10:00 / 10:30 / 11:00 세 개의 평균값 중 **중간값(median)** 추출
3. 위 중간값과 11:30 의 평균값을 **산술평균** (소수점 넷째 자리 반올림)
4. 그 값을 아래 최종 결제 산식에 대입 (소수점 셋째 자리 반올림)

$$\sum\limits_{i=1}^{2\times 만기} \frac{5/2}{(1+y/2)^i} + \frac{100}{(1+y/2)^{2\times 만기}}$$

### 2.2 예시

| 구분 | 10:00 | 10:30 | 11:00 | 11:30 |
|------|:-----:|:-----:|:-----:|:-----:|
| 기초채권 A | 2.157% | 2.167% | 2.150% | 2.156% |
| 기초채권 B | 2.003% | 2.009% | 1.999% | 2.004% |
| 기초채권 C | 2.034% | 2.041% | 2.019% | 2.037% |
| **시점별 평균** | **2.065%** | **2.072%** | **2.056%** | **2.066%** |

- 10:00 / 10:30 / 11:00 평균의 중간값 = **2.065%**
- 최종 수익률(위의 평균과 11:30의 평균) = (2.065% + 2.066%) / 2 = **2.066%**
- 최종 결제 가격: 위 최종 결제 산식에 대입하여 소수점 셋째 자리로 반올림

---

## 3. 이론가(Fair Value) 산출

### 3.1 개요

이론가는 **현물-선물 패리티(cost-of-carry model)** 를 바스켓 채권 각각에 적용해 선도 수익률을 구하고, 이를 산술평균하여 **5% 쿠폰 표준채권**의 가격으로 환산한다.

```
입력:
    T       = 평가일 (valuation date)
    T_e     = 선물 만기일 (expiry date)
    M       = 선물 만기(3/5/10/30 년)
    {B_j}   = 바스켓 채권 집합 (각 B_j = 발행일, 만기일, 쿠폰, YTM)
    r       = CD 91일물 금리 (단기금리, 스칼라)

출력:
    P_theo  = 이론가
```

### 3.2 6-step 산출 절차

각 바스켓 채권 `B_j` 에 대해 다음을 수행한다.

#### Step 1. 현물 더티가격(Spot Dirty Price)

채권의 유통수익률 `y_j` 로 가격을 계산한다.

$$
P_{\text{spot}, j} \;=\; \frac{1}{1 + \frac{y_j}{2} \cdot \frac{d}{t}}
\left[
  \sum_{i=0}^{n-1} \frac{c_j/2}{(1 + y_j/2)^i}
  \;+\; \frac{100}{(1 + y_j/2)^{n-1}}
\right]
$$

- $c_j$ : 쿠폰 이율 (%, 연이율)
- $y_j$ : 유통수익률 (민평3사 YTM)
- $d$ : 평가일에서 다음 쿠폰일까지의 일수
- $t$ : 직전 쿠폰일에서 다음 쿠폰일까지의 일수
- $n$ : 평가일 기준 잔여 쿠폰 횟수

#### Step 2. 기간 중 쿠폰의 현재가치(Interim Coupon PV)

구간 `(T, T_e]` 에 지급이 예정된 쿠폰을 단기금리 `r` 로 할인한다.

$$
PV_{\text{coup}, j} \;=\; \frac{c_j/2 \cdot 100}{1 + r \cdot \frac{d_1}{365}}
$$

- `d_1` : 평가일에서 해당 쿠폰일까지의 일수
- **KTB 는 반기 쿠폰이고 `(T, T_e]` 의 길이는 최대 ~6개월이므로 구간 내 쿠폰은 최대 1개**. 다수 쿠폰이 들어가는 병리적 상황은 실제 거래 환경에서 발생하지 않는다.

#### Step 3. 정리가격(Clean Price)

> ⚠️ 본 문서에서의 "Clean Price"는 전통적 의미(더티 − 경과이자)가 아니라 **스폿 더티에서 구간 내 쿠폰 PV를 뺀 값**이다. 혼동 주의.

$$
P_{\text{clean}, j} \;=\; P_{\text{spot}, j} \;-\; PV_{\text{coup}, j}
$$

#### Step 4. 선도 더티가격(Forward Dirty Price)

Step 3 의 가격을 만기까지 `r` 로 재투자(복리는 하지 않고 단리).

$$
P_{\text{fwd}, j} \;=\; P_{\text{clean}, j} \cdot \left(1 + r \cdot \frac{d_e}{365}\right)
$$

- `d_e` : 평가일에서 선물 만기까지의 일수

#### Step 5. 선도 수익률(Implied Forward Yield)

만기 시점 기준으로 재계산한 쿠폰 구조(`d', t', n'`) 에서 선도 더티가격이 성립하는 수익률 `f_j` 를 Newton-Raphson 으로 해낸다.

$$
P_{\text{fwd}, j} \;=\; \frac{1}{1 + \frac{f_j}{2} \cdot \frac{d'}{t'}}
\left[
  \sum_{i=0}^{n'-1} \frac{c_j/2}{(1 + f_j/2)^i}
  \;+\; \frac{100}{(1 + f_j/2)^{n'-1}}
\right]
$$

- `d'`, `t'`, `n'` : Step 1 과 같은 정의이나 기준일을 선물 만기일로 이동

#### Step 6. 선도수익률 산술평균 → 표준채권 가격

바스켓 전체의 선도수익률을 단순 평균한다.

$$
\bar{f} \;=\; \frac{1}{N}\sum_{j=1}^{N} f_j
$$

이 `\bar{f}` 를 **5% 표면금리·반기 쿠폰·만기 M 년의 표준 채권** 산식에 대입하여 이론가를 얻는다.

$$
\boxed{\;
P_{\text{theo}} \;=\; \sum_{i=1}^{2M} \frac{2.5}{(1 + \bar{f}/2)^i}
\;+\; \frac{100}{(1 + \bar{f}/2)^{2M}}
\;}
$$

### 3.3 molib 구현 매핑

| 절차 | 구현 위치 |
|------|-----------|
| Step 1 (현물 더티가격) | `bond.KTBMarketPrice` |
| Step 2~3 (구간 쿠폰 PV, Clean) | `bond.ktbForwardYield` 내 `couponBeforeExpiry / couponPV` 블록 |
| Step 4 (선도 더티가격) | `bond.ktbForwardYield` 내 `fwdDirty` 할당 |
| Step 5 (선도수익률) | `bond.KTBSolveImpliedYield` (+ 이분법 fallback) |
| Step 6 (표준채권 가격) | `bond.ktbFairValue` |
| 전체 orchestration | `bond.ComputeKTBFairValues` |

### 3.4 만기일 계산 규칙

- **근월물(`is_near_month=true`)**: `calendar.KTBFuturesExpiry(today)` — 현재 분기 또는 다음 분기의 3 째주 화요일 (한국 영업일 roll).
- **원월물(`is_near_month=false`)**: `calendar.KTBFuturesExpiryFarMonth(today)` — 근월물 만기 + 3 개월 후의 3 째주 화요일.

---

## 4. 리스크 지표

`molib` 는 이론가 위에 네 개의 유한차분(finite-difference) 리스크 지표를 얹는다.

```
입력: 이론가 산출에 필요한 모든 입력 + 아래 추가 필드
   market_price     → basis 계산용
   ktb_curve        → KRD 계산용 (20-node par yield curve)
   on_the_run_ktb   → on-off 계산용 (지표채권 집합)
   next_business_date → theta 계산용
```

### 4.1 Theta (시간감마)

선물 가치가 평가일 이동에 따라 변하는 속도.

$$
\theta \;=\; \frac{P_{\text{theo}}(T = T_{\text{next}}) \;-\; P_{\text{theo}}(T)}{100}
$$

- `T_{\text{next}}` 는 입력 `next_business_date`. 호출부가 선택 (일반적으로 `T+1` 달력일).
- 기초 바스켓, 쿠폰, YTM, CD91 은 **모두 고정**. 오직 평가일만 이동.
- **VBA/Excel 레퍼런스 정합**: `T_{\text{next}} = T + 1\ \text{calendar day}` (주말/공휴일 무시).
- **트레이더 실감형 1영업일 theta**: `T_{\text{next}}` = 다음 KOFR 영업일 (`marketdata.benchmark_rates.index_name='KOFR'` 의 다음 날짜).

### 4.2 Key Rate Duration (KRD, 20-bucket delta)

20-node 국고 par yield curve 의 각 key tenor 에 **Bloomberg Wave 삼각 이동**을 1bp 가함으로써 각 tenor 별 민감도를 산출한다.

#### 4.2.1 키 테너 집합

```
τ ∈ {0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 20, 25, 30, 40}
```

`pricing.curve_data WHERE curve_id='ktb' AND is_keytenor=true` 로 조회.
0.25Y 의 par yield 는 관례상 zero rate 와 동일(반기 쿠폰 주기 미만이므로 정의상 par = zero).

#### 4.2.2 삼각 이동(Triangular Shift)

버킷 `τ_i` 에서의 1bp bump 는 `τ_{i-1}` 과 `τ_{i+1}` 에서 0 이 되는 삼각함수 형태로 커브를 이동시킨다. 경계(0.25, 40)는 반쪽 삼각형으로 처리.

바스켓 채권 `B_j` (잔존만기 `T_j`) 에 대한 수익률 이동량은 삼각형을 `T_j` 에서 interpolate한 값:

$$
\Delta y_j(\tau_i) \;=\; \text{Triangle}(\tau_i, \;T_j)\cdot 1\text{bp}
$$

`cd91` 은 **0.25Y 버킷 bump 에만** `\Delta r = \text{Triangle}(0.25, 0.25) \cdot 1\text{bp}` 만큼 이동. 다른 버킷에서는 `cd91` 불변.

#### 4.2.3 중앙차분

각 버킷 `τ_i` 에 대해 상향·하향 1bp 을 각각 적용한 후 6-step 이론가 산출을 다시 실행:

$$
\text{KRD}_i \;=\; \frac{P_{\text{theo}}^{\downarrow}(\tau_i) \;-\; P_{\text{theo}}^{\uparrow}(\tau_i)}{2 \cdot 1\text{bp} \cdot 100}
$$

총합은 유효 듀레이션(effective duration) 과 일치:

$$
\sum_i \text{KRD}_i \;\approx\; \text{Effective Duration}
$$

### 4.3 Basis (시장가-이론가)

$$
\text{basis} \;=\; P_{\text{market}} \;-\; P_{\text{theo}}
$$

- `P_{\text{market}}` 은 `marketdata.commodities WHERE field='F15001' AND ticker=<futures_code>` 에서 조회한 장 종료가.
- 입력에 `market_price` 가 없으면 출력 `basis = null`.

### 4.4 On-off Basis (온오프 베이시스)

바스켓 채권이 **off-the-run** 으로 흘러가면서 지표채권(on-the-run) 곡선 대비 가지는 유동성 프리미엄이 선물 이론가에 미치는 영향을 가격 단위로 표현.

#### 4.4.1 정의

$$
\text{onoff} \;=\; P_{\text{theo}}^{\text{std}} \;-\; P_{\text{theo}}^{\text{ind}}
$$

- `P_{\text{theo}}^{\text{std}}`: 바스켓 채권의 **실제 민평3사 YTM** 을 그대로 사용하여 산출한 이론가 (= `fair_value`).
- `P_{\text{theo}}^{\text{ind}}`: 동일한 바스켓에서 **각 채권의 YTM 만을 par yield 곡선에서 그 채권의 잔존만기 위치에 linear interpolation 한 값** 으로 교체한 후 산출한 이론가.

#### 4.4.2 지표 수익률 선정 방식

`molib` 는 Postgres 의 20-node par yield curve (`pricing.curve_data WHERE is_keytenor=true`) 를 그대로 **선형보간** 한다. 이 curve 는 이미 각 key tenor 의 최근 발행 on-the-run 국고채권을 bootstrap 한 결과이므로, 별도의 지표채권 ISIN 집합을 받을 필요가 없다(입력 `on_the_run_ktb` 는 호환 목적으로 유지하되, 제공 시 그 yield 를 우선 사용).

$$
y_j^{\text{ind}} \;=\; \text{LinearInterp}\bigl(
  \text{ktb\_par\_curve}, \; T_j
\bigr)
$$

#### 4.4.3 일반적 부호

Off-the-run 채권은 유동성 프리미엄으로 인해 on-the-run 대비 YTM 이 높다 → `y_{std} > y_{ind}` → `P_{std} < P_{ind}` → **`onoff < 0` 가 정상 상태**.

#### 4.4.4 Par-yield vs YTM interpolation

Excel 워크북은 1.5/2/3/4/5 (3Y 선물) 또는 7/10 (10Y 선물) 의 **5 개 또는 2 개 지표채권의 실제 YTM 을 선형보간** 한다. `molib` 는 20-node par yield curve 를 사용한다. 두 방식의 이론적·실무적 차이:

| 기준 | YTM 보간 (Excel) | Par-yield 보간 (molib) |
|------|------------------|------------------------|
| 차익거래 정합성 | 없음 (채권별 쿠폰 이질성 혼입) | **있음** (zero curve 에서 유도) |
| 데이터 활용도 | 2~5 개 손수 지정된 지표채권 | **전체 유동 국고채 bootstrap 결과** |
| 복제·감사 용이성 | 지표채권 선정 규칙에 의존 | **결정론적** (curve_id, is_keytenor 만으로 재현) |
| 트레이더 직관 부합 | **높음** (화면 호가 직결) | bp 수준에서 근사 일치 |
| 시스템 프로덕션 적합성 | 낮음 (수기 큐레이션 필요) | **높음** |

`molib` 는 프로덕션 리스크·손익 시스템을 위해 **par-yield curve 방식을 기본 채택**한다. Excel 레퍼런스 대비 on-off 절대값은 계약별 2~5¢ 수준의 methodology drift 가 발생할 수 있으며, 이는 버그가 아니라 설계 선택이다.

---

## 5. 호출 규약 및 데이터 소스

### 5.1 CLI 호출 단위

- **1 호출 = 1 계약**. 동일 평가일의 8 계약(3Y/5Y/10Y/30Y × 근월/원월) 일괄 처리는 호출부(Python / shell / 상위 서비스)에서 병렬 조율.
- 입력/출력 JSON 스키마는 `cmd/ktbfutures/readme.md` 참조.

### 5.2 Postgres 데이터 소스 매핑

| 입력 필드 | Postgres 쿼리 |
|-----------|---------------|
| `bonds[]` (바스켓) | `security_master.ktb_futures_basket ⋈ security_master.bond ⋈ marketdata.bond[민평3사]` |
| `cd91` | `marketdata.benchmark_rates WHERE index_name='CD' AND tenor='3M'` |
| `market_price` | `marketdata.commodities WHERE field='F15001' AND ticker=<futures_code>` |
| `ktb_curve` | `pricing.curve_data WHERE curve_id='ktb' AND is_keytenor=true` |
| `on_the_run_ktb` (옵션) | `security_master.on_the_run_bond WHERE curve_group='ktb' AND is_inflation=false` |
| `next_business_date` | 레퍼런스 정합: `date + 1 calendar day` / 영업일 theta: `marketdata.benchmark_rates[KOFR]` 의 다음 날짜 |

### 5.3 관례 고정사항

- **단기금리는 CD91 만 사용**. Spec 상 "CD91 · RP 등" 허용되나 실제 펀딩이 CD91 기준이므로 다른 금리로 확장하지 않음.
- 바스켓 채권의 YTM 은 **민평3사** (`source='민평3사'`, `data_type='YIELD'`, field `F09125`) 고정.
- 물가연동국채는 on-the-run 조회에서 제외(`is_inflation=false`).

---

## 6. 검증

### 6.1 VBA/Excel 대조 회귀

`ktb_futures_ref.xlsx` 기준 294 계약-일 (2026-01-02 ~ 2026-04-21, 74 거래일 × 3Y/10Y 근월·원월) 대조 결과:

| 지표 | median \|Δ\| | max \|Δ\| | 비고 |
|------|--------------|-----------|------|
| `fair_value` | ~1e-4 | ~1e-3 | 본질적으로 일치 |
| `theta` (T+1 calendar day) | 4.6e-9 | 1.4e-5 | VBA-리터럴 관례에서 일치 |
| `basis` | 3.3e-5 | 2.5e-1 | median 일치. 특정 일자의 outlier 는 upstream 데이터 drift |
| `onoff` | 3.4e-2 | 2.6e-1 | par-curve vs YTM-interp methodology drift (설계 선택) |
| KRD 0.75 / 12 / 20 | 0 | ~1e-15 | 기계적 일치 |
| KRD 0.25 ~ 10 | ~e-9 ~ e-6 | ~e-4 | contract roll 주변에서 outlier |

### 6.2 On-off 매칭 실험

지표채권 ISIN 집합을 Excel `지표물_종목` 시트와 **정확히 동일**하게 사용하면(3Y 선물에 1.5/2/3/4/5 의 5 종목, 10Y 선물에 7/10 의 2 종목), on-off 절대값이 계약별 0.2~0.5¢ 이내로 좁혀진다. 잔차의 주요 원인:

1. **Par-curve vs YTM-interp**: 동일 ISIN 집합이라도 보간 대상이 par yield 인지 실제 YTM 인지에 따른 오차.
2. **기타 상수 반올림**: 민평3사 YTM 의 소수점 표시 정밀도 차이 (±0.005% 수준).

본 차이는 설계상 수용 가능 범위이며, molib 는 par-curve 방식을 프로덕션 방식으로 채택한다.

---

## 7. 참조

- `cmd/ktbfutures/readme.md` — CLI 입출력 스키마 및 SQL 쿼리
- `.omc/specs/ktbfutures-greeks.md` — 그릭스 확장 설계 명세
- `bond/ktb.go` — 이론가 산출 6-step 구현
- `bond/greeks/ktb.go` — 네 가지 리스크 지표 구현
- `cmd/bondkrd/readme.md` — Bloomberg-Wave KRD 방법론 (모듈화 재사용)
