package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func runConvergenceMetrics(args []string, stdout, stderr io.Writer) int {
	var asJSON bool
	fs := flag.NewFlagSet("convergence metrics", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&asJSON, "json", false, "print the metrics as JSON")
	if err := fs.Parse(args); err != nil {
		return exitConvergenceBad
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "evolve convergence metrics: "+convergenceUsage)
		return exitConvergenceBad
	}
	inputs, err := readConvergenceRounds(fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "convergence metrics: %v\n", err)
		return exitConvergenceBad
	}
	m := convergence.Measure(inputs)
	if asJSON {
		return encodeConvergenceMetrics(m, stdout, stderr)
	}
	fmt.Fprint(stdout, convergenceMetricsText(m))
	return 0
}

func readConvergenceRounds(paths []string) ([]convergence.Input, error) {
	inputs := make([]convergence.Input, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		in, err := convergence.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func encodeConvergenceMetrics(m convergence.Metrics, stdout, stderr io.Writer) int {
	return encodeConvergenceJSON("metrics", m, stdout, stderr)
}

func convergenceMetricsText(m convergence.Metrics) string {
	var b strings.Builder
	b.WriteString("rounds to land:\n")
	for _, r := range m.RoundsToLand {
		fmt.Fprintf(&b, "  %s: median %g, max %d, runs %d\n", r.Loop, r.Median, r.Max, r.Runs)
	}
	fmt.Fprintf(&b, "deferred then closed: %d of %d (%.2f)\n", m.DeferredClosed.Closed, m.DeferredClosed.Deferred, m.DeferredClosed.Share)
	fmt.Fprintf(&b, "falsification drop rate: %d of %d (%.2f)\n", m.Falsification.Refuted, m.Falsification.Checked, m.Falsification.Rate)
	return b.String()
}
