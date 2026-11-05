package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func entryWorkspaces(t *testing.T) (CycleState, string) {
	t.Helper()
	ws, wt := t.TempDir(), t.TempDir()
	dir := filepath.Join(wt, "go", "acs", "cycle9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("package cycle9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: wt}, dir
}

func TestSealResumedBuildEntry_AnEntryOtherThanBuildSealsNothing(t *testing.T) {
	t.Parallel()
	cs, _ := entryWorkspaces(t)

	got, err := sealResumedBuildEntry(PhaseAudit, t.TempDir(), cs, 9, sealedPredicateBase)

	if err != nil || len(got.sources) != 0 || got.fault != "" {
		t.Fatalf("base = %+v err %v; only a Build entry holds a base", got, err)
	}
	if _, fault := readPredicateSnapshot(cs.WorkspacePath); fault == "" {
		t.Fatal("a non-Build entry wrote a snapshot")
	}
}

func TestSealResumedBuildEntry_ARebaseSplitOrInterruptedEntryKeepsTheLastSealedBase(t *testing.T) {
	t.Parallel()
	cs, dir := entryWorkspaces(t)
	if err := writePredicateSnapshot(cs.WorkspacePath, map[string]string{"p_test.go": "sealed earlier"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("edited since"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := sealResumedBuildEntry(PhaseBuild, t.TempDir(), cs, 9, keptPredicateBase)

	if err != nil || got.sources["p_test.go"] != "sealed earlier" || got.fault != "" {
		t.Fatalf("base = %+v err %v; the first entry of a resume keeps the base that the interrupted dispatch sealed", got, err)
	}
}

func TestSealResumedBuildEntry_AReentryAfterAuditRepairSealsTheTreeOfThatEntry(t *testing.T) {
	t.Parallel()
	cs, dir := entryWorkspaces(t)
	if err := writePredicateSnapshot(cs.WorkspacePath, map[string]string{"p_test.go": "sealed earlier"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("repaired"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := sealResumedBuildEntry(PhaseBuild, t.TempDir(), cs, 9, sealedPredicateBase)

	stored, fault := readPredicateSnapshot(cs.WorkspacePath)
	if err != nil || got.sources["p_test.go"] != "repaired" || stored["p_test.go"] != "repaired" || fault != "" {
		t.Fatalf("base = %+v stored = %v err %v; a later Build entry takes the tree of that entry as its base", got, stored, err)
	}
}

func TestSealResumedBuildEntry_AnUnsealableBaseNamesTheStep(t *testing.T) {
	t.Parallel()
	cs, _ := entryWorkspaces(t)
	if err := os.MkdirAll(filepath.Join(cs.WorkspacePath, predicateSnapshotFile, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := sealResumedBuildEntry(PhaseBuild, t.TempDir(), cs, 9, sealedPredicateBase)

	if err == nil || err.Error()[:len("resume seal predicate base")] != "resume seal predicate base" {
		t.Fatalf("err = %v; a failed seal names the predicate base", err)
	}
}

func TestSealResumedBuildEntry_AnActiveExplanationContractWithNoWorktreeFails(t *testing.T) {
	t.Parallel()
	cs := CycleState{CycleID: 9, WorkspacePath: t.TempDir(), ExplanationDocumentationVersion: 1}

	_, err := sealResumedBuildEntry(PhaseBuild, t.TempDir(), cs, 9, sealedPredicateBase)

	if err == nil || err.Error()[:len("resume seal Build explanation context")] != "resume seal Build explanation context" {
		t.Fatalf("err = %v; a failed explanation seal names its step", err)
	}
}

func TestResumedEndOutcome_NamesTheTypedEndsAndNothingElse(t *testing.T) {
	t.Parallel()
	end := blockerEnd{outcome: Unroutable, diag: Diagnostic{Code: DiagCodeBlockerUnroutable, Message: "no route"}}
	got, ok := resumedEndOutcome(end, PhaseBuild, PhaseResponse{}, 1, "t0")
	if !ok || got.Verdict != VerdictFAIL || got.Diagnostics[0].Code != DiagCodeBlockerUnroutable {
		t.Fatalf("typed end = %+v ok %v; want a FAIL that leads with its code", got, ok)
	}
	rejected := resumedRejection{err: errors.New("rejected after 2 correction(s)")}
	got, ok = resumedEndOutcome(rejected, PhaseBuild, PhaseResponse{}, 2, "t0")
	if !ok || got.AbortReason != "rejected after 2 correction(s)" || got.AttemptCount != 2 {
		t.Fatalf("rejection = %+v ok %v; want the rejection reason and attempt count", got, ok)
	}
	if _, ok = resumedEndOutcome(errors.New("other"), PhaseBuild, PhaseResponse{}, 1, "t0"); ok {
		t.Fatal("an untyped error must not close out as a typed end")
	}
}

type journeyReentryBuild struct {
	calls   *int
	defects []string
}

func (journeyReentryBuild) Name() string { return string(PhaseBuild) }
func (b journeyReentryBuild) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if *b.calls == 0 {
		*b.calls = 1
		return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
	}
	return claimingBuild{defects: b.defects, dispatches: b.calls}.Run(ctx, req)
}

type journeyReentryReviewer struct {
	calls    *int
	failures []string
}

func (r journeyReentryReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) || *r.calls <= 1 {
		return ReviewResult{Approve: true}
	}
	return ReviewResult{Approve: false, Retry: true, Reason: "floor", Failures: r.failures}
}

type journeyRepairAudit struct{ file string }

func (journeyRepairAudit) Name() string { return string(PhaseAudit) }
func (a journeyRepairAudit) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(a.file, []byte("package cycle9\n\n// repaired by the re-entry\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return gateForcedAuditRunner{}.Run(ctx, req)
}

func TestResumeJourney_AReentryAfterAuditRepairSealsItsOwnTreeThroughTheLoop(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	repo := gittest.Fixture(t)
	writeBlockerFile(t, repo.Dir, selfcheckPath, "package core\n")
	writeBlockerFile(t, repo.Dir, breakerPath, "package core\n")
	pred := filepath.Join(repo.Dir, "go", "acs", "cycle9", "predicates_test.go")
	writeBlockerFile(t, repo.Dir, "go/acs/cycle9/predicates_test.go", strings.ReplaceAll(cycle1950Predicates, "cycle1950", "cycle9"))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(oneCardDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	sealedBase(t, ws, repo.Dir)
	calls := 0
	runners := buildRunners(nil)
	runners[PhaseBuild] = journeyReentryBuild{calls: &calls, defects: []string{cycle1946Claim}}
	runners[PhaseAudit] = journeyRepairAudit{file: pred}
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: repo.Dir}}
	var stderr strings.Builder
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: repo.Dir}), WithReviewer(journeyReentryReviewer{calls: &calls, failures: []string{cycle1946PredicateFailure}}), WithProtectedSurface(protectsSelfcheck), WithStderr(&stderr))
	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "lane-goal", Env: laneEnv}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9})
	if err != nil || !IsProvenBlockerResult(res) || calls != 2 {
		t.Fatalf("err=%v term=%q builds=%d stderr=%s", err, res.TerminationReason, calls, stderr.String())
	}
}

