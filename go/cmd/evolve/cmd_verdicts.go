package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

const verdictsUsage = "evolve verdicts: usage: verdicts stats [--project-root P] | verdicts wipe [--apply] [--project-root P]"

func runVerdicts(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, verdictsUsage)
		return 10
	}
	switch args[0] {
	case "stats":
		return runVerdictsStats(args[1:], stdout, stderr)
	case "wipe":
		return runVerdictsWipe(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s\nevolve verdicts: unknown subcommand %q (want: stats, wipe)\n", verdictsUsage, args[0])
		return 10
	}
}

func verdictsProjectRoot(flagRoot string) string {
	if flagRoot != "" {
		return flagRoot
	}
	if env := os.Getenv("EVOLVE_PROJECT_ROOT"); env != "" {
		return env
	}
	return "."
}

func verdictsFlags(name string, args []string, stderr io.Writer, apply *bool) (*testverdict.Store, bool) {
	fs := flag.NewFlagSet("verdicts "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "project root (default $EVOLVE_PROJECT_ROOT, else .)")
	if apply != nil {
		fs.BoolVar(apply, "apply", false, "empty the store; without it the wipe only counts what it would remove")
	}
	if err := fs.Parse(args); err != nil {
		return nil, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s\nevolve verdicts %s: unexpected arguments %q\n", verdictsUsage, name, fs.Args())
		return nil, false
	}
	return testverdict.NewStore(testverdict.DirFor(verdictsProjectRoot(*root)), time.Now), true
}

func runVerdictsStats(args []string, stdout, stderr io.Writer) int {
	store, ok := verdictsFlags("stats", args, stderr, nil)
	if !ok {
		return 10
	}
	st, err := store.Stats()
	if err != nil {
		fmt.Fprintf(stderr, "evolve verdicts: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "%s: %d verdict(s), %d file(s), %d bytes; by gate: %s\n", store.Root(), st.Entries, st.Files, st.Bytes, byGate(st.ByGate))
	return 0
}

func runVerdictsWipe(args []string, stdout, stderr io.Writer) int {
	var apply bool
	store, ok := verdictsFlags("wipe", args, stderr, &apply)
	if !ok {
		return 10
	}
	if !apply {
		st, err := store.Stats()
		if err != nil {
			fmt.Fprintf(stderr, "evolve verdicts: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s: would wipe %d verdict(s) in %d file(s); run again with --apply to empty the store\n", store.Root(), st.Entries, st.Files)
		return 0
	}
	st, err := store.Wipe()
	if err != nil {
		fmt.Fprintf(stderr, "evolve verdicts: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "%s: wiped %d verdict(s) in %d file(s)\n", store.Root(), st.Entries, st.Files)
	return 0
}

func byGate(counts map[string]int) string {
	parts := make([]string, 0, len(counts))
	for gate, n := range counts {
		if gate == "" {
			gate = "(none)"
		}
		parts = append(parts, fmt.Sprintf("%s %d", gate, n))
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}
