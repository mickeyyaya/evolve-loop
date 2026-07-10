package main

// cmd_lessons.go — `evolve lessons recurrence` renders the deterministic
// recurrence ledger (internal/recurrence): patterns sorted by descending count
// with each pattern's fix-item status. Pure reader — no state mutation, safe to
// run mid-batch.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func runLessons(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve lessons: usage: lessons recurrence [--project-root P]")
		return 10
	}
	switch args[0] {
	case "recurrence":
		return runLessonsRecurrence(stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve lessons: unknown subcommand %q (want: recurrence)\n", args[0])
		return 10
	}
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
		fix := "none"
		if e.FixItemID != "" {
			fix = e.FixItemID
		}
		fmt.Fprintf(w, "%d  %s  fix=%s\n", e.Count, e.Pattern, fix)
	}
}