type journeyRebaseBuild struct {
	calls   *int
	defects []string
}

func (journeyRebaseBuild) Name() string { return string(PhaseBuild) }
func (b journeyRebaseBuild) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if _, err := (&requiredExplanationBuilder{}).Run(ctx, req); err != nil {
		return PhaseResponse{}, err
	}
	return claimingBuild{defects: b.defects, dispatches: b.calls}.Run(ctx, req)
}

func TestResumeJourney_ARebaseSplitKeepsTheLastSealedBaseThroughTheLoop(t *testing.T) {
	t.Parallel()
	root, ws, newBase := rebasedSplitFixture(t)
	writeBlockerFile(t, root, "go/acs/cycle42/predicates_test.go", "package cycle42\n\n// edited before the split\n")
	if err := writePredicateSnapshot(ws, map[string]string{"predicates_test.go": "package cycle42\n"}); err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{state: State{LastCycleNumber: 42}, cycleState: CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: ws, ActiveWorktree: root,
		WorktreeBaseSHA: newBase, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	writeStateFile(t, filepath.Join(root, ".evolve"), storage.cycleState)
	calls := 0
	runners := buildRunners(nil)
	runners[PhaseBuild] = journeyRebaseBuild{calls: &calls, defects: []string{cycle1946Claim}}
	var stderr strings.Builder
	o := NewOrchestrator(storage, &fakeLedger{}, runners, WithReviewer(floorRejecting{failures: []string{cycle1946PredicateFailure}}), WithProtectedSurface(protectsSelfcheck), WithStderr(&stderr))
	_, _ = o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{
		CycleID: 42, Phase: string(PhaseAudit), WorktreePath: root,
		StatePath: filepath.Join(root, ".evolve", CycleStateFile),
	})
	if !strings.Contains(stderr.String(), "changed the predicates of the cycle") || !strings.Contains(stderr.String(), "go/acs/cycle42/predicates_test.go") || calls == 0 {
		t.Fatalf("calls=%d stderr=%s", calls, stderr.String())
	}
}
