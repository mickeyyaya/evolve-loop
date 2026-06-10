package core

// correction_ladder_test.go — ADR-0045 I2 (§8): the orchestrator-side ladder.
// White-box: reuses fakeStorage / fakeLedger / buildRunners /
// sequencedReviewer / recordingReviewer and the slice-1 ledger readers.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// fakeVerifier implements ContractVerifier over a configurable contracted
// destination (a SUBDIR of the workspace so a workspace-root stray is a
// legitimate candidate ≠ dest). destExisted records, per call, whether the
// destination existed AT CALL TIME — the relocate-FIRST-then-verify (TOCTOU,
// S2) order proof.
type fakeVerifier struct {
	mu          sync.Mutex
	destSub     string // contracted path relative to the workspace
	absDest     string // when set, the contracted path verbatim (S2 confinement tests)
	alwaysBad   bool   // verify !OK even when the destination exists
	destExisted []bool
}

func (f *fakeVerifier) VerifyDeliverable(_ context.Context, in ReviewInput) (ContractVerification, error) {
	dest := filepath.Join(in.Workspace, f.destSub)
	if f.absDest != "" {
		dest = f.absDest
	}
	_, statErr := os.Stat(dest)
	exists := statErr == nil
	f.mu.Lock()
	f.destExisted = append(f.destExisted, exists)
	f.mu.Unlock()
	v := ContractVerification{ArtifactPath: dest, OK: exists && !f.alwaysBad}
	if !v.OK {
		v.Violations = []string{"[missing_artifact] deliverable not found — write it to exactly: " + dest}
	}
	return v, nil
}

// ladderOrchestrator builds the RunCycle harness at a given recovery stage.
func ladderOrchestrator(rev DeliverableReviewer, fv ContractVerifier, stage config.Stage) (*Orchestrator, map[Phase]PhaseRunner) {
	cfg := config.RoutingConfig{}
	cfg.PhaseRecovery = stage
	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners,
		WithReviewer(rev), WithContractVerifier(fv), WithRouting(cfg, nil))
	return o, runners
}

func readLadderOutcomes(t *testing.T, ws string) []interaction.Outcome {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, "build-interactions.ndjson"))
	if err != nil {
		t.Fatalf("interaction ledger must exist: %v", err)
	}
	var outs []interaction.Outcome
	for _, ln := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if ln == "" {
			continue
		}
		var o interaction.Outcome
		if err := json.Unmarshal([]byte(ln), &o); err != nil {
			t.Fatalf("ledger line: %v", err)
		}
		outs = append(outs, o)
	}
	return outs
}

// strayWriterRunner delegates to fakeRunner and plants a stray deliverable at
// the LIVE workspace root on its first dispatch — modeling the cycle-265
// agent that wrote a valid report at the wrong path. (RunCycle provisions a
// fresh workspace, so pre-seeding from the test would be wiped.)
type strayWriterRunner struct {
	*fakeRunner
	strayName string
	planted   string // absolute path actually planted (read by assertions)
}

func (s *strayWriterRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if s.planted == "" {
		s.planted = filepath.Join(req.Workspace, s.strayName)
		if err := os.WriteFile(s.planted, []byte("# build report\nVerdict: PASS\n"), 0o644); err != nil {
			return PhaseResponse{}, err
		}
	}
	return s.fakeRunner.Run(ctx, req)
}

// plantStray swaps the build runner for a stray-planting one.
func plantStray(runners map[Phase]PhaseRunner) *strayWriterRunner {
	sw := &strayWriterRunner{fakeRunner: runners[PhaseBuild].(*fakeRunner), strayName: "build-report.md"}
	runners[PhaseBuild] = sw
	return sw
}

