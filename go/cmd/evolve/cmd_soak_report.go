package main

// cmd_soak_report.go — R8.3: `evolve soak-report --cycles A-B` renders the
// read-only soak evidence table (internal/soakreport) the
// EVOLVE_PHASE_RECOVERY enforce flip is gated on. Pure reader: no state,
// ledger, or registry mutation; safe to run mid-batch.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/soakreport"
)

func runSoakReport(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("soak-report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cyclesFlag := fs.String("cycles", "", "cycle range A-B or single cycle N (required)")
	root := fs.String("project-root", "", "project root (default: cwd or EVOLVE_PROJECT_ROOT)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	cycles, err := parseCycleRange(*cyclesFlag)
	if err != nil {
		fmt.Fprintf(stderr, "evolve soak-report: %v\n", err)
		return 10
	}
	pr := *root
	if pr == "" {
		pr = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if pr == "" {
		if pr, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "evolve soak-report: %v\n", err)
			return 1
		}
	}
	fmt.Fprint(stdout, soakreport.Collect(pr, cycles).Render())
	return 0
}

// parseCycleRange accepts "N" or "A-B" (inclusive, ascending).
func parseCycleRange(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("--cycles is required (e.g. --cycles 281-284)")
	}
	lo, hi, found := strings.Cut(s, "-")
	a, err := strconv.Atoi(strings.TrimSpace(lo))
	if err != nil || a <= 0 {
		return nil, fmt.Errorf("bad cycle %q (want >=1)", lo)
	}
	if !found {
		return []int{a}, nil
	}
	b, err := strconv.Atoi(strings.TrimSpace(hi))
	if err != nil || b < a {
		return nil, fmt.Errorf("bad range %q (want A-B ascending)", s)
	}
	out := make([]int, 0, b-a+1)
	for c := a; c <= b; c++ {
		out = append(out, c)
	}
	return out, nil
}
