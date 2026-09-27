package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// spineGateOrch's fake runners write no handoff artifacts, so
// SpineSatisfiedUpTo(build) is false at the build transition: the
// missing-mandatory-predecessor condition the spine gate must catch.
func spineGateOrch(t *testing.T, recovery config.Stage, mandatory []string) (*Orchestrator, *fakeWorktree, *fakeStorage) {
	t.Helper()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.SpineFloor = recovery
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

	// The workspace path isn't known until RunCycle creates it, so the degraded
	// artifact is planted from the scout runner instead: handoff-scout.json as
	// a DIRECTORY → os.ReadFile fails EISDIR (≠ NotExist) → DigestDegraded.
	// insertedLeakRunner (shared, same package) gives onRun side-effects plus
	// an unconditional VerdictPASS so the cycle advances to the build gate.
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
	// The operator escape is cfg.Mandatory; dropping scout/build/audit here
	// pins the waiver mechanism only, not a recommended configuration.
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
