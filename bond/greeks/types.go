package greeks

// KRDInput holds all inputs for the KRD calculation.
type KRDInput struct {
	ValuationDate   string       `json:"valuation_date"`
	BumpBP          float64      `json:"bump_bp"`
	CouponFrequency int          `json:"coupon_frequency"` // 2=semi-annual, 4=quarterly
	Curve           []CurvePoint `json:"curve"`
	Bonds           []BondInput  `json:"bonds"`
}

// CurvePoint is a par-yield curve observation keyed by tenor in years.
type CurvePoint struct {
	Tenor    float64 `json:"tenor"`
	ParYield float64 `json:"par_yield"` // percent
}

// BondInput describes a bond for KRD calculation.
type BondInput struct {
	ISIN       string    `json:"isin"`
	DirtyPrice float64   `json:"dirty_price"`
	Cashflows  []CFInput `json:"cashflows"`
}

// CFInput is a single dated cashflow.
type CFInput struct {
	Date   string  `json:"date"`   // "2006-01-02"
	Amount float64 `json:"amount"`
}

// KRDOutput is the top-level output from ComputeKRD.
type KRDOutput struct {
	ValuationDate string       `json:"valuation_date"`
	BumpBP        float64      `json:"bump_bp"`
	Results       []BondResult `json:"results"`
}

// BondResult holds the complete KRD output for a single bond.
type BondResult struct {
	ISIN              string         `json:"isin"`
	DirtyPrice        float64        `json:"dirty_price"`
	BasePrice         float64        `json:"base_price"`
	EffectiveDuration float64        `json:"effective_duration"`
	KeyRateDeltas     []KeyRateDelta `json:"key_rate_deltas"`
}

// KeyRateDelta holds the KRD result for a single key tenor.
type KeyRateDelta struct {
	Tenor        float64 `json:"tenor"`
	PriceDown    float64 `json:"price_down"`
	PriceUp      float64 `json:"price_up"`
	Delta1Sided  float64 `json:"delta_1sided"`
	DeltaCentral float64 `json:"delta_central"`
	KRD          float64 `json:"krd"`
}

// zeroCurve is an internal bootstrapped discount curve.
type zeroCurve struct {
	points     []CurvePoint
	grid       []float64
	discounts  []float64
	shiftIndex int
	bumpPct    float64
	freq       int     // coupon frequency (2 or 4)
	step       float64 // 1.0/freq
}

// cashflow is an internal parsed cashflow with pre-computed tenor.
type cashflow struct {
	amount float64
	tenor  float64
}

// shiftedCurveResult is used for goroutine communication.
type shiftedCurveResult struct {
	keyIndex  int
	direction int
	curve     *zeroCurve
	err       error
}
