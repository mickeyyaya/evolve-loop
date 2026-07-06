package main

// cmd_loop_wave_minwidth_wiring_test.go — cycle 552, task
// eliminate-sequential-fallback-min-width-lane (triage-report.md top_n, the
// single fleet-assigned id for this lane).
//
// GAP (triage-report.md Rationale): dispatchIteration and
// forceOneLaneDispatch (cmd_loop_wave.go, cycle-547) are both independently
// unit-tested as pure functions (cmd_loop_wave_test.go,
// cmd_loop_wave_minwidth_test.go), but nothing exercises the RunLoop
// call-site (cmd_loop.go's batch for-loop, ~lines 486-514) that wires them
// together: the `fleetCfg.Count > 1 && waveCfg.Count <= 1` guard that
// decides whether the min-width repair even applies, the one-lane launcher
// construction, and the WARN-vs-dispatch stderr branching that decides
// whether the batch iteration `continue`s (repaired) or falls through to
// the legacy sequential path. That wiring could silently regress — an
// inverted guard condition, or the whole call site deleted during an
// unrelated refactor — without any existing test catching it, because the
// call site itself was never extracted into a testable unit.
//
// FIX CONTRACT (this cycle's new surface — undefined until the Builder adds
// it, so this package's test build fails to compile today; that compile
// failure IS the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	minWidthRepair(ctx, fleetCfg, waveCfg, preflight, planFn, launcher,
//	waveIndex, stderr) (handled bool) — extracted from RunLoop's inline
//	switch (byte-identical stderr messages + control flow) so the guard
//	condition and WARN-vs-dispatch branching are independently testable
//	without a real fleet-lane subprocess:
//	  - guard not met (fleetCfg.Count<=1 or waveCfg.Count>1): WARNs "planned
//	    zero lanes (empty triage plan)", returns handled=false, NEVER calls
//	    preflight/planFn/launcher.
//	  - guard met, forceOneLaneDispatch dispatches a candidate: WARNs/logs
//	    "min-width repair dispatched N/M isolated lane (fleet.count=X shrank
//	    to Y)", returns handled=true (caller must `continue`).
//	  - guard met, forceOneLaneDispatch reports a genuinely empty backlog
//	    (ran=false, err=nil): WARNs "planned zero lanes (empty backlog)",
//	    returns handled=false (true sequential fallback — the case it stays
//	    reserved for).
//	  - guard met, forceOneLaneDispatch errors (preflight refusal or plan
//	    adapt failure): WARNs "min-width repair failed: <err>", returns
//	    handled=false — the error is surfaced, never silently swallowed.
//	RunLoop's call site becomes: on dispatchIteration's default (ran=false,
//	err=nil) case, construct the one-lane launcher and call minWidthRepair;
//	`continue` the batch iteration when handled is true.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative (the critical anti-gaming case): TestMinWidthRepair_GuardNotMetNeverInvokesLauncher
//     — a fleetCfg.Count<=1 (or waveCfg.Count>1) config must leave the
//     launcher UNTOUCHED; an inverted/loosened guard is the exact wiring
//     regression this task exists to catch.
//   - Positive: TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue
//   - Edge (empty backlog): TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential
//   - Edge (error surfaced, never swallowed): TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack
import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// wiringFakeLauncher records every Run call's specs — distinct name from the
// other fake launchers in this package's test files (each scoped to its own
// file to avoid cross-file coupling), per the minWidthFakeLauncher precedent.
type wiringFakeLauncher struct {
	calls [][]fleet.CycleSpec
}

func (f *wiringFakeLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	f.calls = append(f.calls, specs)
	results := make([]fleet.Result, len(specs))
	for i := range specs {
		results[i] = fleet.Result{Index: i, ExitCode: 0}
	}
	return results
}

// TestMinWidthRepair_GuardNotMetNeverInvokesLauncher (AC1: guard condition).
// The single most important regression this task exists to catch: an
// ineligible config (fleetCfg.Count<=1, so the repair must never apply —
// fleet.count=1 legacy behavior stays untouched) must leave preflight/planFn/
// launcher UNTOUCHED and report handled=false with the "empty triage plan"
// WARN, not the "empty backlog" one (they are distinct messages for distinct
// causes).
func TestMinWidthRepair_GuardNotMetNeverInvokesLauncher(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	preflightCalled, planFnCalled := false, false
	preflight := func() error { preflightCalled = true; return nil }
	planFn := func(context.Context, int) ([]byte, []string, error) {
		planFnCalled = true
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1},
		preflight, planFn, launcher, 0, &stderr)

	if handled {
		t.Fatal("minWidthRepair must report handled=false when fleetCfg.Count<=1 — the repair is not eligible at all")
	}
	if preflightCalled || planFnCalled {
		t.Error("an ineligible guard must never invoke preflight or planFn")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("an ineligible guard must never invoke the launcher; got %d calls", len(launcher.calls))
	}
	if !strings.Contains(stderr.String(), "empty triage plan") {
		t.Errorf("ineligible guard must WARN the empty-triage-plan message; got %q", stderr.String())
	}
}

// TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue
// (AC2: launcher construction + dispatch). When the guard IS met
// (fleetCfg.Count>1, waveCfg.Count<=1) and a candidate exists, the repair
// must dispatch exactly one isolated lane through the injected launcher and
// signal handled=true (the caller's `continue`).
func TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 1},
		func() error { return nil }, planFn, launcher, 2, &stderr)

	if !handled {
		t.Fatal("minWidthRepair must report handled=true when a candidate is dispatched — the caller must continue, not fall through to sequential")
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher.Run invoked %d times, want exactly 1 isolated lane", len(launcher.calls))
	}
	if len(launcher.calls[0]) != 1 {
		t.Errorf("dispatched %d specs, want 1 (capped to a single lane)", len(launcher.calls[0]))
	}
	if !strings.Contains(stderr.String(), "min-width repair dispatched") {
		t.Errorf("a successful repair must log the min-width-repair-dispatched message; got %q", stderr.String())
	}
}

// TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential (AC3:
// WARN-vs-dispatch branching, empty-backlog edge). The guard is met but the
// triage plan adapts to zero candidates — true sequential fallback is the
// ONLY case it stays reserved for; the launcher must never be invoked, and
// the WARN must name "empty backlog", not "empty triage plan" (the two
// distinct ineligibility/emptiness causes must not be conflated).
func TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
		func() error { return nil }, planFn, launcher, 0, &stderr)

	if handled {
		t.Fatal("an empty adapted backlog must report handled=false — true sequential fallback")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher invoked %d times for an empty adapted plan, want 0", len(launcher.calls))
	}
	if !strings.Contains(stderr.String(), "empty backlog") {
		t.Errorf("empty-backlog edge must WARN the empty-backlog message; got %q", stderr.String())
	}
}

// TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack (AC4: error path
// never silently swallowed). A preflight/plan-adapt error from
// forceOneLaneDispatch must be surfaced in the WARN (never dropped) and
// report handled=false so the caller falls back to sequential rather than
// silently losing the iteration.
func TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	refusal := errors.New("dirty control plane")
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
		func() error { return refusal }, planFn, launcher, 0, &stderr)

	if handled {
		t.Fatal("a preflight refusal must report handled=false")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher invoked despite a preflight refusal")
	}
	if !strings.Contains(stderr.String(), "min-width repair failed") || !strings.Contains(stderr.String(), refusal.Error()) {
		t.Errorf("the error must be surfaced in the WARN message, not swallowed; got %q", stderr.String())
	}
}
