package core

import (
	"context"
	"path/filepath"
	"testing"
)

// N12 (ADR-0049): each concurrent fleet cycle must build with its OWN GOCACHE
// so concurrent `go build` invocations across cycles never race on a shared
// cache (golang/go#43052). The cache dir lives under the cycle's run workspace.
func TestPerCycleGOCACHE_AbsoluteUnderWorkspace(t *testing.T) {
	got, ok := perCycleGOCACHE("/abs/proj/.evolve/runs/cycle-5")
	if !ok {
		t.Fatalf("perCycleGOCACHE: ok=false, want true for a non-empty workspace")
	}
	want := "/abs/proj/.evolve/runs/cycle-5/.gocache"
	if got != want {
		t.Errorf("perCycleGOCACHE = %q, want %q", got, want)
	}
}

// go build HARD-fails on a relative GOCACHE ("GOCACHE is not an absolute
// path"). The default --project-root "." makes WorkspacePath relative, so the
// helper MUST resolve the cache dir to an absolute path.
func TestPerCycleGOCACHE_RelativeWorkspaceResolvedAbsolute(t *testing.T) {
	got, ok := perCycleGOCACHE(".evolve/runs/cycle-3")
	if !ok {
		t.Fatalf("perCycleGOCACHE: ok=false, want true")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("perCycleGOCACHE(%q) = %q, want an ABSOLUTE path (go build rejects a relative GOCACHE)", ".evolve/runs/cycle-3", got)
	}
	if filepath.Base(got) != ".gocache" {
		t.Errorf("perCycleGOCACHE = %q, want basename .gocache", got)
	}
}

// Worktree-less test cycles have no workspace; the helper signals "leave the
// inherited GOCACHE untouched" rather than inventing a path.
func TestPerCycleGOCACHE_EmptyWorkspaceSkips(t *testing.T) {
	if got, ok := perCycleGOCACHE(""); ok || got != "" {
		t.Errorf("perCycleGOCACHE(\"\") = (%q,%v), want (\"\",false)", got, ok)
	}
}

// Wiring: every phase in a cycle must inherit the per-cycle GOCACHE in its
// PhaseRequest.Env, so the go toolchain the agents invoke writes to the
// isolated cache and concurrent fleet cycles never collide on a shared one.
func TestOrchestrator_PerCycleGOCACHE_ReachesEveryPhase(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// cycle 1 (LastCycleNumber 0 -> +1); workspace = <root>/.evolve/runs/cycle-1.
	want := "/tmp/p/.evolve/runs/cycle-1/.gocache"
	for _, p := range []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip} {
		fr := runners[p].(*fakeRunner)
		if fr.calls == 0 {
			t.Errorf("phase %s never ran", p)
			continue
		}
		if got := fr.requests[0].Env["GOCACHE"]; got != want {
			t.Errorf("phase %s: req.Env[GOCACHE]=%q, want %q", p, got, want)
		}
	}
}
