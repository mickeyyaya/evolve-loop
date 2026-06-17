package core

import (
	"context"
	"slices"
	"testing"
)

// TestCycleLoop_PostScoutHookFiresOncePreBuild pins the WS2-S0 hook call site
// (ADR-0052): the post-scout re-plan hook fires EXACTLY ONCE per cycle, after
// scout's handoff has been recorded (scout ∈ CompletedPhases) and BEFORE build
// (build ∉ CompletedPhases) — the pre-build ordering that lets a future re-plan
// run without contradicting a completed anchor. Uses the postScoutReplanProbe
// DI seam (the body is still a no-op this slice).
func TestCycleLoop_PostScoutHookFiresOncePreBuild(t *testing.T) {
	// NOT parallel: mutates the package-level probe seam.
	var fires int
	var completedAtFire [][]string
	prev := postScoutReplanProbe
	postScoutReplanProbe = func(cr *cycleRun) {
		fires++
		completedAtFire = append(completedAtFire, slices.Clone(cr.cs.CompletedPhases))
	}
	t.Cleanup(func() { postScoutReplanProbe = prev })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if fires != 1 {
		t.Fatalf("post-scout hook fired %d times, want exactly 1 (the cycle runs scout once)", fires)
	}
	cp := completedAtFire[0]
	if !slices.Contains(cp, "scout") {
		t.Errorf("hook fired before scout was recorded: completed=%v", cp)
	}
	if slices.Contains(cp, "build") {
		t.Errorf("hook fired post-build: completed=%v (must be pre-build)", cp)
	}
}
