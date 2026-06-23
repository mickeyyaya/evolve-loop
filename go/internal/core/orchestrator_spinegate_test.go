// orchestrator_spinegate_test.go — R5 (concurrency-factory plan): the spine
// gate fails CLOSED at EVOLVE_PHASE_RECOVERY=enforce.
//
// Cycle-283: build proceeded despite a missing mandatory-predecessor handoff
// — "[orchestrator] WARN spine not satisfied for next=build ... proceeding
// fail-open". The fail-open rationale was that Digest cannot distinguish a
// transient READ MISS from a genuine ABSENCE; that distinction now exists
// (RoutingSignals.DigestDegraded), so a clean absence can block.
//
// Contract pinned here:
//  1. enforce + clean absence → the cycle ABORTS (FAILED-EXPLAINED: typed
//     error naming the gate; worktree preserved) instead of running a phase
//     whose mandatory predecessor never delivered.
//  2. shadow (default) → byte-compatible with today: WARN + proceed. The
//     block ships dormant until the R8.5 dial flip.
//  3. enforce + DEGRADED digest (handoff unreadable for a non-absence
//     reason) → fail-open WARN: a transient read error must never false-
//     block a real cycle.
//  4. The waiver is the EXISTING config escape (R5.3): an anchor removed
//     from cfg.Mandatory is not required — no new machinery.
package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// spineGateOrch builds an advisory-routing orchestrator whose fake runners
// write NO handoff artifacts — the cycle-283 shape: CompletedPhases says
// scout ran, but no handoff-scout.json exists, so SpineSatisfiedUpTo(build)
// is false at the build transition.
func spineGateOrch(t *testing.T, recovery config.Stage, mandatory []string) (*Orchestrator, *fakeWorktree, *fakeStorage) {
	t.Helper()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.PhaseRecovery = recovery
	if mandatory != nil {
		cfg.Mandatory = mandatory
	}
	st := &fakeStorage{}
	wt := &fakeWorktree{path: t.TempDir()}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}),
		WithWorktreeProvisioner(wt))
	return o, wt, st
}

func TestSpineGate_EnforceBlocksOnCleanAbsence(t *testing.T) {
	t.Parallel()
	o, wt, _ := spineGateOrch(t, config.StageEnforce, nil)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err == nil {
		t.Fatalf("RED (cycle-283): missing mandatory handoff at enforce did not block — cycle completed (phases=%v)", res.PhasesRun)
	}
	if !strings.Contains(err.Error(), "spine") {
		t.Errorf("abort must name the spine gate; got: %v", err)
	}
	// FAILED-EXPLAINED, work-preserving: the worktree must survive the abort.
	if len(wt.cleaned) != 0 {
		t.Errorf("worktree pruned on spine abort (cleaned=%v) — must be preserved for recovery", wt.cleaned)
	}
}

func TestSpineGate_ShadowKeepsFailOpen(t *testing.T) {
	t.Parallel()
	o, _, _ := spineGateOrch(t, config.StageShadow, nil)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("shadow must keep today's fail-open behavior (the block ships dormant): %v", err)
	}
	var sawShip bool
	for _, p := range res.PhasesRun {
		if p == PhaseShip {
			sawShip = true
		}
	}
	if !sawShip {
		t.Errorf("shadow cycle did not complete to ship (phases=%v)", res.PhasesRun)
	}
}

func TestSpineGate_EnforceFailsOpenOnDegradedDigest(t *testing.T) {
	t.Parallel()
	o, _, _ := spineGateOrch(t, config.StageEnforce, nil)

	// The workspace is created by RunCycle under ProjectRoot/.evolve/runs/…;
	// we cannot pre-plant the degraded artifact before knowing the path, so
	// plant it from the scout runner instead: handoff-scout.json as a
	// DIRECTORY → os.ReadFile fails EISDIR (≠ NotExist) → DigestDegraded.
	// insertedLeakRunner (shared from orchestrator_insertedleak_test.go,
	// same package) is relied on for two things: onRun side-effects AND an
	// unconditional VerdictPASS so the cycle advances to the build gate.
	o.runners[PhaseScout] = &insertedLeakRunner{name: string(PhaseScout), onRun: func(req PhaseRequest) {
		if err := os.MkdirAll(filepath.Join(req.Workspace, "handoff-scout.json"), 0o755); err != nil {
			t.Errorf("plant degraded handoff: %v", err)
		}
	}}

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("a DEGRADED digest (read error ≠ absence) must fail open even at enforce: %v (phases=%v)", err, res.PhasesRun)
	}
}

func TestSpineGate_ConfigWaiverSkipsAnchor(t *testing.T) {
	t.Parallel()
	// R5.3: the operator escape is the existing cfg.Mandatory set — with
	// scout and build removed, their missing handoffs cannot block. (Audit
	// must also produce no artifact here, so drop it too: this pins the
	// waiver MECHANISM, not a recommended configuration.)
	o, _, _ := spineGateOrch(t, config.StageEnforce, []string{"ship"})

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("config-waived anchors must not block (the R5.3 escape): %v (phases=%v)", err, res.PhasesRun)
	}
}

func TestDigest_DegradedVsAbsent(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	// Absent: no handoff file at all → Present:false, NOT degraded.
	sig, err := router.Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest(absent): %v", err)
	}
	if sig.Scout.Present {
		t.Error("absent handoff must not be Present")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("clean absence must not be degraded: %v", sig.DigestDegraded)
	}

	// Degraded: handoff path exists but is unreadable as a file (EISDIR).
	if err := os.MkdirAll(filepath.Join(ws, "handoff-build.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	sig, err = router.Digest(ws, []string{"build"})
	if err != nil {
		t.Fatalf("Digest(degraded): %v", err)
	}
	if sig.Build.Present {
		t.Error("unreadable handoff must not be Present")
	}
	if len(sig.DigestDegraded) == 0 {
		t.Error("RED: a non-absence read failure must mark the digest DEGRADED (the read-miss vs gap distinction the fail-open comment asked for)")
	}
}
