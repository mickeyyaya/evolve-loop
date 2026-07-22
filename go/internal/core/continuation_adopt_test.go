package core

// continuation_adopt_test.go — ADR-0076 slice C, consume side. A new cycle
// whose scoped item carries a valid continuation provisions its worktree FROM
// the snapshot commit (standard provisioning path — work inherits via git
// history, never via dirty-state adoption), sets the review base to the
// ORIGINAL base so cumulative work is reviewed whole, and serves the prior
// attempt's findings to the build phase. Any validation failure falls back to
// fresh provisioning, loudly.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// stampedContinuation runs the produce side for real over a repo fixture:
// dirty worktree for oldCycle → snapshot + manifest → returns the manifest.
func stampedContinuation(t *testing.T, root, wt string, oldCycle int) continuation.Continuation {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", cycleDirName(oldCycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "prior_work.go"), []byte("package prior\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, wt, "rev-parse", "HEAD")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: oldCycle, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}
	o.stampContinuationManifest(context.Background(), cs, oldCycle, root)
	m, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("fixture stamp failed: ok=%v err=%v", ok, err)
	}
	return m
}

func cycleDirName(cycle int) string {
	return "cycle-" + itoa(cycle)
}

func TestGitWorktree_CreateFromSeedsSnapshot(t *testing.T) {
	root, wt := initContinuationRepo(t, 81)
	m := stampedContinuation(t, root, wt, 81)

	wt2, err := gitWorktree{}.CreateFrom(root, 85, m.SnapshotSHA)
	if err != nil {
		t.Fatalf("CreateFrom: %v", err)
	}
	if got := gitOut(t, wt2, "rev-parse", "HEAD"); got != m.SnapshotSHA {
		t.Errorf("new worktree HEAD = %s, want snapshot %s", got, m.SnapshotSHA)
	}
	if _, err := os.Stat(filepath.Join(wt2, "prior_work.go")); err != nil {
		t.Errorf("prior work must be present in the seeded worktree: %v", err)
	}
}

