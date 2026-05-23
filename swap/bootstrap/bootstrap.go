package bootstrap

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/instruments/swaps"
	krx "github.com/meenmo/molib/swap/clearinghouse/krx"
	"github.com/meenmo/molib/swap/curve"
	"github.com/meenmo/molib/utils"
)

// Request is the JSON-compatible input schema for par swap curve bootstrapping.
type Request struct {
	CurveDate      string             `json:"curve_date"`
	SettlementDate string             `json:"settlement_date,omitempty"`
	Currency       string             `json:"currency"`
	Index          string             `json:"index"`
	CurveQuotes    map[string]float64 `json:"curve_quotes"`
	QuoteType      string             `json:"quote_type,omitempty"`
	Compounding    string             `json:"compounding,omitempty"`
}

// Node is a single bootstrapped curve point.
type Node struct {
	Tenor            string  `json:"tenor"`
	Date             string  `json:"date"`
	DF               float64 `json:"df"`
	ZeroPct          float64 `json:"zero_pct"`
	ParPct           float64 `json:"par_pct"`
	RepricingErrorBP float64 `json:"repricing_error_bp"`
}

// Result is the JSON-compatible response schema.
type Result struct {
	CurveDate      string `json:"curve_date"`
	SettlementDate string `json:"settlement_date"`
	Currency       string `json:"currency"`
	Index          string `json:"index"`
	QuoteType      string `json:"quote_type,omitempty"`
	Nodes          []Node `json:"nodes"`
	Error          string `json:"error,omitempty"`
}

type tenorPair struct {
	key   string
	years float64
	rate  float64
}

