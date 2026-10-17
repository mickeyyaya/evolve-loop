package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acspathscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

const joinedPersonaPredicate = `//go:build acs

package cycle10

import (
	"path/filepath"
	"testing"
)

func TestP(t *testing.T) {
	persona := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-auditor.md")
	acsassert.FileExists(t, persona)
}
`

const unprotectedPredicate = `package cycle10

func TestP(t *testing.T) {
	acsassert.FileExists(t, "go/internal/evalgate/materialization.go")
}
`

const tempBasePredicate = `package cycle10

func TestP(t *testing.T) {
	tmp := t.TempDir()
	os.ReadFile(filepath.Join(tmp, "agents", "evolve-auditor.md"))
}
`

const deadCodePredicate = `package cycle10

var unused = "agents/evolve-auditor.md"

func TestP(t *testing.T) {
	_ = filepath.Join("agents", "evolve-auditor.md")
	t.Errorf("the line in agents/evolve-auditor.md must stay")
}
`

type predicateWritingTDD struct {
	body, extraFile string
	blockDir        string
	blockFile       string
}

func (predicateWritingTDD) Name() string { return string(PhaseTDD) }

func (r predicateWritingTDD) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	dir := filepath.Join(req.Worktree, "go", "acs", "cycle"+itoa(req.Cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return PhaseResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(r.body), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	if r.extraFile != "" {
		if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte(r.extraFile), 0o644); err != nil {
			return PhaseResponse{}, err
		}
	}
	if r.blockDir != "" {
		if err := os.MkdirAll(filepath.Join(req.Workspace, r.blockDir), 0o755); err != nil {
			return PhaseResponse{}, err
		}
	}
	if r.blockFile != "" {
		if err := os.WriteFile(filepath.Join(req.Workspace, r.blockFile), nil, 0o644); err != nil {
			return PhaseResponse{}, err
		}
	}
	return PhaseResponse{Phase: string(PhaseTDD), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func runTDDPredicateCycle(t *testing.T, predicate string, member func(string) bool, decision string, env map[string]string) (CycleResult, map[Phase]PhaseRunner, string) {
	result, runners, workspace, _ := runTDDPredicateCycleWT(t, predicateWritingTDD{body: predicate}, member, decision, env)
	return result, runners, workspace
}

func runTDDPredicateCycleWT(t *testing.T, tdd PhaseRunner, member func(string) bool, decision string, env map[string]string) (CycleResult, map[Phase]PhaseRunner, string, *fakeWorktree) {
	t.Helper()
	root := writeClaimableInbox(t, 1)
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: decision}
	runners[PhaseTDD] = tdd
	worktree := &fakeWorktree{path: t.TempDir()}
	opts := []Option{WithWorktreeProvisioner(worktree)}
	if member != nil {
		opts = append(opts, WithProtectedSurface(member))
	}
	result, err := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners, opts...).RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "lane-goal",
		Env:         env,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return result, runners, filepath.Join(root, ".evolve", "runs", "cycle-10"), worktree
}

var laneEnv = map[string]string{ipcenv.FleetScopeKey: laneScopedItem}

func protectsAuditorPersona(p string) bool { return p == "agents/evolve-auditor.md" }

const oneCardDecision = `{"top_n":[{"id":"claimable-lane-item"}]}`

func buildWasDispatched(runners map[Phase]PhaseRunner) bool {
	fr, ok := runners[PhaseBuild].(*fakeRunner)
	return ok && len(fr.requests) > 0
}

func routed(result CycleResult) bool {
	return result.TerminationReason == CycleTerminationTddProtectedRoute
}

