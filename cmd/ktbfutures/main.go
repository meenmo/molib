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

	"github.com/meenmo/molib/bond"
)

type ktbInput struct {
	Date    string        `json:"date"`
	CD91    float64       `json:"cd91"`
	Baskets []basketInput `json:"baskets"`
}

type basketInput struct {
	Tenor int         `json:"tenor"`
	Bonds []bondInput `json:"bonds"`
}

type bondInput struct {
	ISIN         string  `json:"isin"`
	IssueDate    string  `json:"issue_date"`
	MaturityDate string  `json:"maturity_date"`
	CouponRate   float64 `json:"coupon_rate"`
	MarketYield  float64 `json:"market_yield"`
}

type ktbOutput struct {
	Date          string        `json:"date"`
	FuturesExpiry string        `json:"futures_expiry"`
	Results       []tenorResult `json:"results"`
	Error         string        `json:"error,omitempty"`
}

type tenorResult struct {
	Tenor     int     `json:"tenor"`
	FairValue float64 `json:"fair_value"`
}

func main() {
	inputPath := flag.String("input", "", "JSON input path (reads stdin if omitted)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Fprintln(os.Stderr, "Usage: ktbfutures -input <path>")
		fmt.Fprintln(os.Stderr, "Compute KTB futures fair value.")
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
			outputs = append(outputs, ktbOutput{Date: in.Date, Error: err.Error()})
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
	today, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}

	// Convert JSON input to bond package types
	baskets := make([]bond.KTBBasket, len(in.Baskets))
	for i, b := range in.Baskets {
		bonds := make([]bond.KTBBond, len(b.Bonds))
		for j, bi := range b.Bonds {
			issue, err := time.Parse("2006-01-02", bi.IssueDate)
			if err != nil {
				return nil, fmt.Errorf("parse issue_date for %s: %w", bi.ISIN, err)
			}
			maturity, err := time.Parse("2006-01-02", bi.MaturityDate)
			if err != nil {
				return nil, fmt.Errorf("parse maturity_date for %s: %w", bi.ISIN, err)
			}
			bonds[j] = bond.KTBBond{
				ISIN:         bi.ISIN,
				IssueDate:    issue,
				MaturityDate: maturity,
				CouponRate:   bi.CouponRate,
				MarketYield:  bi.MarketYield,
			}
		}
		baskets[i] = bond.KTBBasket{Tenor: b.Tenor, Bonds: bonds}
	}

	results, err := bond.ComputeKTBFairValues(bond.KTBFairValueInput{
		Date:    today,
		CD91:    in.CD91,
		Baskets: baskets,
	})
	if err != nil {
		return nil, err
	}

	out := &ktbOutput{Date: in.Date}
	for _, r := range results {
		if out.FuturesExpiry == "" {
			out.FuturesExpiry = r.FuturesExpiry.Format("2006-01-02")
		}
		out.Results = append(out.Results, tenorResult{
			Tenor:     r.Tenor,
			FairValue: r.FairValue,
		})
	}
	return out, nil
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
