package main

// cmd_loop_chain_boundaryrefresh_fleetlane_test.go — RED tests (cycle 1364,
// inbox item auto-refresh-binary-at-boundary).
//
// Gap found by this cycle's scout (fleet_scope pinned to this one item):
// maybeRefreshChainBoundary (cmd_loop_chain.go, landed cycles 1314/1320/1323/
// 1325) already implements the ahead-check -> rebuild -> repin -> ledger ->
// re-exec sequence and is wired into BOTH the chain loop (cmd_loop_chain.go)
// and the plain sequential loop (cmd_loop.go ~L552) — the "sequential loop
// AND fleet mode" caller-proof obligation this very phase's own house rules
// require is already satisfied for those two paths. What is still MISSING is
// the one safety check a stranded, never-landed salvage commit
// (cycle-42824668-1360, e057d1b3, ".../cmd_loop_boot_refresh.go") added for
// the OLDER boot-time-only healer and this newer boundary healer never
// inherited: refusing the rebuild while a SIBLING fleet lane holds a live run.
// `grep -n "FleetLane\|fleet.*lane" cmd_loop_chain_boundaryrefresh*.go` has
// zero hits in this worktree today — chainRebuildFn currently runs
// unconditionally once chainBoundaryAheadFn reports stale, even if another
// lane is mid-batch on the SAME shared go/bin/evolve binary this rebuild
// overwrites. That is the exact scenario the standing rule "NEVER rebuild
// plane binary mid-batch (SELF_SHA)" (project memory
// stale_binary_false_fail) exists to prevent, and it is the AC4 the
// stranded salvage commit's design already solved for the boot-time path.
//
// This file does NOT re-implement the salvage commit's bespoke
// EvolveDir/runs/* scanner (bootRefreshFleetLaneFn / defaultBootRefreshFleetLane
// in the stranded commit) — that would duplicate logic gc.Discover
// (internal/gc/discover.go) already owns and adversarially hardens (L3.2):
// the SAME lease-aware run-dir scan the retention engine uses to decide a run
// is untouchable. Reusing it here means one fewer independent .lease reader
// to keep in sync (never_duplicate_centralize_via_design_patterns).
//
// Contract the Builder implements (TDD-defined seam):
//
//	// chainBoundaryFleetLaneFn is the test seam for "is a sibling fleet lane
//	// active" — checked AFTER chainBoundaryAheadFn confirms staleness and
//	// BEFORE chainBoundaryRefreshAlreadyAttempted/chainRebuildFn are ever
//	// reached, so an active sibling lane refuses the boundary heal before
//	// either rebuild or exec (mirrors the boot-time healer's own guard order
//	// in the stranded salvage design). An error from the check is UNVERIFIABLE
//	// safety state and must be treated exactly like laneActive=true — "cannot
//	// prove the plane is idle" refuses the rebuild — while still letting the
//	// chain continue on the current binary (fail-open for the CHAIN, fail-safe
//	// for the REBUILD; these are not the same axis).
//	var chainBoundaryFleetLaneFn = defaultChainBoundaryFleetLaneActive
//	func defaultChainBoundaryFleetLaneActive(cfg loopConfig) (active bool, err error)
//
// defaultChainBoundaryFleetLaneActive wraps gc.Discover(cfg.EvolveDir,
// gc.DiscoverOptions{}) and reports true iff any returned RunDir has
// Live==true. At the instant maybeRefreshChainBoundary runs (strictly
// BETWEEN this lane's own batches — the prior batch's run dir is already
// terminal, the next one has not been created yet), this lane's own history
// never surfaces as Live, so no self-exclusion by path is needed; a Live
// entry unambiguously means a DIFFERENT lane.
//
// maybeRefreshChainBoundary MUST call chainBoundaryFleetLaneFn(cfg)
// immediately after a positive chainBoundaryAheadFn result and log a
// stderr line containing "fleet lane" before returning refreshed=false,
// exactly like every other guard in this function (rebuild failure,
// ahead-check error) — auditable, never a silent no-op.
//
// RED now: chainBoundaryFleetLaneFn / defaultChainBoundaryFleetLaneActive are
// undefined -> this package's test build fails. Do NOT modify this file —
// implement the seam and wire the call site in cmd_loop_chain.go.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// brflRunsDir creates evolveDir/runs and returns its path.
func brflRunsDir(t *testing.T, evolveDir string) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// brflWriteRunMarker plants the run.json marker gc.Discover requires as
// evidence before it will even classify a directory as a run dir.
func brflWriteRunMarker(t *testing.T, runDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AC(fleetlane-1): an active sibling fleet lane (fresh .lease heartbeat under
// a DIFFERENT run dir) must refuse the boundary heal before either rebuild or
// re-exec, even though the local binary IS stale.
func TestMaybeRefreshChainBoundary_FleetLaneActiveRefusesRebuild(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	runsDir := brflRunsDir(t, evolveDir)
	siblingDir := filepath.Join(runsDir, "cycle-sibling-live")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, siblingDir)
	if err := runlease.Write(siblingDir, runlease.Lease{RunID: "cycle-sibling-live"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	restore := brfStubSeams(t, true, nil, nil) // stale=true (ahead)
	defer restore()

	rebuildCalled, reexecCalled := false, false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an active sibling fleet lane must never report refreshed=true")
	}
	if rebuildCalled || reexecCalled {
		t.Errorf("an active sibling fleet lane must refuse BEFORE rebuild/exec (rebuild=%v reexec=%v) — the standing rule is NEVER rebuild the plane binary mid-batch", rebuildCalled, reexecCalled)
	}
	if !strings.Contains(stderr.String(), "fleet lane") {
		t.Errorf("the refusal must be logged (auditable degrade, not a silent no-op), stderr=%q", stderr.String())
	}
}

// AC(fleetlane-2): an unverifiable fleet-lane check (scan error) is
// unverifiable SAFETY state, not proof of an idle plane — it must refuse the
// rebuild exactly like an active lane, while the CHAIN itself still degrades
// to "no refresh" rather than halting (fail-safe for the rebuild, fail-open
// for the chain — the same two-axis contract the existing ahead-check-error
// and rebuild-failure guards already use in this file's sibling tests).
func TestMaybeRefreshChainBoundary_FleetLaneCheckErrorRefusesRebuild(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil) // stale=true (ahead)
	defer restore()

	prevLane := chainBoundaryFleetLaneFn
	defer func() { chainBoundaryFleetLaneFn = prevLane }()
	chainBoundaryFleetLaneFn = func(loopConfig) (bool, error) {
		return false, errors.New("runlease: parse .evolve/runs/cycle-garbage/.lease: unexpected end of JSON input")
	}

	rebuildCalled, reexecCalled := false, false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an unverifiable fleet-lane state must never report refreshed=true")
	}
	if rebuildCalled || reexecCalled {
		t.Errorf("an unverifiable fleet-lane state must refuse BEFORE rebuild/exec (rebuild=%v reexec=%v) — cannot prove the plane is idle, so it must not rebuild it", rebuildCalled, reexecCalled)
	}
	if stderr.Len() == 0 {
		t.Error("an unverifiable fleet-lane state must be logged (auditable degrade), not swallowed silently")
	}
}

