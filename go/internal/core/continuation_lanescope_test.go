package core

// continuation_lanescope_test.go — ADR-0076 slice C, G2 (cycle-1104). G1 binds
// preserved work to an INBOX CLAIM. A lane whose scope came from the wave
// planner has no claim file, so cycle-1078's FAIL had nothing to stamp and its
// snapshot was orphaned. G2 adds the second scope-identity class: the preserve
// decision ALSO registers the manifest under the lane's todo ids (the
// authoritative <workspace>/lane-scope.json pin), and adoption resolves claims
// first, lane scope second.
//
// Everything here rides the EXISTING gate: only work the carry-forward screen
// classifies Clean is registered, and every registry failure is a WARN that
// never fails finalization.

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

// writeLaneScope pins todo ids into a workspace exactly as materializeLaneScope
// does, so the produce side reads a REAL lane-scope.json (not a test-only shape).
func writeLaneScope(t *testing.T, workspace string, ids ...string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(LaneScope{TodoIDs: ids, GoalHash: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, LaneScopeFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestStampContinuationManifest_RegistersLaneScopeBinding — the produce-side
// headline AC: a FAIL whose scope exists ONLY as a lane-scope pin (no
// processing claim anywhere) still leaves a resolvable binding behind, keyed by
// the lane's todo id and carrying the same snapshot the workspace manifest does.
func TestStampContinuationManifest_RegistersLaneScopeBinding(t *testing.T) {
	root, wt := initContinuationRepo(t, 1078)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1078")
	writeLaneScope(t, ws, "chain-boundary-loop")
	if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, wt, "rev-parse", "HEAD")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 1078, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}

	o.stampContinuationManifest(context.Background(), cs, 1078, root)

	m, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("manifest must still be written: ok=%v err=%v", ok, err)
	}
	entry, ok, err := continuation.ReadRegistryEntry(root, "chain-boundary-loop")
	if err != nil || !ok {
		t.Fatalf("a lane-scope-only FAIL must register its binding: ok=%v err=%v", ok, err)
	}
	if entry.SnapshotSHA != m.SnapshotSHA || entry.Branch != m.Branch || entry.Cycle != 1078 {
		t.Errorf("registry entry must mirror the manifest:\n entry %+v\n manifest %+v", entry, m)
	}
	if entry.BaseSHA != base {
		t.Errorf("registry entry base = %q, want the attempt's base %q", entry.BaseSHA, base)
	}
	// End-to-end with the real resolver: a LATER cycle carrying the same lane
	// scope and no claim of its own resolves the binding.
	got := inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root}, 1102, []string{"chain-boundary-loop"})
	if got == nil || got.SnapshotSHA != m.SnapshotSHA {
		t.Errorf("later cycle must resolve the lane-scope binding, got %+v", got)
	}
}

// TestStampContinuationManifest_RegistersEveryLaneScopeID — a multi-id lane
// registers each of its todo ids, so whichever id the next attempt is scoped to
// finds the work.
func TestStampContinuationManifest_RegistersEveryLaneScopeID(t *testing.T) {
	root, wt := initContinuationRepo(t, 1079)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1079")
	writeLaneScope(t, ws, "scope-a", "scope-b")
	if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 1079, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}

	o.stampContinuationManifest(context.Background(), cs, 1079, root)

	for _, id := range []string{"scope-a", "scope-b"} {
		if _, ok, err := continuation.ReadRegistryEntry(root, id); err != nil || !ok {
			t.Errorf("todo id %q must carry a binding: ok=%v err=%v", id, ok, err)
		}
	}
}