func TestValidateContinuation_CleanPassesConflictAndGarbageFail(t *testing.T) {
	root, wt := initContinuationRepo(t, 82)
	m := stampedContinuation(t, root, wt, 82)

	if err := validateContinuation(context.Background(), root, &m); err != nil {
		t.Errorf("clean continuation must validate: %v", err)
	}

	garbage := m
	garbage.SnapshotSHA = "0000000000000000000000000000000000000000"
	if err := validateContinuation(context.Background(), root, &garbage); err == nil {
		t.Error("missing snapshot commit must fail validation")
	}

	// main moves and now conflicts with the snapshot → re-screen must reject.
	if err := os.WriteFile(filepath.Join(root, "prior_work.go"), []byte("package mainside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "prior_work.go")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "conflicting")
	if err := validateContinuation(context.Background(), root, &m); err == nil {
		t.Error("a snapshot that now conflicts with main must fail the adopt-time re-screen")
	}
}

func TestRunCycle_AdoptsContinuationAndServesFindings(t *testing.T) {
	root, wt := initContinuationRepo(t, 83)
	m := stampedContinuation(t, root, wt, 83)
	if err := os.WriteFile(m.FindingsPath, []byte(`{"phase":"build","summary":"export X unnamed in tests"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// PRODUCTION flow: the FAILed cycle's release stamps the item at the inbox
	// ROOT; the NEXT cycle's triage claims it into processing/cycle-N
	// mid-cycle; only then does the resolver see it (architect finding #1).
	seedStampedInboxItem(t, root, 83, "task-a")

	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	probe := &worktreeProbeRunner{fakeRunner: buildR, probeFile: "prior_work.go"}
	runners[PhaseBuild] = probe
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(productionResolver(t)))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	req := buildR.requests[0]
	if req.Worktree == "" || req.Worktree == wt {
		t.Fatalf("build must run in a NEW seeded worktree, got %q (old %q)", req.Worktree, wt)
	}
	if !probe.sawFile {
		t.Error("prior work must be intact in the adopted worktree at build time")
	}
	if !strings.Contains(req.Context["continuation_findings"], "export X unnamed") {
		t.Errorf("build context must carry prior findings; got %q", req.Context["continuation_findings"])
	}
	// Ship-manifest breadcrumb: the adopting cycle's OWN workspace carries the
	// manifest copy (reconcileManifest unions the prior declared paths).
	if _, ok, _ := continuation.ReadManifest(req.Workspace); !ok {
		t.Error("adopting cycle's workspace must carry the continuation manifest copy")
	}
}

func TestRunCycle_InvalidContinuationFallsBackFresh(t *testing.T) {
	root, wt := initContinuationRepo(t, 84)
	m := stampedContinuation(t, root, wt, 84)
	// Corrupt the stamp BEFORE it reaches the inbox item: point the item at a
	// snapshot that does not exist.
	m.SnapshotSHA = "1111111111111111111111111111111111111111"
	seedStampedItemDirect(t, root, m, "task-a")

	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	probe := &worktreeProbeRunner{fakeRunner: buildR, probeFile: "prior_work.go"}
	runners[PhaseBuild] = probe
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(productionResolver(t)))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle must not fail on a stale continuation: %v", err)
	}
	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	if probe.sawFile {
		t.Error("stale continuation must NOT seed prior work (fresh worktree kept)")
	}
	if buildR.requests[0].Context["continuation_findings"] != "" {
		t.Error("fresh fallback must not serve findings")
	}
}

// productionResolver mirrors the composition root's closure (cmd_cycle.go)
// byte-for-byte: the REAL inboxmover.ResolveContinuation over processing
// claims — the composed-path proof the mock resolver could not give.
func productionResolver(t *testing.T) func(string, int) *continuation.Continuation {
	t.Helper()
	return func(root string, cycle int) *continuation.Continuation {
		return inboxmover.ResolveContinuation(inboxmover.Options{ProjectRoot: root}, cycle)
	}
}

// seedStampedInboxItem exercises the PRODUCTION stamp path: an item claimed by
// the FAILed cycle is released through releaseCycleProcessing, which stamps it
// from the cycle's continuation manifest and lands it at the inbox root.
func seedStampedInboxItem(t *testing.T, root string, failedCycle int, taskID string) {
	t.Helper()
	procDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+itoa(failedCycle))
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, taskID+".json"), []byte(`{"id":"`+taskID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inboxmover.ReleaseCycleProcessingWithReason(inboxmover.Options{ProjectRoot: root}, failedCycle, "cycle-failure-release"); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// seedStampedItemDirect writes a stamped item straight to the inbox root (for
// corrupt-stamp scenarios the production release would never produce).
func seedStampedItemDirect(t *testing.T, root string, m continuation.Continuation, taskID string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"id": taskID, "continuation": m})
	if err := os.WriteFile(filepath.Join(inbox, taskID+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// claimingTriageRunner mimics the production triage phase's side effect: it
// CLAIMS its selected item from the inbox root into processing/cycle-N via the
// real inboxmover.Claim, mid-cycle — the ordering the adoption seam is keyed to.
type claimingTriageRunner struct {
	*fakeRunner
	root   string
	taskID string
}

func (r *claimingTriageRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: r.root}, r.taskID, itoa(req.Cycle)); err != nil {
		return PhaseResponse{}, err
	}
	return r.fakeRunner.Run(ctx, req)
}

// worktreeProbeRunner records whether probeFile existed in the dispatched
// worktree at phase-run time (the worktree is pruned on normal completion, so
// post-RunCycle stats race the cleanup).
type worktreeProbeRunner struct {
	*fakeRunner
	probeFile string
	sawFile   bool
}

func (r *worktreeProbeRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if req.Worktree != "" {
		if _, err := os.Stat(filepath.Join(req.Worktree, r.probeFile)); err == nil {
			r.sawFile = true
		}
	}
	return r.fakeRunner.Run(ctx, req)
}
