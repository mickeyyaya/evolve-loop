//go:build acs

// Package cycle1182 materialises the cycle-1182 acceptance criteria for the sole
// triage-committed top_n task of this fleet lane: `wave-planner-pass-scope-prune`.
//
// The defect. `triagecap.pruneConsumed` drops committed ids the inbox lifecycle
// has already consumed, but it is wired into the FRESH-SEED path only
// (SelectWaveSeedMenus). The PRIMARY per-wave path — `widenNarrowDecision` in
// package main (go/cmd/evolve/cmd_loop_wave.go), which runs whenever a prior
// cycle's triage-decision.json exists — builds its committed prefix straight from
// decision.top_n and either short-circuits on `len(committed) >= count` or hands
// the list to WidenTopNToFleetWidth, which copies it through VERBATIM. Neither
// branch consults the lifecycle, so a consumed id survives in the prior decision
// file and is re-pinned into the next wave's lane-scope.json (cycle-1116
// re-pinned tdd-topn-binding-gate after cycle-1113 consumed it).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-1098 precedent). The
// subject `widenNarrowDecision` lives in `package main`, which cannot be
// imported, so each predicate shells `go test -run` over the RED contract tests
// authored this cycle (go/cmd/evolve/cmd_loop_wave_prune_test.go and
// go/internal/triagecap/lane_menu_prune_export_test.go). Every one of those
// CALLS the system under test — widenNarrowDecision over a real temp-dir inbox
// lifecycle, and the planner end-to-end via fleet.PlanFromTriage — and asserts on
// the returned decision bytes / lane scopes. None is a source-grep of production
// code (the cycle-85 degenerate-predicate ban).
//
// RED at authoring time: 001 fails (consumed ids survive every branch), 002 fails
// to COMPILE (`undefined: PruneConsumed`), 003 is the pre-existing-green
// no-regression guard.
package cycle1182

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// wavePkg is the wave planner's home package (package main — importable
	// only via `go test`, hence the subprocess form).
	wavePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	// triagecapPkg owns the prune primitive that must become exported.
	triagecapPkg = "github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (e.g. the still-unexported pruneConsumed) surfaces as a
// non-zero exit — the intended RED signal before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1182_001_WidenSeamPrunesConsumedCommittedIDs — the crux AC. Drives
// widenNarrowDecision over a real inbox lifecycle: a committed id in a TERMINAL
// state (processed/rejected/quarantine) must be absent from the emitted top_n
// (a) when the pre-prune committed list was already fleet-width, so the
// `len(committed) >= count` short-circuit cannot smuggle it through, and (b) when
// the backlog is EMPTY so there is no replacement and the
// `len(topN) <= len(committed)` guard would otherwise return the original bytes.
// The fail-open half is asserted in the same run: pending/processing/retry and an
// id with NO lifecycle evidence at all must be RETAINED — a prune that dropped
// what it cannot resolve would starve every wave of non-inbox-backed cards.
func TestC1182_001_WidenSeamPrunesConsumedCommittedIDs(t *testing.T) {
	ok, out := runGoTest(t, wavePkg,
		"TestWidenNarrowDecision_DropsConsumedCommittedAtFleetWidth|TestWidenNarrowDecision_ConsumedIDDroppedEvenWithNoBacklogReplacement|TestWidenNarrowDecision_PrunesTerminalStatesOnly")
	if !ok {
		t.Errorf("widenNarrowDecision still carries lifecycle-consumed committed ids forward "+
			"(or over-prunes ids it cannot resolve) — a consumed id will be re-pinned into the next wave's lane-scope.json:\n%s", out)
	}
}

// TestC1182_002_PruneConsumedIsExportedAndSingleSourced — the reuse AC. The
// widen seam lives in package main and cannot reach the package-private
// pruneConsumed, so the primitive must be EXPORTED (triagecap.PruneConsumed)
// rather than reimplemented at the second call site
// (never_duplicate_centralize_via_design_patterns). This predicate calls the
// exported function directly through its contract test across all seven
// lifecycle states; while it is still unexported the package does not compile
// and this predicate is RED.
func TestC1182_002_PruneConsumedIsExportedAndSingleSourced(t *testing.T) {
	ok, out := runGoTest(t, triagecapPkg,
		"TestPruneConsumed_ExportedTerminalDropNonTerminalKeep|TestPruneConsumed_ExportedEmptyInputIsIdentity")
	if !ok {
		t.Errorf("triagecap.PruneConsumed is not callable from outside the package with pruneConsumed's "+
			"terminal-drop / fail-open semantics — the widen seam cannot reuse the SSOT:\n%s", out)
	}
}

// TestC1182_003_WaveNPlusOneAndFloorsPassthroughIntact — the regression AC. Two
// behaviours must hold after the change: (1) the two-wave fixture the inbox item
// describes (wave N commits+consumes an id; wave N+1 re-plans from wave N's
// decision) shows the consumed id in NO lane scope, driven end-to-end through the
// real fleet.PlanFromTriage; and (2) the pre-existing guards are undisturbed — a
// decision carrying committed_floors still passes through byte-identical, and the
// committed lane still deepens with its pending same-file cluster mates.
func TestC1182_003_WaveNPlusOneAndFloorsPassthroughIntact(t *testing.T) {
	ok, out := runGoTest(t, wavePkg,
		"TestWaveNPlusOneExcludesConsumedScope|TestWidenNarrowDecision_CommittedFloorsShortCircuit|TestWidenNarrowDecision_ExpandsCommittedLaneWithClusterMates")
	if !ok {
		t.Errorf("wave N+1 still inherits the consumed scope, or the prune disturbed the committed_floors "+
			"byte-identical passthrough / cluster-mate deepening:\n%s", out)
	}
}