func TestRunCycle_APredicateThatReadsAProtectedPathStopsAfterTddAsAFailureWithTheRouteCode(t *testing.T) {
	t.Parallel()
	result, runners, workspace, worktree := runTDDPredicateCycleWT(t, predicateWritingTDD{body: joinedPersonaPredicate}, protectsAuditorPersona, oneCardDecision, laneEnv)
	if !routed(result) {
		t.Fatalf("TerminationReason = %q, want %q", result.TerminationReason, CycleTerminationTddProtectedRoute)
	}
	if result.FinalVerdict != VerdictFAIL {
		t.Errorf("FinalVerdict = %q: the route is decoupled from the verdict, so every counter sees a FAIL", result.FinalVerdict)
	}
	if buildWasDispatched(runners) {
		t.Error("build ran after the route: the cycle must stop at tdd")
	}
	if len(worktree.cleaned) != 0 {
		t.Errorf("a sealed FAIL keeps its worktree for forensics like any other FAIL: cleaned=%v", worktree.cleaned)
	}
	if route, ok := committedset.ReadProtectedRoute(workspace); !ok || route.ItemID != laneScopedItem || route.Cycle != 10 {
		t.Errorf("route artifact = %+v ok=%v", route, ok)
	}
	if _, err := os.Stat(filepath.Join(workspace, committedset.ProtectedPredicatesDir, "predicates_test.go")); err != nil {
		t.Errorf("the predicate is kept for the console and for the closeout: %v", err)
	}
	entries, err := phasetiming.Read(workspace)
	if err != nil || len(entries) == 0 {
		t.Fatalf("phase timing: %v", err)
	}
	last := entries[len(entries)-1]
	codes := cyclestate.ErrorCodes(last.Diagnostics)
	if last.Phase != string(PhaseTDD) || last.Verdict != VerdictFAIL || len(codes) != 1 || codes[0] != cyclestate.DiagCodeProtectedPredicateRoute {
		t.Errorf("last timing entry = %+v codes=%v", last, codes)
	}
	if !strings.Contains(last.Diagnostics[0].Message, "agents/evolve-auditor.md") || !strings.Contains(last.Diagnostics[0].Message, "TestP") || last.Diagnostics[0].Subject != laneScopedItem {
		t.Errorf("the diagnostic names the file, the test function and the item: %+v", last.Diagnostics[0])
	}
}

func TestRunCycle_APredicateThatDoesNotReadAProvenProtectedPathRunsTheSpine(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"an unprotected path":              unprotectedPredicate,
		"a temp directory base":            tempBasePredicate,
		"dead code and a message":          deadCodePredicate,
		"a file that does not parse alone": "package cycle10\nfunc (",
	} {
		result, runners, _ := runTDDPredicateCycle(t, body, protectsAuditorPersona, oneCardDecision, laneEnv)
		if routed(result) || !buildWasDispatched(runners) {
			t.Errorf("%s: must not route and must run build: %+v", name, result)
		}
	}
}

func TestRunCycle_NoProofMeansNoRoute(t *testing.T) {
	t.Parallel()
	twoCards := `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`
	for name, run := range map[string]func() (CycleResult, map[Phase]PhaseRunner, string){
		"no membership wired": func() (CycleResult, map[Phase]PhaseRunner, string) {
			return runTDDPredicateCycle(t, joinedPersonaPredicate, nil, oneCardDecision, laneEnv)
		},
		"two committed items": func() (CycleResult, map[Phase]PhaseRunner, string) {
			return runTDDPredicateCycle(t, joinedPersonaPredicate, protectsAuditorPersona, twoCards, nil)
		},
	} {
		if result, _, _ := run(); routed(result) {
			t.Errorf("%s: %+v", name, result)
		}
	}
}

func TestRunCycle_ARouteThatCannotBeRecordedLeavesTheCycleOnItsNormalPath(t *testing.T) {
	t.Parallel()
	for name, tdd := range map[string]predicateWritingTDD{
		"the artifact cannot be written": {body: joinedPersonaPredicate, blockDir: committedset.ProtectedRouteFile},
		"the snapshot cannot be written": {body: joinedPersonaPredicate, blockFile: committedset.ProtectedPredicatesDir},
	} {
		result, runners, _, _ := runTDDPredicateCycleWT(t, tdd, protectsAuditorPersona, oneCardDecision, laneEnv)
		if routed(result) || !buildWasDispatched(runners) {
			t.Errorf("%s: an unrecorded route must not end the cycle: %+v", name, result)
		}
	}
}

func TestRunCycle_AScanFaultIsLoudButTheFindingsStillCount(t *testing.T) {
	t.Parallel()
	result, _, _, _ := runTDDPredicateCycleWT(t, predicateWritingTDD{body: joinedPersonaPredicate, extraFile: "package cycle10\nfunc ("}, protectsAuditorPersona, oneCardDecision, laneEnv)
	if !routed(result) {
		t.Errorf("a broken sibling file must not hide a proven need: %+v", result)
	}
}

func TestProtectedRouteMessage_NamesEachPathItsTestAndItsFile(t *testing.T) {
	t.Parallel()
	got := ProtectedRouteMessage([]acspathscan.Need{
		{Path: "agents/evolve-auditor.md", Test: "TestC1933_004", File: "predicates_test.go"},
		{Path: "go/internal/guards/role.go", Test: "TestC1933_005", File: "more_test.go"},
	})

	for _, want := range []string{
		"agents/evolve-auditor.md (read by TestC1933_004 in predicates_test.go)",
		"go/internal/guards/role.go (read by TestC1933_005 in more_test.go)",
		"a lane cannot edit it, so the item is console work",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ProtectedRouteMessage = %q, want it to contain %q", got, want)
		}
	}
}