// TestSalvage_RelocatesThenVerifiesDESTINATION — the 265 replay: a
// misplaced-but-valid deliverable is salvaged in rung 1 with ZERO agent
// re-dispatches; the verifier saw the destination absent on the locate call
// and PRESENT on the verify-after-move call (relocate-first order, S2); the
// relocated artifact still went through the review gate (final outcome).
func TestSalvage_RelocatesThenVerifiesDESTINATION(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable not found at the contracted path"},
		{Approve: true},
	}}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
	o, runners := ladderOrchestrator(rev, fv, config.StageEnforce)
	buildR := plantStray(runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("salvage should repair the cycle: %v", err)
	}
	ws := cycleWorkspaceDir(root, res.Cycle)

	if buildR.calls != 1 {
		t.Errorf("build dispatched %d times, want 1 — salvage must avert the re-dispatch (the 265 proof)", buildR.calls)
	}
	if _, err := os.Stat(buildR.planted); !os.IsNotExist(err) {
		t.Errorf("the stray must have been MOVED, not copied (stat err=%v)", err)
	}
	dest := filepath.Join(ws, "deliverables", "build-report.md")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("the deliverable must land at the contracted path: %v", err)
	}
	if rev.calls != 2 {
		t.Errorf("reviewer called %d times, want 2 — salvage NEVER upgrades a verdict; the gate decides (final outcome)", rev.calls)
	}
	if len(fv.destExisted) < 2 || fv.destExisted[0] || !fv.destExisted[len(fv.destExisted)-1] {
		t.Errorf("verify order wrong (destExisted=%v): locate must see the dest absent, verify-after-move must read the DESTINATION", fv.destExisted)
	}

	outs := readLadderOutcomes(t, ws)
	var salv []interaction.Outcome
	for _, ou := range outs {
		if ou.Kind == interaction.KindSalvage {
			salv = append(salv, ou)
		}
	}
	if len(salv) != 1 {
		t.Fatalf("salvage outcomes = %d, want 1; ledger=%+v", len(salv), outs)
	}
	if salv[0].Result != interaction.ResultAccepted || salv[0].Rung != interaction.RungSalvage {
		t.Errorf("salvage outcome wrong: %+v", salv[0])
	}
	if salv[0].DecisionID == "" {
		t.Error("salvage must carry the correction DecisionID")
	}
}

// TestSalvage_InvalidDestinationFallsThrough — found-but-invalid: the
// relocation lands but the destination fails verification ⇒ the ladder falls
// through to re-dispatch, whose directive carries the kernel evidence digest
// including the misplaced file's original path (§8 evidence-enriched rung 3).
func TestSalvage_InvalidDestinationFallsThrough(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable not found at the contracted path"},
		{Approve: true},
	}}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md"), alwaysBad: true}
	o, runners := ladderOrchestrator(rev, fv, config.StageEnforce)
	buildR := plantStray(runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("redispatch should repair the cycle after salvage falls through: %v", err)
	}
	if buildR.calls != 2 {
		t.Fatalf("build dispatched %d times, want 2 (initial + 1 correction)", buildR.calls)
	}
	cd := buildR.requests[1].CorrectionDirective
	if !strings.Contains(cd, "REJECTED") {
		t.Errorf("directive must keep composeCorrection's rejection framing: %q", cd)
	}
	if !strings.Contains(cd, "Evidence") || !strings.Contains(cd, buildR.planted) {
		t.Errorf("enforce directive must carry the kernel evidence digest naming the found-but-invalid path %s; got %q", buildR.planted, cd)
	}

	ws := cycleWorkspaceDir(root, res.Cycle)
	outs := readLadderOutcomes(t, ws)
	var ids []string
	sawFoundInvalid := false
	for _, ou := range outs {
		if ou.Kind == interaction.KindSalvage && ou.Result == interaction.ResultFoundButInvalid {
			sawFoundInvalid = true
		}
		if ou.DecisionID != "" {
			ids = append(ids, ou.DecisionID)
		}
	}
	if !sawFoundInvalid {
		t.Errorf("salvage must record found_but_invalid honestly; ledger=%+v", outs)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Errorf("all rungs of one decision share a DecisionID; got %v", ids)
		}
	}
}

