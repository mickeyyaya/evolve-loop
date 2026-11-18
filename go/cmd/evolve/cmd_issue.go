package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentbundle"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const issueUsage = `Usage:
  evolve issue evidence add [--project-root P] <incident-id> <source-file>
  evolve issue check [--project-root P] [<incident-id>]`

func runIssue(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) >= 2 && args[0] == "evidence" && args[1] == "add":
		return runIssueEvidenceAdd(args[2:], stdout, stderr)
	case len(args) >= 1 && args[0] == "check":
		return runIssueCheck(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "evolve issue: expected `evidence add` or `check`\n"+issueUsage)
	return 10
}

func issueFlags(name string, args []string, stderr io.Writer) (root string, rest []string, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", ".", "repository root holding knowledge-base/incidents/")
	if fs.Parse(args) != nil {
		return "", nil, false
	}
	return *projectRoot, fs.Args(), true
}

func runIssueEvidenceAdd(args []string, stdout, stderr io.Writer) int {
	root, rest, ok := issueFlags("evolve issue evidence add", args, stderr)
	if !ok || len(rest) != 2 {
		fmt.Fprintln(stderr, issueUsage)
		return 10
	}
	pol := policy.LoadOrDefaults(filepath.Join(root, ".evolve", "policy.json"), stderr)
	entry, err := incidentbundle.Add(root, rest[0], rest[1], pol.IncidentEvidenceBudgetBytes(), time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "evolve issue evidence add: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "evidence %s %s state=%s redactions=%d\n", rest[0], entry.ID, entry.State, entry.Redactions)
	return 0
}

func runIssueCheck(args []string, stdout, stderr io.Writer) int {
	root, rest, ok := issueFlags("evolve issue check", args, stderr)
	if !ok || len(rest) > 1 {
		fmt.Fprintln(stderr, issueUsage)
		return 10
	}
	results, err := issueCheckResults(root, rest)
	if err != nil {
		fmt.Fprintf(stderr, "evolve issue check: %v\n", err)
		return 1
	}
	return printIssueCheck(results, stdout, stderr)
}

func issueCheckResults(root string, ids []string) (map[string][]string, error) {
	if len(ids) == 0 {
		return incidentbundle.CheckAll(root)
	}
	problems, err := incidentbundle.Check(root, ids[0])
	if err != nil {
		return nil, err
	}
	return map[string][]string{ids[0]: problems}, nil
}

func printIssueCheck(results map[string][]string, stdout, stderr io.Writer) int {
	ids := make([]string, 0, len(results))
	for id := range results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	failed := 0
	for _, id := range ids {
		if len(results[id]) == 0 {
			fmt.Fprintf(stdout, "ok %s\n", id)
			continue
		}
		failed++
		for _, p := range results[id] {
			fmt.Fprintf(stderr, "FAIL %s: %s\n", id, p)
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}
