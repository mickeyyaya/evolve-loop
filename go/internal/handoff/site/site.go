package site

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

var floorFailure = regexp.MustCompile(`^\./acs/cycle[0-9]+ \(-tags [^)]*\): unit tests FAIL`)

type Site struct {
	RunDir    string
	InboxRoot string
	Tree      string
	Stale     string
}

type Evaluation struct {
	Pair           string
	RequestAbsent  bool
	LedgerAbsent   bool
	Outcome        string
	Unmet          []string
	Blocked        []string
	Paths          []string
	FailedEvidence []string
	Errors         []string
	Request        handoff.Request
	Ledger         handoff.Ledger
}

func ForPhase(workspace, projectRoot, worktree string) Site {
	tree := worktree
	if tree == "" {
		tree = projectRoot
	}
	return Site{RunDir: workspace, InboxRoot: projectRoot, Tree: tree}
}

func Evaluate(s Site, cycle int, pair string) Evaluation {
	ev := evaluate(s, cycle, pair)
	ev.Outcome, ev.Errors = handoff.ApplyStale(ev.Outcome, ev.Errors, s.Stale)
	return ev
}

func failed(ev Evaluation, outcome string, errs ...string) Evaluation {
	ev.Outcome = outcome
	ev.Errors = append(ev.Errors, errs...)
	return ev
}

func evaluate(s Site, cycle int, pair string) Evaluation {
	ev := Evaluation{Pair: pair}
	req, err := BuildRequest(s, cycle, pair)
	if err != nil {
		ev.RequestAbsent = request.PairAbsent(err)
		return failed(ev, "error", err.Error())
	}
	ev.Request = req
	raw, err := ReadLedger(filepath.Join(s.RunDir, "handoff-"+pair+".json"))
	if err != nil {
		ev.LedgerAbsent = errors.Is(err, fs.ErrNotExist)
		return failed(ev, "error", err.Error())
	}
	ledger, err := handoff.Parse(raw)
	if err == nil {
		err = handoff.Validate(ledger, req)
	}
	if err != nil {
		return failed(ev, "invalid", strings.Split(err.Error(), "\n")...)
	}
	ev.Ledger = ledger
	failures, err := readFailures(filepath.Join(s.RunDir, "handoff-"+pair+"-failures.json"))
	if err != nil {
		return failed(ev, "error", err.Error())
	}
	src := evidence.Source{Root: s.Tree, Cycle: cycle}
	policy := exit.Policy{MaxRedispatch: 1, Prover: prover(s.Tree, cycle, failures)}
	entries, unresolved := results(src, ledger)
	decision := exit.Decide(req, entries, nil, policy)
	ev.Outcome = string(decision.Outcome)
	ev.FailedEvidence = unresolved
	ev.Unmet = decision.Unmet
	ev.Blocked = decision.Blocked
	ev.Paths = decision.Paths
	return ev
}

func BuildRequest(s Site, cycle int, pair string) (handoff.Request, error) {
	return request.ForPair(pair, request.Site{Workspace: s.RunDir, Worktree: s.Tree, Cycle: cycle, Resolve: InboxResolver(s.InboxRoot)})
}

func InboxResolver(root string) request.PathResolver {
	return func(id string) string {
		loc, err := inboxmover.Locate(filepath.Join(root, ".evolve", "inbox"), id)
		if err != nil {
			return ""
		}
		return loc.Path
	}
}

func ReadLedger(path string) ([]byte, error) {
	raw, err := core.ReadHandoffLedger(path)
	if err == nil && handoff.IsCheckOnly(raw) {
		return nil, fmt.Errorf("%s holds only the check record: %w", filepath.Base(path), fs.ErrNotExist)
	}
	return raw, err
}

func readFailures(path string) ([]string, error) {
	raw, err := core.ReadHandoffLedger(path)
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

func results(src evidence.Source, ledger handoff.Ledger) ([]exit.Result, []string) {
	out := make([]exit.Result, 0, len(ledger.Entries))
	var failed []string
	for _, e := range ledger.Entries {
		unresolved := unresolvedEvidence(src, e)
		failed = append(failed, unresolved...)
		res := exit.Result{Ask: e.Ask, Status: e.Status, Resolved: len(e.Evidence) > 0 && len(unresolved) == 0}
		if e.Status == handoff.StatusBlocked {
			res.Claim = e.Note
		}
		out = append(out, res)
	}
	return out, failed
}

func unresolvedEvidence(src evidence.Source, e handoff.Entry) []string {
	if e.Status != handoff.StatusMet {
		return nil
	}
	if len(e.Evidence) == 0 {
		return []string{e.Ask + ": a met entry cites no evidence"}
	}
	var failed []string
	for _, ev := range e.Evidence {
		if err := evidence.Resolve(src, ev); err != nil {
			failed = append(failed, e.Ask+": "+err.Error())
		}
	}
	return failed
}

func predicateSources(root string, cycle int) map[string]string {
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

func prover(root string, cycle int, failures []string) exit.Prover {
	predicates := predicateSources(root, cycle)
	return func(defects []string) exit.Proof {
		proof := core.NewBlockerProver().Prove(core.BlockerCase{
			Phase:               "build",
			Worktree:            root,
			PredicatesAtRequest: predicates,
			PredicatesNow:       predicates,
			Protected:           guards.TouchesProtectedSurface,
			Failures:            failures,
			Defects:             defects,
			Explainable:         floorFailure.MatchString,
		})
		out := exit.Proof{Blocked: proof.Blocked()}
		for _, c := range proof.Proven {
			out.Proven = append(out.Proven, exit.Claim{Defect: c.Defect, Path: c.Path, Failing: c.Failing, Guard: c.Guard})
		}
		return out
	}
}