// TestLadder_ShadowLogsOnly_ByteIdenticalLegacy — shadow (the default):
// the stray stays where it is, the dispatch counts and directive are exactly
// today's, and the would-act soak signal lands in the ledger.
func TestLadder_ShadowLogsOnly_ByteIdenticalLegacy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable missing required header"},
		{Approve: true},
	}}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
	o, runners := ladderOrchestrator(rev, fv, config.StageShadow)
	buildR := plantStray(runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if buildR.calls != 2 {
		t.Errorf("build dispatched %d times, want 2 — shadow must keep today's correction loop", buildR.calls)
	}
	if _, err := os.Stat(buildR.planted); err != nil {
		t.Errorf("shadow must NOT relocate anything: %v", err)
	}
	cd := buildR.requests[1].CorrectionDirective
	if cd != composeCorrection("deliverable missing required header") {
		t.Errorf("shadow directive must be byte-identical to today's composeCorrection; got %q", cd)
	}
	outs := readLadderOutcomes(t, cycleWorkspaceDir(root, res.Cycle))
	sawWouldAct := false
	for _, ou := range outs {
		if ou.Kind == interaction.KindSalvage && ou.Result == interaction.ResultWouldAct {
			sawWouldAct = true
		}
	}
	if !sawWouldAct {
		t.Errorf("shadow must record the would-act soak signal (§10); ledger=%+v", outs)
	}
}

// TestLadder_BudgetsExhaust_CycleAbortsAsToday — nothing salvageable and the
// reviewer never approves ⇒ the ladder exhausts and aborts with EXACTLY
// today's error and dispatch counts.
func TestLadder_BudgetsExhaust_CycleAbortsAsToday(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
	o, runners := ladderOrchestrator(rev, fv, config.StageEnforce)
	buildR := runners[PhaseBuild].(*fakeRunner)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort after the ladder exhausts")
	}
	want := `review gate: phase "build" deliverable rejected after 2 correction(s): still malformed`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("abort error must carry today's EXACT inner message %q; got %v", want, err)
	}
	if buildR.calls != 3 {
		t.Errorf("build dispatched %d times, want 3 (initial + 2 corrections, as today)", buildR.calls)
	}
	outs := readLadderOutcomes(t, cycleWorkspaceDir(root, res.Cycle))
	sawNotFound := false
	for _, ou := range outs {
		if ou.Kind == interaction.KindSalvage && ou.Result == interaction.ResultNotFound {
			sawNotFound = true
		}
	}
	if !sawNotFound {
		t.Errorf("an enforce salvage attempt with nothing to relocate records not_found; ledger=%+v", outs)
	}
}

