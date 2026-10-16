package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// runLessons implements `evolve lessons recurrence`, which renders the
// deterministic recurrence ledger (internal/recurrence). It is a pure reader —
// no state mutation — so it is safe to run mid-batch.
func runLessons(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve lessons: usage: lessons recurrence [--project-root P] | backfill [--write] [--project-root P]")
		return 10
	}
	switch args[0] {
	case "recurrence":
		return runLessonsRecurrence(stdout, stderr)
	case "backfill":
		return runLessonsBackfill(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve lessons: unknown subcommand %q (want: recurrence or backfill)\n", args[0])
		return 10
	}
}

func runLessonsBackfill(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lessons backfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		write       bool
		projectRoot string
	)
	fs.BoolVar(&write, "write", false, "replace ledger atomically")
	fs.StringVar(&projectRoot, "project-root", "", "project root directory")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "evolve lessons backfill: unexpected argument %q\n", fs.Args()[0])
		return 10
	}
	root := "."
	if projectRoot != "" {
		root = projectRoot
	} else if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	lessonsDir := filepath.Join(root, ".evolve", "instincts", "lessons")
	st, err := os.Stat(lessonsDir)
	if err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "evolve lessons backfill: lessons directory not accessible: %v\n", err)
		return 2
	}
	led, skipped, err := recurrence.BackfillFromLessons(lessonsDir, recurrence.DefaultEscalationPolicy())
	if err != nil {
		fmt.Fprintf(stderr, "evolve lessons backfill: %v\n", err)
		return 2
	}
	for _, f := range skipped {
		fmt.Fprintf(stdout, "skipped: %s\n", f)
	}
	renderRecurrenceReport(stdout, led)
	if write {
		target := filepath.Join(root, ".evolve", "recurrence-ledger.json")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			fmt.Fprintf(stderr, "evolve lessons backfill: create dir: %v\n", err)
			return 2
		}
		if err := led.Save(target); err != nil {
			fmt.Fprintf(stderr, "evolve lessons backfill: save ledger: %v\n", err)
			return 2
		}
	}
	return 0
}

func runLessonsRecurrence(stdout, stderr io.Writer) int {
	root := "."
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	led, err := recurrence.Load(filepath.Join(root, ".evolve", "recurrence-ledger.json"))
	if err != nil {
		fmt.Fprintf(stderr, "lessons recurrence: load ledger: %v\n", err)
		return 1
	}
	renderRecurrenceReport(stdout, led)
	return 0
}

// renderRecurrenceReport prints the ledger's patterns sorted by descending
// count (via Ledger.Patterns), one row per pattern with its recurrence count
// and fix-item status ("fix=<id>" when a linked open item is recorded, else
// "fix=none"). Pure and I/O-injected so the CLI axis is unit-testable.
func renderRecurrenceReport(w io.Writer, led *recurrence.Ledger) {
	pats := led.Patterns()
	if len(pats) == 0 {
		fmt.Fprintln(w, "recurrence: ledger is empty")
		return
	}
	fmt.Fprintln(w, "COUNT  PATTERN  FIX")
	for _, e := range pats {
		if e.Generic {
			continue // classification noise — de-noised out of the operator report
		}
		fix := "none"
		if e.FixItemID != "" {
			fix = e.FixItemID
		}
		fmt.Fprintf(w, "%d  %s  fix=%s\n", e.Count, e.Pattern, fix)
	}
}
