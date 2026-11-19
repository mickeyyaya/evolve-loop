package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentbundle"
	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const issueUsage = `Usage:
  evolve issue evidence add [--project-root P] <incident-id> <source-file>
  evolve issue check [--project-root P] [<incident-id>]
  evolve issue list [--project-root P] [--status S] [--code C] [--json]`

func runIssue(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) >= 2 && args[0] == "evidence" && args[1] == "add":
		return runIssueEvidenceAdd(args[2:], stdout, stderr)
	case len(args) >= 1 && args[0] == "check":
		return runIssueCheck(args[1:], stdout, stderr)
	case len(args) >= 1 && args[0] == "list":
		return runIssueList(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "evolve issue: expected `evidence add`, `check`, or `list`\n"+issueUsage)
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
	results, legacy, err := issueCheckResults(root, rest)
	if err != nil {
		fmt.Fprintf(stderr, "evolve issue check: %v\n", err)
		return 1
	}
	return printIssueCheck(results, legacy, stdout, stderr)
}

func issueCheckResults(root string, ids []string) (map[string][]string, []string, error) {
	if len(ids) == 0 {
		bundleRes, err := incidentbundle.CheckAll(root)
		if err != nil {
			return nil, nil, err
		}
		kbReport, err := incidentkb.CheckAll(root)
		if err != nil {
			return nil, nil, err
		}
		merged := make(map[string][]string, len(bundleRes)+len(kbReport.Problems))
		for id, probs := range bundleRes {
			merged[id] = append(merged[id], probs...)
		}
		for id, probs := range kbReport.Problems {
			merged[id] = append(merged[id], probs...)
		}
		return merged, kbReport.Legacy, nil
	}
	return issueCheckOne(root, ids[0])
}

func issueCheckOne(root, id string) (map[string][]string, []string, error) {
	bundleProbs, bundleErr := incidentbundle.Check(root, id)
	kbProbs, kbLegacy, kbErr := incidentkb.Check(root, id)
	if bundleErr != nil && !errors.Is(bundleErr, os.ErrNotExist) {
		return nil, nil, bundleErr
	}
	if bundleErr != nil && kbErr != nil {
		return nil, nil, bundleErr
	}
	if kbLegacy && errors.Is(bundleErr, os.ErrNotExist) {
		return map[string][]string{}, []string{id}, nil
	}
	merged := append([]string{}, bundleProbs...)
	merged = append(merged, kbProbs...)
	return map[string][]string{id: merged}, nil, nil
}

func printIssueCheck(results map[string][]string, legacy []string, stdout, stderr io.Writer) int {
	for _, id := range legacy {
		fmt.Fprintf(stdout, "legacy %s\n", id)
	}
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

func runIssueList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve issue list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", ".", "repository root holding docs/incidents/")
	status := fs.String("status", "", "filter records by status")
	code := fs.String("code", "", "filter records by effectiveness metric code")
	jsonOut := fs.Bool("json", false, "output JSON array")
	if fs.Parse(args) != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, issueUsage)
		return 10
	}
	rows, err := incidentkb.List(*projectRoot, *status, *code)
	if err != nil {
		fmt.Fprintf(stderr, "evolve issue list: %v\n", err)
		return 1
	}
	if *jsonOut {
		raw, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Fprintf(stdout, "%s\n", raw)
		return 0
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s\t%s\tlegacy=%t\n", r.ID, r.Status, r.Legacy)
	}
	return 0
}