// TestSalvageHelper_RejectsUnsafeCandidates — the S2 candidate gates, helper
// level: symlinks, directories, oversize, stale, and candidate==dest are all
// refused; worktree precedes workspace in the search order.
func TestSalvageHelper_RejectsUnsafeCandidates(t *testing.T) {
	t.Parallel()
	newO := func(fv ContractVerifier) *Orchestrator {
		o, _ := ladderOrchestrator(&recordingReviewer{default_: ReviewResult{Approve: true}}, fv, config.StageEnforce)
		return o
	}
	ctx := context.Background()

	t.Run("symlink_refused", func(t *testing.T) {
		ws, wt := t.TempDir(), t.TempDir()
		target := filepath.Join(wt, "secret.md")
		if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(ws, "build-report.md")); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws})
		if sr.Relocated {
			t.Errorf("a symlink candidate must never be relocated: %+v", sr)
		}
	})

	t.Run("oversize_refused", func(t *testing.T) {
		ws := t.TempDir()
		big := make([]byte, salvageMaxBytes+1)
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), big, 0o644); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws})
		if sr.Relocated {
			t.Errorf("an oversize candidate must be refused: %+v", sr)
		}
	})

	t.Run("stale_refused", func(t *testing.T) {
		ws := t.TempDir()
		p := filepath.Join(ws, "build-report.md")
		if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-salvageMaxAge - time.Hour)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws})
		if sr.Relocated {
			t.Errorf("a stale candidate must be refused: %+v", sr)
		}
	})

	t.Run("candidate_at_dest_not_self_moved", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Contracted path IS the workspace root file: the only candidate
		// equals the destination — nothing to relocate.
		fv := &fakeVerifier{destSub: "build-report.md", alwaysBad: true}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws})
		if sr.Relocated {
			t.Errorf("candidate==dest must not count as a salvage: %+v", sr)
		}
	})

	t.Run("worktree_precedes_workspace", func(t *testing.T) {
		ws, wt := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(wt, "build-report.md"), []byte("FROM-WORKTREE"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("FROM-WORKSPACE"), 0o644); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws, Worktree: wt})
		if !sr.Relocated {
			t.Fatalf("expected a relocation: %+v", sr)
		}
		got, err := os.ReadFile(filepath.Join(ws, "deliverables", "build-report.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "FROM-WORKTREE" {
			t.Errorf("search order is worktree → workspace → cwd; relocated %q", got)
		}
	})

	t.Run("dest_outside_roots_refused", func(t *testing.T) {
		// S2 defense-in-depth: a verifier returning a contracted dest OUTSIDE
		// the workspace/.evolve roots must never make salvage relocate there,
		// even when a valid candidate exists in the workspace.
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "elsewhere", "build-report.md")
		fv := &fakeVerifier{absDest: outside}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: ws})
		if sr.Relocated {
			t.Errorf("an out-of-roots dest must be refused: %+v", sr)
		}
		if _, err := os.Stat(outside); !os.IsNotExist(err) {
			t.Errorf("nothing may be written to the out-of-roots dest (stat err=%v)", err)
		}
	})

	t.Run("evolve_dir_dest_allowed", func(t *testing.T) {
		// The guard must NOT break TargetEvolveDir contracts (router/advisor
		// JSON under <ProjectRoot>/.evolve): a dest there is legitimate.
		root := t.TempDir()
		ws := filepath.Join(root, "ws")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "routing-decision.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		evolveDest := filepath.Join(root, ".evolve", "routing-decision.json")
		fv := &fakeVerifier{absDest: evolveDest}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "router", Workspace: ws, ProjectRoot: root})
		if !sr.Relocated {
			t.Fatalf("a dest under <ProjectRoot>/.evolve must be allowed: %+v", sr)
		}
		if _, err := os.Stat(evolveDest); err != nil {
			t.Errorf("the deliverable must land under .evolve: %v", err)
		}
	})
}

// TestLadder_ZeroCorrections_SalvageFails_LegacyAbortMessage — the decision-id
// condition expansion enables the ladder when corrections are DISABLED
// (maxCorrections==0) but a verifier is wired. A failed salvage must then
// abort with the legacy zero-corrections message, byte-identical.
func TestLadder_ZeroCorrections_SalvageFails_LegacyAbortMessage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	fv := &fakeVerifier{destSub: filepath.Join("deliverables", "build-report.md")}
	cfg := config.RoutingConfig{}
	cfg.PhaseRecovery = config.StageEnforce
	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners,
		WithReviewer(rev), WithContractVerifier(fv), WithRouting(cfg, nil))
	buildR := runners[PhaseBuild].(*fakeRunner)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g",
		Env: map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "0"},
	})
	if err == nil {
		t.Fatal("expected the zero-corrections abort")
	}
	want := `review gate: phase "build" deliverable rejected: still malformed`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("zero-corrections abort must keep the legacy message %q; got %v", want, err)
	}
	if buildR.calls != 1 {
		t.Errorf("build dispatched %d times, want 1 (corrections disabled)", buildR.calls)
	}
}
