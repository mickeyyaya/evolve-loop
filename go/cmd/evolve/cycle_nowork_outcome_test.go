package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acspathscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func seedNoWorkLane(t *testing.T, root string, cycle int) string {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	ws := cycleWorkspace(root, cycle)
	for _, dir := range []string{inbox, ws} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(inbox, "2026-09-26T00-00-00Z-answered.json"): `{"id":"answered","title":"fixture","kind":"bug"}`,
		filepath.Join(ws, "lane-scope.json"):                       `{"todo_ids":["answered"],"goal_hash":"g"}`,
		filepath.Join(ws, "triage-decision.json"):                  `{"top_n":[],"dropped":[{"id":"answered","reason":"already-shipped-cycle-1679"}]}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return evolveDir
}

func plannedNoWork(cycle int) core.CycleResult {
	return core.CycleResult{
		Cycle:             cycle,
		FinalVerdict:      core.CycleOutcomeSkippedUnknown,
		TerminationReason: core.CycleTerminationTriageNoWork,
		PhasesRun:         []core.Phase{core.PhaseScout, core.PhaseTriage},
	}
}

func routeOf(t *testing.T, evolveDir, id string) string {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(filepath.Join(evolveDir, "inbox"), id)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	body, _ := os.ReadFile(path)
	var item struct {
		Route string `json:"route"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item.Route
}

func TestCloseoutCycleOutcome_PlannedNoWorkHandsTheLanesAnsweredItemToTheConsole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	fake := newFakeLedger()
	var stderr bytes.Buffer

	closeoutCycleOutcome(plannedNoWork(7), root, evolveDir, &stderr, fake, nil)

	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Fatalf("route = %q — the lane's answered item goes to the console, or the next wave draws it into another no-work lane", got)
	}
	if len(fake.lifecycle) == 0 {
		t.Error("the hand-off's lifecycle line goes through the root's ledger (ADR-0101 S4a)")
	}
}

func TestCloseoutCycleOutcome_OnlyAPlannedNoWorkHandsOver(t *testing.T) {
	t.Parallel()
	notNoWork := plannedNoWork(7)
	notNoWork.TerminationReason = "audit-advisory"
	for _, result := range []core.CycleResult{{Cycle: 7, FinalVerdict: core.VerdictPASS}, notNoWork} {
		root := t.TempDir()
		evolveDir := seedNoWorkLane(t, root, 7)
		closeoutCycleOutcome(result, root, evolveDir, io.Discard, newFakeLedger(), nil)
		if got := routeOf(t, evolveDir, "answered"); got != "" {
			t.Errorf("verdict %q / reason %q routed the scope to %q", result.FinalVerdict, result.TerminationReason, got)
		}
	}
}

func TestCloseoutCycleOutcome_ReportsWhatItApplied(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	if applied, err := closeoutCycleOutcome(plannedNoWork(7), root, evolveDir, io.Discard, newFakeLedger(), nil); applied != "no-work hand-off" || err != nil {
		t.Errorf("planned no-work: applied=%q err=%v", applied, err)
	}
	if applied, err := closeoutCycleOutcome(core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictPASS}, root, evolveDir, io.Discard, newFakeLedger(), nil); applied != "" || err != nil {
		t.Errorf("a PASS applies nothing: applied=%q err=%v", applied, err)
	}
}

func TestCompleteSequentialCycle_PlannedNoWorkHandsTheLaneOver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	state := &sequentialBatchState{maxConsecutiveFails: 5, stallCfg: goalStallConfig{threshold: 100, nonprogressThreshold: 100}}
	b.completeSequentialCycle(0, sequentialCycle{cycle: 7, workspace: cycleWorkspace(root, 7), result: plannedNoWork(7)}, state)
	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Fatalf("the sequential loop hands a planned-no-work lane's answered item over: route=%q stderr=%s", got, stderr.String())
	}
	if len(fake.lifecycle) == 0 {
		t.Error("through the root's ledger")
	}
}

func routedByProtectedPredicate(cycle int) core.CycleResult {
	return core.CycleResult{
		Cycle:             cycle,
		FinalVerdict:      core.VerdictFAIL,
		TerminationReason: core.CycleTerminationTddProtectedRoute,
		PhasesRun:         []core.Phase{core.PhaseScout, core.PhaseTriage, core.PhaseTDD},
	}
}

const keptPersonaPredicate = `package cycle7

func TestC1933_004(t *testing.T) {
	persona := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-auditor.md")
	acsassert.FileExists(t, persona)
}
`

func seedProtectedRouteLane(t *testing.T, root string, cycle int, predicate string) string {
	t.Helper()
	evolveDir := seedNoWorkLane(t, root, cycle)
	ws := cycleWorkspace(root, cycle)
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"answered"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteProtectedRoute(ws, committedset.ProtectedRoute{Cycle: cycle, ItemID: "answered"}); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(ws, committedset.ProtectedPredicatesDir)
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "predicates_test.go"), []byte(predicate), 0o644); err != nil {
		t.Fatal(err)
	}
	timing := `[{"phase":"tdd","verdict":"FAIL","diagnostics":[{"severity":"error","code":"PROTECTED_PREDICATE_ROUTE","subject":"answered","message":"m"}]}]`
	if err := os.WriteFile(phasetiming.Path(ws), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
	return evolveDir
}

func TestCloseoutCycleOutcome_AProvenProtectedPredicateRoutesTheItemButStaysAFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedProtectedRouteLane(t, root, 7, keptPersonaPredicate)
	result := routedByProtectedPredicate(7)

	applied, err := closeoutCycleOutcome(result, root, evolveDir, io.Discard, newFakeLedger(), nil)

	if applied != "failure outcome" || err != nil {
		t.Fatalf("applied=%q err=%v", applied, err)
	}
	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Errorf("route = %q", got)
	}
	if rc := cycleRunExitCode(result); rc != 2 {
		t.Errorf("exit code %d: the route must count as the FAIL it is, or a gamed predicate would hide a failure", rc)
	}
}

func TestCloseoutCycleOutcome_APathTheManifestDoesNotProtectIsNotRouted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedProtectedRouteLane(t, root, 7, "package p\nfunc TestX(t *testing.T) { os.ReadFile(\"go/internal/evalgate/materialization.go\") }\n")

	closeoutCycleOutcome(routedByProtectedPredicate(7), root, evolveDir, io.Discard, newFakeLedger(), nil)

	if got := routeOf(t, evolveDir, "answered"); got != "" {
		t.Errorf("route = %q: guards.IsProtectedSurface denies this path", got)
	}
}

func TestProtectedMembership_IsTheManifestAndTheCycle1933ReplayUsesIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(keptPersonaPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	needs, err := acspathscan.Needs(dir, protectedMembership)
	if err != nil || len(needs) != 1 || needs[0].Path != "agents/evolve-auditor.md" || needs[0].Test != "TestC1933_004" {
		t.Fatalf("needs = %+v err=%v", needs, err)
	}
	if !guards.IsProtectedSurface(needs[0].Path) {
		t.Error("the route's file must be proven by guards.IsProtectedSurface")
	}
}