// BootstrapParSwapCurve bootstraps a par swap curve and returns ordered curve nodes.
func BootstrapParSwapCurve(in Request) (*Result, error) {
	if in.CurveDate == "" {
		return nil, fmt.Errorf("curve_date is required")
	}
	if len(in.CurveQuotes) == 0 {
		return nil, fmt.Errorf("curve_quotes is required")
	}

	idx := strings.ToUpper(strings.TrimSpace(in.Index))
	cfg, ok := swaps.BootstrapConfigByIndex(idx)
	if !ok {
		return nil, fmt.Errorf("unknown index: %q (supported: ESTR, SOFR, TONAR, SONIA, HIBOR3M, EURIBOR3M, EURIBOR6M, CD91D)", in.Index)
	}

	curveDate, err := time.Parse("2006-01-02", in.CurveDate)
	if err != nil {
		return nil, fmt.Errorf("invalid curve_date: %v", err)
	}

	quoteType, err := normalizeQuoteType(in.QuoteType)
	if err != nil {
		return nil, err
	}
	compounding, err := normalizeCompounding(in.Compounding)
	if err != nil {
		return nil, err
	}

	settlement, err := settlementDate(in, curveDate, cfg)
	if err != nil {
		return nil, err
	}

	pairs, err := sortedTenors(in.CurveQuotes, cfg.Kind)
	if err != nil {
		return nil, err
	}

	resultQuoteType := ""
	if strings.TrimSpace(in.QuoteType) != "" || quoteType == "spot" {
		resultQuoteType = quoteType
	}
	out := &Result{
		CurveDate:      curveDate.Format("2006-01-02"),
		SettlementDate: settlement.Format("2006-01-02"),
		Currency:       in.Currency,
		Index:          idx,
		QuoteType:      resultQuoteType,
		Nodes:          make([]Node, 0, len(pairs)),
	}

	switch cfg.Kind {
	case swaps.BootstrapKRX:
		if quoteType == "spot" {
			return nil, fmt.Errorf("quote_type spot is not supported for KRX curves")
		}
		if err := appendKRXNodes(out, settlement, pairs, cfg); err != nil {
			return nil, err
		}
	case swaps.BootstrapOIS, swaps.BootstrapIBOR:
		if quoteType == "spot" {
			if err := appendSpotCurveNodes(out, settlement, pairs, cfg, compounding); err != nil {
				return nil, err
			}
			break
		}
		if err := appendParCurveNodes(out, settlement, in.CurveQuotes, pairs, cfg); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func normalizeQuoteType(value string) (string, error) {
	quoteType := strings.ToLower(strings.TrimSpace(value))
	if quoteType == "" {
		return "par", nil
	}
	switch quoteType {
	case "par", "spot":
		return quoteType, nil
	default:
		return "", fmt.Errorf("unknown quote_type: %q (supported: par, spot)", value)
	}
}

func normalizeCompounding(value string) (string, error) {
	compounding := strings.ToLower(strings.TrimSpace(value))
	if compounding == "" {
		return "continuous", nil
	}
	switch compounding {
	case "continuous", "simple":
		return compounding, nil
	default:
		return "", fmt.Errorf("unknown compounding: %q (supported: continuous, simple)", value)
	}
}

func settlementDate(in Request, curveDate time.Time, cfg swaps.BootstrapConfig) (time.Time, error) {
	if in.SettlementDate != "" {
		settlement, err := time.Parse("2006-01-02", in.SettlementDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid settlement_date: %v", err)
		}
		return settlement, nil
	}
	return calendar.AddBusinessDays(cfg.Calendar, curveDate, cfg.SpotLag), nil
}

func sortedTenors(quotes map[string]float64, kind swaps.BootstrapKind) ([]tenorPair, error) {
	pairs := make([]tenorPair, 0, len(quotes))
	for k, v := range quotes {
		yrs, err := parseTenorYears(k, kind)
		if err != nil {
			return nil, fmt.Errorf("parse tenor %q: %w", k, err)
		}
		pairs = append(pairs, tenorPair{key: k, years: yrs, rate: v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].years < pairs[j].years })
	return pairs, nil
}

func appendParCurveNodes(out *Result, settlement time.Time, quotes map[string]float64, pairs []tenorPair, cfg swaps.BootstrapConfig) error {
	var c *curve.Curve
	if cfg.Kind == swaps.BootstrapOIS {
		c = curve.BuildCurve(settlement, quotes, cfg.Calendar, 1)
	} else {
		c = curve.BuildIBORDiscountCurve(settlement, quotes, cfg.Calendar, 1)
	}
	if c == nil {
		return fmt.Errorf("curve.BuildCurve returned nil")
	}

	for _, p := range pairs {
		d := tenorDate(settlement, p.years, cfg.Calendar)
		implied := c.ImpliedParRatePct(d)
		out.Nodes = append(out.Nodes, Node{
			Tenor:            p.key,
			Date:             d.Format("2006-01-02"),
			DF:               c.DF(d),
			ZeroPct:          c.ZeroRateAt(d),
			ParPct:           p.rate,
			RepricingErrorBP: (implied - p.rate) * 100.0,
		})
	}
	return nil
}

func appendSpotCurveNodes(out *Result, settlement time.Time, pairs []tenorPair, cfg swaps.BootstrapConfig, compounding string) error {
	dfs := make(map[time.Time]float64, len(pairs)+1)
	dfs[settlement] = 1.0
	for _, p := range pairs {
		d := tenorDate(settlement, p.years, cfg.Calendar)
		t := utils.YearFraction(settlement, d, "ACT/365F")
		if t < 0 {
			return fmt.Errorf("negative time for tenor %s", p.key)
		}
		r := p.rate / 100.0
		switch compounding {
		case "continuous":
			dfs[d] = math.Exp(-r * t)
		case "simple":
			dfs[d] = 1.0 / (1.0 + r*t)
		}
	}

	c := curve.NewCurveFromDFs(settlement, dfs, cfg.Calendar, 1)
	if c == nil {
		return fmt.Errorf("curve.NewCurveFromDFs returned nil")
	}
	for _, p := range pairs {
		d := tenorDate(settlement, p.years, cfg.Calendar)
		out.Nodes = append(out.Nodes, Node{
			Tenor:            p.key,
			Date:             d.Format("2006-01-02"),
			DF:               c.DF(d),
			ZeroPct:          c.ZeroRateAt(d),
			ParPct:           c.ImpliedParRatePct(d),
			RepricingErrorBP: 0,
		})
	}
	return nil
}

func appendKRXNodes(out *Result, settlement time.Time, pairs []tenorPair, cfg swaps.BootstrapConfig) error {
	quotes := make(krx.ParSwapQuotes, len(pairs))
	for _, p := range pairs {
		quotes[p.years] = p.rate
	}
	c := krx.BootstrapCurve(settlement.Format("2006-01-02"), quotes)
	if c == nil {
		return fmt.Errorf("krx.BootstrapCurve returned nil")
	}
	for _, p := range pairs {
		d := tenorDate(settlement, p.years, cfg.Calendar)
		implied := c.ImpliedParRatePct(d)
		out.Nodes = append(out.Nodes, Node{
			Tenor:            p.key,
			Date:             d.Format("2006-01-02"),
			DF:               c.DF(d),
			ZeroPct:          c.ZeroRateAt(d),
			ParPct:           p.rate,
			RepricingErrorBP: (implied - p.rate) * 100.0,
		})
	}
	return nil
}

func tenorDate(settle time.Time, years float64, cal calendar.CalendarID) time.Time {
	if years >= 1.0/12 {
		months := int(years*12 + 0.5)
		d := calendar.AddMonth(settle, months)
		return calendar.Adjust(cal, d)
	}
	days := int(years*365 + 0.5)
	d := settle.AddDate(0, 0, days)
	return calendar.Adjust(cal, d)
}

func parseTenorYears(tenor string, kind swaps.BootstrapKind) (float64, error) {
	t := strings.ToUpper(strings.TrimSpace(tenor))
	if kind == swaps.BootstrapKRX {
		if t == "91D" {
			return 0.25, nil
		}
		if t == "1D" {
			return 0, nil
		}
	}
	return curve.ParseTenorYears(tenor)
}
