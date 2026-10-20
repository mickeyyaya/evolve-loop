package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const (
	handoffExitPass    = 0
	handoffExitBlocked = 1
	handoffExitIO      = 2
	handoffExitUsage   = 10
	handoffUsage       = `usage: evolve handoff check --cycle N --pair triage-tdd|tdd-build [--json] [--project-root P]
       evolve handoff show --cycle N [--json] [--project-root P]`
)

var (
	handoffAbsPath      = filepath.Abs
	handoffPairs        = []string{request.PairTriageTDD, request.PairTDDBuild}
	handoffFloorFailure = regexp.MustCompile(`^\./acs/cycle[0-9]+ \(-tags [^)]*\): unit tests FAIL`)
)

type handoffArgs struct {
	cycle  int
	pair   string
	root   string
	asJSON bool
}

type handoffResult struct {
	Cycle   int      `json:"cycle"`
	Pair    string   `json:"pair"`
	Outcome string   `json:"outcome"`
	Exit    int      `json:"exit"`
	Unmet   []string `json:"unmet"`
	Blocked []string `json:"blocked"`
	Paths   []string `json:"paths"`
	Errors  []string `json:"errors"`
}

type handoffPairOutcome struct {
	Pair    string `json:"pair"`
	Outcome string `json:"outcome"`
}

type handoffShowResult struct {
	Cycle   int                  `json:"cycle"`
	Outcome string               `json:"outcome"`
	Exit    int                  `json:"exit"`
	Pairs   []handoffPairOutcome `json:"pairs"`
	Errors  []string             `json:"errors"`
}

func runHandoff(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	asJSON := slices.Contains(args, "--json") || slices.Contains(args, "-json")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return handoffUsageFailure(asJSON, errors.New("a verb is required: check or show"), stdout, stderr)
	}
	switch args[0] {
	case "check":
		return runHandoffCheck(args[1:], asJSON, stdout, stderr)
	case "show":
		return runHandoffShow(args[1:], asJSON, stdout, stderr)
	}
	return handoffUsageFailure(asJSON, fmt.Errorf("unknown verb %q", args[0]), stdout, stderr)
}

func handoffUsageFailure(asJSON bool, err error, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "evolve handoff: %v\n%s\n", err, handoffUsage)
	if asJSON {
		writeHandoffJSON(stdout, stderr, handoffResult{Outcome: "usage", Exit: handoffExitUsage, Errors: []string{err.Error()}})
	}
	return handoffExitUsage
}

