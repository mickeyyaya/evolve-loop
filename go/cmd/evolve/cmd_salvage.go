package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const salvageUsage = "evolve salvage: usage: salvage report [-json] [-project-root P] | salvage list [--json] [--project-root P]"

var salvageGitRunner sysexec.RunFunc = sysexec.DefaultRunner

func runSalvage(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, salvageUsage)
		return 10
	}
	switch args[0] {
	case "report":
		return runSalvageReport(args[1:], stdout, stderr)
	case "list":
		return runSalvageList(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve salvage: unknown subcommand %q (want: report, list)\n", args[0])
		return 10
	}
}

func salvageProjectRoot(flagRoot string) string {
	if flagRoot != "" {
		return flagRoot
	}
	if env := os.Getenv("EVOLVE_PROJECT_ROOT"); env != "" {
		return env
	}
	return "."
}

type salvageListRow struct {
	gc.SalvageLeaf
	Landed *bool `json:"landed"`
}

func runSalvageList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("salvage list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the leaves as a JSON array")
	rootFlag := fs.String("project-root", "", "project root (default $EVOLVE_PROJECT_ROOT, else .)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s\nevolve salvage list: unexpected arguments %q\n", salvageUsage, fs.Args())
		return 10
	}
	root := salvageProjectRoot(*rootFlag)
	leaves, listErr := gc.ListSalvage(filepath.Join(root, ".evolve"))
	rows := make([]salvageListRow, 0, len(leaves))
	for _, l := range leaves {
		rows = append(rows, salvageListRow{SalvageLeaf: l, Landed: salvageLanded(root, l.Head)})
	}
	code := printSalvageRows(rows, *asJSON, stdout, stderr)
	if listErr != nil {
		for _, line := range strings.Split(listErr.Error(), "\n") {
			fmt.Fprintf(stderr, "evolve salvage list: %s\n", line)
		}
		return 2
	}
	return code
}

func salvageLanded(root, head string) *bool {
	_, _, code, err := gitexec.Git{Dir: root, Exec: salvageGitRunner}.Capture(context.Background(), "merge-base", "--is-ancestor", head, "origin/main")
	if err != nil || (code != 0 && code != 1) {
		return nil
	}
	landed := code == 0
	return &landed
}

func printSalvageRows(rows []salvageListRow, asJSON bool, stdout, stderr io.Writer) int {
	if len(rows) == 0 {
		return 0
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "evolve salvage list: encode: %v\n", err)
			return 2
		}
		return 0
	}
	now := time.Now()
	for _, r := range rows {
		branch := r.Branch
		if branch == "" {
			branch = "-"
		}
		fmt.Fprintf(stdout, "%s  cycle=%d  branch=%s  head=%s  changed=%d  patch=%dB  untracked=%d  age=%dh  landed=%s\n",
			r.Leaf, r.Cycle, branch, r.Head, r.ChangedFiles, r.PatchBytes, r.UntrackedFiles, int(now.Sub(r.SalvagedAt).Hours()), landedWord(r.Landed))
	}
	return 0
}

func landedWord(landed *bool) string {
	switch {
	case landed == nil:
		return "unknown"
	case *landed:
		return "yes"
	default:
		return "no"
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

	root := salvageProjectRoot(*rootFlag)
	path := filepath.Join(root, ".evolve", deliverable.BadVerdictBaselineFile)

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
	default:
		fmt.Fprintf(stderr, "salvage report: open %s: %v\n", path, err)
		return 1
	}

	appliedPath := filepath.Join(root, ".evolve", deliverable.SalvageAppliedFile)
	af, err := os.Open(appliedPath)
	switch {
	case err == nil:
		summary.Saved, summary.Malformed, err = deliverable.CountSalvageApplied(af)
		_ = af.Close()
		if err != nil {
			fmt.Fprintf(stderr, "salvage report: %v\n", err)
			return 1
		}
	case os.IsNotExist(err):
	default:
		fmt.Fprintf(stderr, "salvage report: open %s: %v\n", appliedPath, err)
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
	if summary.Malformed > 0 {
		fmt.Fprintf(stdout, "  WARN: %d unreadable record(s) skipped in %s — counts below are a floor\n",
			summary.Malformed, deliverable.SalvageAppliedFile)
	}
	if summary.Total == 0 {
		fmt.Fprintln(stdout, "  no bad_verdict deliverables classified yet — rate 0.000 (0.0%)")
		return 0
	}
	fmt.Fprintf(stdout, "  %d bad_verdict deliverable(s) classified, %d recoverable, %d actually salvaged\n",
		summary.Total, summary.Recoverable, summary.Saved)
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
