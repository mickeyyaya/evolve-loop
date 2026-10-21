package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

func runACSTIAReport(args []string, stdout, stderr io.Writer) int {
	const name = "evolve acs tia-report"
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	evolveDir := fs.String("evolve-dir", ".evolve", "path to an existing .evolve/ state directory")
	asJSON := fs.Bool("json", false, "print the rollup as JSON")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	dir, err := existingEvolveDir(*evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 10
	}
	s, err := regressiontia.Rollup(filepath.Join(dir, "runs"))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	if *asJSON {
		raw, _ := json.MarshalIndent(s, "", "  ")
		fmt.Fprintln(stdout, string(raw))
		return 0
	}
	printTIAReport(stdout, s)
	return 0
}

func printTIAReport(w io.Writer, s regressiontia.Summary) {
	share := 0
	if total := s.Selected + s.WouldSkip; total > 0 {
		share = 100 * s.WouldSkip / total
	}
	fmt.Fprintf(w, "[acs tia] %d cycle(s) with a shadow decision: %d selected, %d would skip (%d%%); %d miss(es); selection took %d ms in total, %d ms at most\n", s.Cycles, s.Selected, s.WouldSkip, share, len(s.Misses), s.TotalMS, s.MaxMS)
	for _, m := range s.Misses {
		fmt.Fprintf(w, "  MISS cycle %d: %s would skip %s, which was red\n", m.Cycle, m.Package, m.Predicate)
	}
	for _, c := range s.NoVerdict {
		fmt.Fprintf(w, "  cycle %d has a shadow decision but no acs-verdict.json\n", c)
	}
	for _, u := range s.Unreadable {
		fmt.Fprintf(w, "  unreadable: %s\n", u)
	}
}