func parseHandoffArgs(verb string, args []string) (handoffArgs, error) {
	var a handoffArgs
	flags := flag.NewFlagSet("handoff "+verb, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.IntVar(&a.cycle, "cycle", 0, "")
	flags.StringVar(&a.root, "project-root", "", "")
	flags.BoolVar(&a.asJSON, "json", false, "")
	if verb == "check" {
		flags.StringVar(&a.pair, "pair", "", "")
	}
	if err := flags.Parse(args); err != nil {
		return a, err
	}
	switch {
	case flags.NArg() > 0:
		return a, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	case a.cycle <= 0:
		return a, errors.New("--cycle N is required and must be positive")
	case verb == "check" && !slices.Contains(handoffPairs, a.pair):
		return a, fmt.Errorf("--pair must be one of %s", strings.Join(handoffPairs, ", "))
	}
	return a, nil
}

func handoffProjectRoot(flagValue string) (string, error) {
	root := flagValue
	if root == "" {
		root = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	return handoffAbsPath(root)
}

func runHandoffCheck(args []string, asJSON bool, stdout, stderr io.Writer) int {
	a, err := parseHandoffArgs("check", args)
	if err != nil {
		return handoffUsageFailure(asJSON, err, stdout, stderr)
	}
	root, err := handoffProjectRoot(a.root)
	if err != nil {
		return handoffUsageFailure(a.asJSON, err, stdout, stderr)
	}
	res := evaluateHandoff(root, a.cycle, a.pair)
	if a.asJSON {
		writeHandoffJSON(stdout, stderr, res)
		return res.Exit
	}
	printHandoffResult(stdout, stderr, res)
	return res.Exit
}

func printHandoffResult(stdout, stderr io.Writer, res handoffResult) {
	if res.Outcome == "error" {
		fmt.Fprintf(stderr, "evolve handoff: %s: %s\n", res.Pair, strings.Join(res.Errors, "; "))
		return
	}
	fmt.Fprintf(stdout, "%s: %s\n", res.Pair, res.Outcome)
	for _, line := range []struct {
		label string
		items []string
	}{{"unmet", res.Unmet}, {"blocked", res.Blocked}, {"paths", res.Paths}, {"errors", res.Errors}} {
		if len(line.items) > 0 {
			fmt.Fprintf(stdout, "  %s: %s\n", line.label, strings.Join(line.items, ", "))
		}
	}
}

func runHandoffShow(args []string, asJSON bool, stdout, stderr io.Writer) int {
	a, err := parseHandoffArgs("show", args)
	if err != nil {
		return handoffUsageFailure(asJSON, err, stdout, stderr)
	}
	root, err := handoffProjectRoot(a.root)
	if err != nil {
		return handoffUsageFailure(a.asJSON, err, stdout, stderr)
	}
	res := handoffShowResult{Cycle: a.cycle, Outcome: "listed", Exit: handoffExitPass, Pairs: []handoffPairOutcome{}, Errors: []string{}}
	if _, err := os.Stat(handoffRunDir(root, a.cycle)); err != nil {
		res.Outcome, res.Exit, res.Errors = "error", handoffExitIO, []string{err.Error()}
	}
	for _, pair := range handoffPairs {
		if res.Exit != handoffExitPass {
			break
		}
		res.Pairs = append(res.Pairs, handoffPairOutcome{Pair: pair, Outcome: handoffShowOutcome(root, a.cycle, pair)})
	}
	if a.asJSON {
		writeHandoffJSON(stdout, stderr, res)
		return res.Exit
	}
	if res.Exit != handoffExitPass {
		fmt.Fprintf(stderr, "evolve handoff: %s\n", strings.Join(res.Errors, "; "))
		return res.Exit
	}
	for _, p := range res.Pairs {
		fmt.Fprintf(stdout, "%s: %s\n", p.Pair, p.Outcome)
	}
	return res.Exit
}

func handoffShowOutcome(root string, cycle int, pair string) string {
	if _, err := os.Stat(handoffLedgerPath(root, cycle, pair)); errors.Is(err, fs.ErrNotExist) {
		return "absent"
	}
	return evaluateHandoff(root, cycle, pair).Outcome
}

func writeHandoffJSON(stdout, stderr io.Writer, v any) {
	if err := json.NewEncoder(stdout).Encode(v); err != nil {
		fmt.Fprintf(stderr, "evolve handoff: write json: %v\n", err)
	}
}

func handoffRunDir(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
}

func handoffLedgerPath(root string, cycle int, pair string) string {
	return filepath.Join(handoffRunDir(root, cycle), "handoff-"+pair+".json")
}

func handoffFailuresPath(root string, cycle int, pair string) string {
	return filepath.Join(handoffRunDir(root, cycle), "handoff-"+pair+"-failures.json")
}

func newHandoffResult(cycle int, pair, outcome string, code int, errs ...string) handoffResult {
	return handoffResult{
		Cycle: cycle, Pair: pair, Outcome: outcome, Exit: code,
		Unmet: []string{}, Blocked: []string{}, Paths: []string{}, Errors: append([]string{}, errs...),
	}
}

func evaluateHandoff(root string, cycle int, pair string) handoffResult {
	req, err := buildHandoffRequest(root, cycle, pair)
	if err != nil {
		return newHandoffResult(cycle, pair, "error", handoffExitIO, err.Error())
	}
	raw, err := os.ReadFile(handoffLedgerPath(root, cycle, pair))
	if err != nil {
		return newHandoffResult(cycle, pair, "error", handoffExitIO, err.Error())
	}
	ledger, err := handoff.Parse(raw)
	if err == nil {
		err = handoff.Validate(ledger, req)
	}
	if err != nil {
		return newHandoffResult(cycle, pair, "invalid", handoffExitBlocked, strings.Split(err.Error(), "\n")...)
	}
	failures, err := readHandoffFailures(handoffFailuresPath(root, cycle, pair))
	if err != nil {
		return newHandoffResult(cycle, pair, "error", handoffExitIO, err.Error())
	}
	src := evidence.Source{Root: root, Cycle: cycle}
	policy := exit.Policy{MaxRedispatch: 1, Prover: handoffProver(root, cycle, failures)}
	decision := exit.Decide(req, handoffResults(src, ledger), nil, policy)
	res := newHandoffResult(cycle, pair, string(decision.Outcome), handoffExitBlocked)
	if decision.Outcome == exit.Pass {
		res.Exit = handoffExitPass
	}
	res.Unmet = append(res.Unmet, decision.Unmet...)
	res.Blocked = append(res.Blocked, decision.Blocked...)
	res.Paths = append(res.Paths, decision.Paths...)
	return res
}

func buildHandoffRequest(root string, cycle int, pair string) (handoff.Request, error) {
	switch pair {
	case request.PairTDDBuild:
		return request.TDDBuild(filepath.Join(root, "go", "acs", fmt.Sprintf("cycle%d", cycle)))
	case request.PairTriageTDD:
		return request.TriageTDD(handoffRunDir(root, cycle), handoffInboxResolver(root))
	}
	return handoff.Request{}, fmt.Errorf("no request for pair %s", pair)
}

func handoffInboxResolver(root string) request.PathResolver {
	return func(id string) string {
		loc, err := inboxmover.Locate(filepath.Join(root, ".evolve", "inbox"), id)
		if err != nil {
			return ""
		}
		return loc.Path
	}
}

func readHandoffFailures(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var failures []string
	if err := json.Unmarshal(raw, &failures); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return failures, nil
}

func handoffResults(src evidence.Source, ledger handoff.Ledger) []exit.Result {
	results := make([]exit.Result, 0, len(ledger.Entries))
	for _, e := range ledger.Entries {
		res := exit.Result{Ask: e.Ask, Status: e.Status, Resolved: handoffEvidenceResolves(src, e.Evidence)}
		if e.Status == handoff.StatusBlocked {
			res.Claim = e.Note
		}
		results = append(results, res)
	}
	return results
}

func handoffEvidenceResolves(src evidence.Source, evs []handoff.Evidence) bool {
	if len(evs) == 0 {
		return false
	}
	for _, ev := range evs {
		if evidence.Resolve(src, ev) != nil {
			return false
		}
	}
	return true
}

func handoffPredicateSources(root string, cycle int) map[string]string {
	sources := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(root, "go", "acs", fmt.Sprintf("cycle%d", cycle), "*.go"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			sources[filepath.Base(f)] = "unreadable: " + err.Error()
			continue
		}
		sources[filepath.Base(f)] = string(data)
	}
	return sources
}

func handoffProver(root string, cycle int, failures []string) exit.Prover {
	predicates := handoffPredicateSources(root, cycle)
	return func(defects []string) exit.Proof {
		proof := core.NewBlockerProver().Prove(core.BlockerCase{
			Phase:               "build",
			Worktree:            root,
			PredicatesAtRequest: predicates,
			PredicatesNow:       predicates,
			Protected:           guards.TouchesProtectedSurface,
			Failures:            failures,
			Defects:             defects,
			Explainable:         handoffFloorFailure.MatchString,
		})
		out := exit.Proof{Blocked: proof.Blocked()}
		for _, c := range proof.Proven {
			out.Proven = append(out.Proven, exit.Claim{Defect: c.Defect, Path: c.Path, Failing: c.Failing, Guard: c.Guard})
		}
		return out
	}
}
