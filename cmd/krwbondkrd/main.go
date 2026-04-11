package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/meenmo/molib/bond"
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

	var input bond.KRDInput
	if err := json.Unmarshal(raw, &input); err != nil {
		exitError(fmt.Sprintf("parse JSON: %v", err))
	}

	output, err := bond.ComputeKRD(input)
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
	out := struct {
		Error string `json:"error"`
	}{Error: msg}
	b, _ := json.Marshal(out)
	fmt.Fprintln(os.Stderr, string(b))
	os.Exit(1)
}