// TestStampContinuationManifest_NoLaneScopeRegistersNothing — NEGATIVE. A
// sequential (non-lane) cycle has no lane-scope.json: the manifest is written
// exactly as before and NO registry is created. Proves the extension is
// additive, not a blanket "always register" that would bind work to phantom
// scope ids.
func TestStampContinuationManifest_NoLaneScopeRegistersNothing(t *testing.T) {
	root, wt := initContinuationRepo(t, 1080)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1080")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 1080, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}

	o.stampContinuationManifest(context.Background(), cs, 1080, root)

	if _, ok, err := continuation.ReadManifest(ws); err != nil || !ok {
		t.Fatalf("manifest must be written unchanged without a lane scope: ok=%v err=%v", ok, err)
	}
	if _, err := os.Stat(continuation.RegistryPath(root)); err == nil {
		t.Error("a cycle with no lane scope must not create a registry (phantom binding)")
	}

	// An EMPTY / blank-id lane scope is the same no-op — materializeLaneScope
	// splits an empty env scope into [""], which must never become a key.
	ws2 := filepath.Join(root, ".evolve", "runs", "cycle-1081")
	writeLaneScope(t, ws2, "")
	cs2 := CycleState{CycleID: 1081, WorkspacePath: ws2, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}
	o.stampContinuationManifest(context.Background(), cs2, 1081, root)
	if _, ok, _ := continuation.ReadRegistryEntry(root, ""); ok {
		t.Error("a blank lane-scope id must never become a registry key")
	}
}

// TestStampContinuationManifest_UnstampableWorkRegistersNothing — NEGATIVE and
// the anti-gaming half: registration rides the SAME Clean gate as the manifest.
// Work that conflicts with main is not resumable, so it must leave no binding —
// otherwise every later attempt on that scope would adopt and re-reject it.
func TestStampContinuationManifest_UnstampableWorkRegistersNothing(t *testing.T) {
	root, wt := initContinuationRepo(t, 1082)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1082")
	writeLaneScope(t, ws, "scope-conflict")
	base := gitOut(t, wt, "rev-parse", "HEAD")
	// Same path edited on BOTH sides ⇒ the carry-forward screen classifies the
	// snapshot as conflicting, not Clean.
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("worktree side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("main side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "a.txt")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "main moves")

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 1082, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}

	o.stampContinuationManifest(context.Background(), cs, 1082, root)

	if _, ok, _ := continuation.ReadManifest(ws); ok {
		t.Fatal("fixture invalid: conflicting work must not stamp a manifest")
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "scope-conflict"); ok {
		t.Error("conflicting work must leave NO lane-scope binding (it is not resumable)")
	}
}

// laneScopeTriageRunner mimics the production triage phase for a NON-CLAIM
// lane: it claims nothing and simply pins the lane identity into the cycle
// workspace, the same file materializeLaneScope writes before phases run.
type laneScopeTriageRunner struct {
	*fakeRunner
	todoIDs []string
}

func (r *laneScopeTriageRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	body, _ := json.Marshal(LaneScope{TodoIDs: r.todoIDs, GoalHash: "g"})
	if err := os.WriteFile(filepath.Join(req.Workspace, LaneScopeFile), body, 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return r.fakeRunner.Run(ctx, req)
}

// productionScopeResolver mirrors the composition root's closure (cmd_cycle.go)
// after G2: the REAL claim-then-lane-scope resolver. The composed-path proof.
func productionScopeResolver(t *testing.T) func(string, int, []string) *continuation.Continuation {
	t.Helper()
	return func(root string, cycle int, scopeIDs []string) *continuation.Continuation {
		return inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root}, cycle, scopeIDs)
	}
}

// TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim — the full
// cycle-1078 story, end to end and through the production resolver: a FAILed
// lane-scope-only attempt registers its snapshot; a later cycle scoped to the
// same todo id — with NO inbox item and NO processing claim in the repo at all —
// re-seeds its worktree from that snapshot and builds on the preserved work.
func TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim(t *testing.T) {
	root, wt := initContinuationRepo(t, 1083)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1083")
	writeLaneScope(t, ws, "lane-only-scope")
	if err := os.WriteFile(filepath.Join(wt, "prior_work.go"), []byte("package prior\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o0 := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 1083, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}
	o0.stampContinuationManifest(context.Background(), cs, 1083, root)
	m, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("fixture stamp failed: ok=%v err=%v", ok, err)
	}
	if err := os.WriteFile(m.FindingsPath, []byte(`{"phase":"build","summary":"lane-scope orphan digest"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	probe := &worktreeProbeRunner{fakeRunner: buildR, probeFile: "prior_work.go"}
	runners[PhaseBuild] = probe
	runners[PhaseTriage] = &laneScopeTriageRunner{
		fakeRunner: runners[PhaseTriage].(*fakeRunner),
		todoIDs:    []string{"lane-only-scope"},
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(productionScopeResolver(t)))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	// No claim was ever created — this is the non-claim path by construction.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing")); err == nil {
		t.Fatal("fixture invalid: this scenario must have NO processing claims at all")
	}
	req := buildR.requests[0]
	if req.Worktree == "" || req.Worktree == wt {
		t.Fatalf("build must run in a NEW seeded worktree, got %q (old %q)", req.Worktree, wt)
	}
	if !probe.sawFile {
		t.Error("preserved work must be present in the adopted worktree at build time (cycle-1078 orphan class)")
	}
	if !strings.Contains(req.Context["continuation_findings"], "lane-scope orphan digest") {
		t.Errorf("build context must carry the prior attempt's findings; got %q", req.Context["continuation_findings"])
	}
}

// TestRunCycle_UnrelatedLaneScopeDoesNotAdopt — NEGATIVE and the cross-lane
// safety proof: a cycle scoped to a DIFFERENT todo id must not inherit another
// lane's preserved work, even though a binding exists in the same repo.
func TestRunCycle_UnrelatedLaneScopeDoesNotAdopt(t *testing.T) {
	root, wt := initContinuationRepo(t, 1084)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1084")
	writeLaneScope(t, ws, "lane-A")
	if err := os.WriteFile(filepath.Join(wt, "prior_work.go"), []byte("package prior\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o0 := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o0.stampContinuationManifest(context.Background(),
		CycleState{CycleID: 1084, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")},
		1084, root)
	if _, ok, _ := continuation.ReadRegistryEntry(root, "lane-A"); !ok {
		t.Fatal("fixture invalid: lane-A binding must exist")
	}

	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	probe := &worktreeProbeRunner{fakeRunner: buildR, probeFile: "prior_work.go"}
	runners[PhaseBuild] = probe
	runners[PhaseTriage] = &laneScopeTriageRunner{
		fakeRunner: runners[PhaseTriage].(*fakeRunner),
		todoIDs:    []string{"lane-B"},
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(productionScopeResolver(t)))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if probe.sawFile {
		t.Error("lane-B must NOT adopt lane-A's preserved work (cross-lane contamination)")
	}
}

// TestAdoptContinuation_PassesLaneScopeIDsToResolver — the seam contract: the
// orchestrator hands the resolver THIS cycle's pinned lane-scope todo ids, read
// from the workspace pin at adoption time (after triage, when the pin and any
// claims are on disk). A resolver that never receives the ids cannot implement
// the fallback at all.
func TestAdoptContinuation_PassesLaneScopeIDsToResolver(t *testing.T) {
	root, _ := initContinuationRepo(t, 1085)
	var gotScopes [][]string
	runners := buildRunners(nil)
	runners[PhaseTriage] = &laneScopeTriageRunner{
		fakeRunner: runners[PhaseTriage].(*fakeRunner),
		todoIDs:    []string{"scope-x", "scope-y"},
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithContinuationResolver(func(_ string, _ int, scopeIDs []string) *continuation.Continuation {
			gotScopes = append(gotScopes, append([]string(nil), scopeIDs...))
			return nil
		}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(gotScopes) == 0 {
		t.Fatal("the continuation resolver must be consulted")
	}
	want := []string{"scope-x", "scope-y"}
	last := gotScopes[len(gotScopes)-1]
	if strings.Join(last, ",") != strings.Join(want, ",") {
		t.Errorf("resolver received scope ids %v, want the pinned lane scope %v", last, want)
	}
}
