package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	ladderUsage        = "Usage: evolve ladder report [--json] [--project-root DIR]"
	exitLadderInput    = 2
	exitLadderUsage    = 10
	ladderClassNone    = "-"
	ladderTextHeader   = "RUNG\tCLASS\tTRIES\tFIXES\tTOKENS"
	ladderRowTextShape = "%s\t%s\t%d\t%d\t%d\n"
)

type ladderReportJSON struct {
	Runs int                     `json:"runs"`
	Rows []retryladder.ReportRow `json:"rows"`
}

func runLadderCommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, ladderUsage)
		return 0
	}
	if len(args) == 0 || args[0] != "report" {
		fmt.Fprintln(stderr, ladderUsage)
		return exitLadderUsage
	}
	return runLadderReport(args[1:], os.Getwd, stdout, stderr)
}

func runLadderReport(args []string, getwd func() (string, error), stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ladder report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print the report as JSON")
	root := fs.String("project-root", "", "project root (default: EVOLVE_PROJECT_ROOT or the cwd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, ladderUsage)
			return 0
		}
		fmt.Fprintf(stderr, "evolve ladder report: %v\n%s\n", err, ladderUsage)
		return exitLadderUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, ladderUsage)
		return exitLadderUsage
	}
	projectRoot, err := routingProjectRoot(*root, getwd)
	if err != nil {
		fmt.Fprintf(stderr, "evolve ladder report: %v\n", err)
		return exitLadderInput
	}
	runs, err := retryladder.LoadRuns(filepath.Join(projectRoot, ".evolve", "runs"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve ladder report: cannot read the runs dir: %v\n", err)
		return exitLadderInput
	}
	rows := retryladder.BuildReport(runs)
	if *asJSON {
		return writeLadderJSON(stdout, stderr, ladderReportJSON{Runs: len(runs), Rows: rows})
	}
	writeLadderText(stdout, rows)
	return 0
}

func writeLadderJSON(stdout, stderr io.Writer, report ladderReportJSON) int {
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintf(stderr, "evolve ladder report: %v\n", err)
		return 1
	}
	return 0
}

func writeLadderText(stdout io.Writer, rows []retryladder.ReportRow) {
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, ladderTextHeader)
	for _, row := range rows {
		class := row.Class
		if class == "" {
			class = ladderClassNone
		}
		t := row.Tokens
		fmt.Fprintf(w, ladderRowTextShape, row.Rung, class, row.Tries, row.Fixes, t.Input+t.Output+t.CacheRead+t.CacheWrite)
	}
	w.Flush()
}
