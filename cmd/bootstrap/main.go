// Command bootstrap reads a JSON curve specification (currency, index,
// curve_quotes), invokes the par swap bootstrap engine, and emits the
// bootstrapped discount factors, zero rates, and quote repricing diagnostics.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/meenmo/molib/swap/bootstrap"
)

func main() {
	inputPath := flag.String("input", "", "JSON input path (optional; if set, ignores stdin)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		usage()
		return
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" {
		if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			usage()
			os.Exit(2)
		}
	}

	raw, err := readInput(path)
	if err != nil {
		writeError(fmt.Sprintf("failed to read input: %v", err))
		return
	}

	var in bootstrap.Request
	if err := json.Unmarshal(raw, &in); err != nil {
		writeError(fmt.Sprintf("failed to parse JSON input: %v", err))
		return
	}

	out, err := bootstrap.BootstrapParSwapCurve(in)
	if err != nil {
		writeError(err.Error())
		return
	}

	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  bootstrap < input.json")
	fmt.Println("  bootstrap -input /path/to/input.json")
	fmt.Println()
	fmt.Println("Read JSON curve specification, run the appropriate engine bootstrap,")
	fmt.Println("emit discount factors, zero rates, par rates, and repricing diagnostics at each input tenor.")
	fmt.Println()
	fmt.Println("Input fields:")
	fmt.Println(`  curve_date       Curve as-of date (YYYY-MM-DD)`)
	fmt.Println(`  settlement_date  Optional. Defaults to curve_date + spot-lag BD.`)
	fmt.Println(`  currency         Informational tag (USD/EUR/JPY/GBP/HKD/KRW)`)
	fmt.Println(`  index            Bootstrap path selector. One of:`)
	fmt.Println(`                     OIS-style:  ESTR, SOFR, TONAR, SONIA`)
	fmt.Println(`                     IBOR-style: HIBOR3M, EURIBOR3M, EURIBOR6M`)
	fmt.Println(`                     KRX:        CD91D`)
	fmt.Println(`  curve_quotes     tenor -> par-rate (%) by default, or spot-rate (%) when quote_type=spot`)
	fmt.Println(`  quote_type       Optional. par (default) or spot`)
	fmt.Println(`  compounding      Optional for quote_type=spot. continuous (default) or simple`)
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println(`  {"curve_date":"2026-05-20","currency":"EUR","index":"ESTR",`)
	fmt.Println(`   "curve_quotes":{"1W":1.93,"1M":1.97,"3M":2.12,"1Y":2.45,...}}`)
}

func readInput(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func writeError(msg string) {
	out := bootstrap.Result{Error: msg}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	os.Exit(1)
}
