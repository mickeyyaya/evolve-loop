package main

// cmd_salvage.go — `evolve salvage report`: the operator-facing surface for the
// recoverable-malformed `bad_verdict` rate.
//
// The salvage layer's extraction/coercion stage is gated on that rate
// (docs/research/deliverable-alignment-2026-08/README.md §7). Instrumentation
// has been appending .evolve/bad-verdict-baseline.jsonl since cycle-1389 with
// no reader, so the number could only be produced by hand-reading JSONL. This
// command is the reader.
//
// Pure reader — opens the sidecar, folds it, prints. No state, ledger, or
// sidecar mutation, so it is safe to run mid-batch.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

func runSalvage(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve salvage: usage: salvage report [-json] [-project-root P]")
		return 10
	}
	switch args[0] {
	case "report":
		return runSalvageReport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve salvage: unknown subcommand %q (want: report)\n", args[0])
		return 10
	}
}

func runSalvageReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("salvage report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the summary as a JSON envelope instead of prose")
	rootFlag := fs.String("project-root", "", "project root (default $EVOLVE_PROJECT_ROOT, else .)")
	if err := fs.Parse(args); err != nil {
		return 10
	}

	root := *rootFlag
	if root == "" {
		root = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, ".evolve", deliverable.BadVerdictBaselineFile)

	// An absent sidecar is the normal state of a fresh project root, not a
	// failure: the writer only creates it once a bad_verdict block occurs. It
	// still reports through the same envelope, so a consumer never has to
	// special-case "no file" versus "no records".
	summary := deliverable.BaselineSummary{ByPattern: map[deliverable.SalvagePattern]int{}}
	f, err := os.Open(path)
	switch {
	case err == nil:
		summary, err = deliverable.SummarizeBadVerdictBaseline(f)
		_ = f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "salvage report: %v\n", err)
			return 1
		}
	case os.IsNotExist(err):
		// zero summary; reported below with an explicit note on the prose path.
	default:
		fmt.Fprintf(stderr, "salvage report: open %s: %v\n", path, err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			fmt.Fprintf(stderr, "salvage report: encode: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "salvage report: %s\n", path)
	if summary.Total == 0 {
		fmt.Fprintln(stdout, "  no bad_verdict deliverables classified yet — rate 0.000 (0.0%)")
		return 0
	}
	fmt.Fprintf(stdout, "  %d bad_verdict deliverable(s) classified, %d recoverable\n", summary.Total, summary.Recoverable)
	fmt.Fprintf(stdout, "  recoverable-malformed rate: %.3f (%.1f%%)\n", summary.Rate, summary.Rate*100)
	fmt.Fprintln(stdout, "  by pattern:")
	pats := make([]string, 0, len(summary.ByPattern))
	for p := range summary.ByPattern {
		pats = append(pats, string(p))
	}
	sort.Strings(pats) // deterministic output: map iteration order is not.
	for _, p := range pats {
		fmt.Fprintf(stdout, "    %-16s %d\n", p, summary.ByPattern[deliverable.SalvagePattern(p)])
	}
	return 0
}