// AC(fleetlane-3, regression guard): with NO active sibling lane (the common
// case — an empty or absent runs/ dir), the boundary heal must proceed
// exactly as cmd_loop_chain_boundaryrefresh_test.go's own
// TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger already
// pins — this test exists only to prove the NEW fleet-lane guard does not
// regress that existing GREEN path once wired in.
func TestMaybeRefreshChainBoundary_NoFleetLaneActiveStillRefreshes(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	// runs/ deliberately left absent — gc.Discover on a missing dir returns an
	// empty list, not an error (see internal/gc/discover.go doc comment).

	restore := brfStubSeams(t, true, nil, nil) // stale=true (ahead)
	defer restore()

	// Same provenance/commit stubs TestMaybeRefreshChainBoundary_
	// LagTriggersRebuildRepinReExecAndLedger uses (cmd_loop_chain_
	// boundaryrefresh_test.go) — this regression guard proves the fleet-lane
	// guard doesn't block that existing GREEN path, so it must reach the SAME
	// downstream repin success that test's fixture already sets up; without
	// these two stubs the real defaultChainBoundaryRepinProvenance runs `git
	// -C <t.TempDir()> merge-base` against a non-repo dir and always refuses,
	// which is a fixture gap unrelated to the fleet-lane seam this file adds.
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() { chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv }()
	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	rebuildCalled := false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if !refreshed || !rebuildCalled {
		t.Fatalf("with no active sibling lane, the boundary heal must still proceed to rebuild; refreshed=%v rebuildCalled=%v stderr=%s", refreshed, rebuildCalled, stderr.String())
	}
}

// AC(fleetlane-4): defaultChainBoundaryFleetLaneActive itself — the
// production implementation — reusing gc.Discover, direct unit coverage
// (not routed through the maybeRefreshChainBoundary seam) so the real
// scanner, not just the fake, is exercised at least once.
func TestDefaultChainBoundaryFleetLaneActive_DetectsLiveSiblingDiscoveredByGC(t *testing.T) {
	evolveDir := t.TempDir()
	runsDir := brflRunsDir(t, evolveDir)
	siblingDir := filepath.Join(runsDir, "cycle-sibling-live")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brflWriteRunMarker(t, siblingDir)
	if err := runlease.Write(siblingDir, runlease.Lease{RunID: "cycle-sibling-live"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("a fresh sibling lease must not error: %v", err)
	}
	if !active {
		t.Fatal("a fresh sibling lease discovered by gc.Discover must report an active fleet lane")
	}
}

// AC(fleetlane-4b): the no-runs-dir baseline for the production function —
// a plane that has never recorded a run must report inactive, not an error.
func TestDefaultChainBoundaryFleetLaneActive_NoRunsDirIsInactive(t *testing.T) {
	active, err := defaultChainBoundaryFleetLaneActive(loopConfig{EvolveDir: t.TempDir()})
	if err != nil {
		t.Fatalf("no runs/ dir must not error: %v", err)
	}
	if active {
		t.Fatal("no runs/ dir must report no active fleet lane")
	}
}
