package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/meenmo/molib/bond/greeks"
)

func main() {
	inputPath := flag.String("input", "", "JSON input path (reads stdin if omitted)")
	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Fprintln(os.Stderr, "Usage: krwbondkrd [-input <path>] [path]")
		fmt.Fprintln(os.Stderr, "Compute Key Rate Durations for KRW bonds using Bloomberg Wave methodology.")
		return
	}

	path := strings.TrimSpace(*inputPath)
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	raw, err := readInput(path)
	if err != nil {
		exitError(fmt.Sprintf("read input: %v", err))
	}

	var input greeks.KRDInput
	if err := json.Unmarshal(raw, &input); err != nil {
		exitError(fmt.Sprintf("parse JSON: %v", err))
	}

	output, err := greeks.ComputeKRD(input)
	if err != nil {
		exitError(err.Error())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func readInput(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func exitError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error":"%s"}`+"\n", msg)
	os.Exit(1)
}
