package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/meenmo/molib/cmd/npv/internal/irs"
	"github.com/meenmo/molib/cmd/npv/internal/krxirs"
	"github.com/meenmo/molib/cmd/npv/internal/ois"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "irs":
		return irs.Run(args[1:], stdin, stdout, stderr)
	case "ois":
		return ois.Run(args[1:], stdin, stdout, stderr)
	case "krx-irs", "krxirs":
		return krxirs.Run(args[1:], stdin, stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: npv <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  irs      Vanilla fixed-vs-IBOR IRS NPV")
	fmt.Fprintln(w, "  ois      Vanilla OIS NPV")
	fmt.Fprintln(w, "  krx-irs  KRX CD91 IRS NPV")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `npv <command> -h` for command-specific help.")
}
