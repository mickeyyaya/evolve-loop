package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

func runIssueCoverage(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "generate" && args[0] != "check") {
		fmt.Fprintln(stderr, issueUsage)
		return 10
	}
	verb := "evolve issue coverage " + args[0]
	root, rest, ok := issueFlags(verb, args[1:], stderr)
	if !ok || len(rest) != 0 {
		fmt.Fprintln(stderr, issueUsage)
		return 10
	}
	path := filepath.Join(root, filepath.FromSlash(incidentkb.CoverageIndexRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", verb, err)
		return 1
	}
	rendered, counts, err := incidentkb.RenderCoverageSummary(raw)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", verb, incidentkb.CoverageIndexRel, err)
		return 1
	}
	if args[0] == "check" {
		return checkCoverageSummary(raw, rendered, stdout, stderr)
	}
	if !bytes.Equal(rendered, raw) {
		if err := atomicwrite.Bytes(path, rendered); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", verb, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "rendered coverage summary: incidents=%d rows=%d covered=%d partial=%d none=%d untestable=%d\n",
		counts.Incidents, counts.Rows, counts.Covered, counts.Partial, counts.None, counts.Untestable)
	return 0
}

func checkCoverageSummary(raw, rendered []byte, stdout, stderr io.Writer) int {
	if !bytes.Equal(rendered, raw) {
		fmt.Fprintf(stderr, "evolve issue coverage check: the summary of %s is stale: run evolve issue coverage generate\n", incidentkb.CoverageIndexRel)
		return 1
	}
	fmt.Fprintf(stdout, "coverage summary of %s is current\n", incidentkb.CoverageIndexRel)
	return 0
}
