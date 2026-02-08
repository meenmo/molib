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

type yieldInput struct {
	TaskID           string         `json:"task_id,omitempty"`
	SettlementDate   string         `json:"settlement_date"`
	FuturesPrice     float64        `json:"futures_price"`
	ConversionFactor float64        `json:"conversion_factor"`
	CouponRate       float64        `json:"coupon_rate"`
	DayCount         string         `json:"day_count"`
	CouponFrequency  int            `json:"coupon_frequency"`
	Cashflows        []cashflowJSON `json:"cashflows"`
}

type cashflowJSON struct {
	Date      string `json:"date"`
	Coupon    int64  `json:"coupon"`
	Principal int64  `json:"principal"`
}

type yieldOutput struct {
	TaskID          string  `json:"task_id,omitempty"`
	SettlementDate  string  `json:"settlement_date"`
	FuturesPrice    float64 `json:"futures_price"`
	InvoicePrice    float64 `json:"invoice_price"`
	AccruedInterest float64 `json:"accrued_interest"`
	ForwardYield    float64 `json:"forward_yield"`
	Iterations      int     `json:"iterations"`
	Error           string  `json:"error,omitempty"`
}

func main() {
	inputPath := flag.String("input", "", "JSON input path (reads stdin if omitted)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Fprintln(os.Stderr, "Usage: fwdyield -input <path>")
		fmt.Fprintln(os.Stderr, "Compute CTD forward yield from invoice price via Newton-Raphson.")
		return
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" {
		if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			fmt.Fprintln(os.Stderr, "Usage: fwdyield -input <path>")
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
	outputs := make([]yieldOutput, 0, len(inputs))
	for _, in := range inputs {
		out, err := process(in)
		if err != nil {
			hadError = true
			outputs = append(outputs, yieldOutput{TaskID: in.TaskID, Error: err.Error()})
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

func process(in yieldInput) (*yieldOutput, error) {
	settlement, err := time.Parse("2006-01-02", in.SettlementDate)
	if err != nil {
		return nil, fmt.Errorf("invalid settlement_date: %v", err)
	}
	if in.DayCount != "ACT/ACT" {
		return nil, fmt.Errorf("unsupported day_count %q (only ACT/ACT)", in.DayCount)
	}

	cfs := make([]bond.Cashflow, 0, len(in.Cashflows))
	for _, cf := range in.Cashflows {
		d, err := time.Parse("2006-01-02", cf.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid cashflow date %s: %v", cf.Date, err)
		}
		cfs = append(cfs, bond.Cashflow{
			Date:      d,
			Coupon:    float64(cf.Coupon) / 10000.0,
			Principal: float64(cf.Principal) / 10000.0,
		})
	}

	res, err := bond.ComputeForwardYield(bond.ForwardYieldInput{
		SettlementDate:   settlement,
		FuturesPrice:     in.FuturesPrice,
		ConversionFactor: in.ConversionFactor,
		CouponRate:       in.CouponRate,
		CouponFrequency:  in.CouponFrequency,
		Cashflows:        cfs,
	})
	if err != nil {
		return nil, err
	}

	return &yieldOutput{
		TaskID:          in.TaskID,
		SettlementDate:  in.SettlementDate,
		FuturesPrice:    in.FuturesPrice,
		InvoicePrice:    res.InvoicePrice,
		AccruedInterest: res.AccruedInterest,
		ForwardYield:    res.ForwardYield,
		Iterations:      res.Iterations,
	}, nil
}

func readInput(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func parseInputs(raw []byte) ([]yieldInput, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, fmt.Errorf("empty input")
	}
	if trimmed[0] == '[' {
		var inputs []yieldInput
		if err := json.Unmarshal(trimmed, &inputs); err != nil {
			return nil, true, err
		}
		if len(inputs) == 0 {
			return nil, true, fmt.Errorf("empty input array")
		}
		return inputs, true, nil
	}
	var input yieldInput
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return nil, false, err
	}
	return []yieldInput{input}, false, nil
}

func exitError(msg string) {
	b, _ := json.Marshal(yieldOutput{Error: msg})
	fmt.Println(string(b))
	os.Exit(1)
}
