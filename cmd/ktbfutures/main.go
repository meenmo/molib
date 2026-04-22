package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/meenmo/molib/bond/greeks"
	"github.com/meenmo/molib/bond/ktb"
)

type ktbInput struct {
	Today            string          `json:"today"`
	NextBusinessDate string          `json:"next_business_date"`
	ShortTermRate    float64         `json:"short_term_rate"`
	FuturesCode      string          `json:"futures_code"`
	IsNearMonth      bool            `json:"is_near_month"`
	Tenor            int             `json:"tenor"`
	MarketPrice      *float64        `json:"market_price,omitempty"`
	Basket           []bondInput     `json:"basket"`
	KTBCurve         []curveInput    `json:"ktb_curve"`
	OnTheRunKTB      []onTheRunInput `json:"on_the_run_ktb,omitempty"`
}

type bondInput struct {
	ISIN         string  `json:"isin"`
	IssueDate    string  `json:"issue_date"`
	MaturityDate string  `json:"maturity_date"`
	CouponRate   float64 `json:"coupon_rate"`
	Yield        float64 `json:"yield"`
}

type curveInput struct {
	Tenor float64 `json:"tenor"`
	Yield float64 `json:"yield"`
}

type onTheRunInput struct {
	ISIN         string  `json:"isin"`
	MaturityDate string  `json:"maturity_date"`
	Yield        float64 `json:"yield"`
}

type krdOut struct {
	Tenor float64 `json:"tenor"`
	Delta float64 `json:"delta"`
}

type ktbOutput struct {
	Date          string   `json:"date"`
	FuturesCode   string   `json:"futures_code"`
	IsNearMonth   bool     `json:"is_near_month"`
	Tenor         int      `json:"tenor"`
	FuturesExpiry string   `json:"futures_expiry"`
	FairValue     float64  `json:"fair_value"`
	MarketPrice   *float64 `json:"market_price"`
	Theta         float64  `json:"theta"`
	Basis         *float64 `json:"basis"`
	OnOffSpread   *float64 `json:"onoff_spread"`
	KRD           []krdOut `json:"krd"`
	Error         string   `json:"error,omitempty"`
}

func main() {
	inputPath := flag.String("input", "", "JSON input path (reads stdin if omitted)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Fprintln(os.Stderr, "Usage: ktbfutures -input <path>")
		fmt.Fprintln(os.Stderr, "Compute KTB futures fair value + greeks (theta, krd, basis, onoff_spread).")
		return
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" {
		if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			fmt.Fprintln(os.Stderr, "Usage: ktbfutures -input <path>")
			os.Exit(2)
		}
	}

	raw, err := readInput(path)
	if err != nil {
		exitError(fmt.Sprintf("read input: %v", err))
	}

	inputs, isArray, err := parseInputs(raw)
	if err != nil {
		exitError(fmt.Sprintf("parse JSON: %v", err))
	}

	hadError := false
	outputs := make([]ktbOutput, 0, len(inputs))
	for _, in := range inputs {
		out, err := process(in)
		if err != nil {
			hadError = true
			outputs = append(outputs, ktbOutput{Date: in.Today, FuturesCode: in.FuturesCode, Error: err.Error()})
			continue
		}
		outputs = append(outputs, *out)
	}

	if isArray {
		b, _ := json.Marshal(outputs)
		fmt.Println(string(b))
	} else {
		b, _ := json.Marshal(outputs[0])
		fmt.Println(string(b))
	}

	if hadError {
		os.Exit(1)
	}
}

func process(in ktbInput) (*ktbOutput, error) {
	today, err := time.Parse("2006-01-02", in.Today)
	if err != nil {
		return nil, fmt.Errorf("parse today: %w", err)
	}
	nextBiz, err := time.Parse("2006-01-02", in.NextBusinessDate)
	if err != nil {
		return nil, fmt.Errorf("parse next_business_date: %w", err)
	}

	bonds := make([]ktb.KTBBond, len(in.Basket))
	for j, bi := range in.Basket {
		issue, err := time.Parse("2006-01-02", bi.IssueDate)
		if err != nil {
			return nil, fmt.Errorf("parse issue_date for %s: %w", bi.ISIN, err)
		}
		maturity, err := time.Parse("2006-01-02", bi.MaturityDate)
		if err != nil {
			return nil, fmt.Errorf("parse maturity_date for %s: %w", bi.ISIN, err)
		}
		bonds[j] = ktb.KTBBond{
			ISIN:         bi.ISIN,
			IssueDate:    issue,
			MaturityDate: maturity,
			CouponRate:   bi.CouponRate,
			MarketYield:  bi.Yield,
		}
	}

	curvePts := make([]greeks.CurvePoint, len(in.KTBCurve))
	for i, p := range in.KTBCurve {
		curvePts[i] = greeks.CurvePoint{Tenor: p.Tenor, ParYield: p.Yield}
	}

	var onRun []greeks.OnTheRunBond
	if len(in.OnTheRunKTB) > 0 {
		onRun = make([]greeks.OnTheRunBond, len(in.OnTheRunKTB))
		for i, o := range in.OnTheRunKTB {
			mat, err := time.Parse("2006-01-02", o.MaturityDate)
			if err != nil {
				return nil, fmt.Errorf("parse on_the_run_ktb maturity_date for %s: %w", o.ISIN, err)
			}
			onRun[i] = greeks.OnTheRunBond{ISIN: o.ISIN, MaturityDate: mat, Yield: o.Yield}
		}
	}

	result, err := greeks.ComputeKTBGreeks(greeks.KTBGreeksInput{
		Date:             today,
		NextBusinessDate: nextBiz,
		CD91:             in.ShortTermRate,
		FuturesCode:      in.FuturesCode,
		IsNearMonth:      in.IsNearMonth,
		Tenor:            in.Tenor,
		MarketPrice:      in.MarketPrice,
		Bonds:            bonds,
		KTBCurve:         curvePts,
		OnTheRunKTB:      onRun,
	})
	if err != nil {
		return nil, err
	}

	krd := make([]krdOut, len(result.KRD))
	for i, k := range result.KRD {
		krd[i] = krdOut{Tenor: k.Tenor, Delta: k.Delta}
	}

	return &ktbOutput{
		Date:          in.Today,
		FuturesCode:   result.FuturesCode,
		IsNearMonth:   result.IsNearMonth,
		Tenor:         result.Tenor,
		FuturesExpiry: result.FuturesExpiry.Format("2006-01-02"),
		FairValue:     result.FairValue,
		MarketPrice:   result.MarketPrice,
		Theta:         result.Theta,
		Basis:         result.Basis,
		OnOffSpread:   result.OnOffSpread,
		KRD:           krd,
	}, nil
}

func readInput(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func parseInputs(raw []byte) ([]ktbInput, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, fmt.Errorf("empty input")
	}
	if trimmed[0] == '[' {
		var inputs []ktbInput
		if err := json.Unmarshal(trimmed, &inputs); err != nil {
			return nil, true, err
		}
		if len(inputs) == 0 {
			return nil, true, fmt.Errorf("empty input array")
		}
		return inputs, true, nil
	}
	var input ktbInput
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return nil, false, err
	}
	return []ktbInput{input}, false, nil
}

func exitError(msg string) {
	b, _ := json.Marshal(ktbOutput{Error: msg})
	fmt.Println(string(b))
	os.Exit(1)
}
