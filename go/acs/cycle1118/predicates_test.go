//go:build acs

// Package cycle1118 materialises the cycle-1118 acceptance criteria for the single
// fleet-scoped task pinned to this lane:
//
//	fatalpane-persistence-gate → the ADR-0044 C2 fatal-pane fast-fail
//	  (go/internal/bridge/fatalpane.go) matches on RAW pane substrings and fires on
//	  ONE observation, while its sibling quota-wall fast-fail
//	  (exhaustion_persistence.go) has been persistence-gated since the
//	  cycle-254/255/314/641 false-FAIL lineage precisely because a WORKING agent can
//	  render fatal-shaped text into the pane it is being judged on. This cycle closes
//	  the asymmetry: a fatal match must persist for `fatalPanePersistObservations`
//	  CONSECUTIVE checkpoints before it can preempt the reviewer or leave C2
//	  evidence, while a genuinely parked pane still fast-fails at the threshold
//	  observation (no regression of the cycle-262 rescue path).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 EXERCISE the gate through the package's behavioural unit tests —
//     the transient/reset class fix, the persistent-still-fires bound, and the
//     shadow would/did parity semantics. Each shelled run REQUIRES a real `--- PASS`
//     line, so a renamed, filtered-away, or deleted test is a FAIL, never a silent
//     green.
//   - 004 is the WIRING proof: the checkpoint loop must own exactly ONE gate
//     instance (a per-call gate can never accumulate a streak — it would leave every
//     behavioural test above green while production stays un-gated) and must reach
//     the seam THROUGH it, with no bare fatalPaneVerdict call left behind. This one
//     reads the loop's AST rather than running it: the fatal-pane branch of the
//     stop-review loop is only reachable through a live tmux session whose liveness
//     projection also decides Busy — which independently suppresses preemption — so
//     an end-to-end run cannot isolate the gate's contribution. Structural, but
//     shape-based (call graph inside the owning function), not a string grep for a
//     magic token.
//   - 005 is the regression axis: fatalPaneVerdict's OWN single-observation contract
//     is unchanged, so the pre-existing fatalpane_test.go / fatalpane_durable_test.go
//     cases must still pass UNMODIFIED.
package cycle1118

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the package under test; goDir is the worktree's Go module root, so
// every shelled lane compiles THIS cycle's tree, not main's stale copy.
const bridgePkg = "./internal/bridge/"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// runGoTests runs the named tests in the worktree module and requires a real PASS
// line for each one (a filtered-away or renamed test exits 0 with no PASS — that is
// a FAIL here, not a silent green).
func runGoTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	filter := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", filter, pkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", pkg, filter, code, out)
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Fatalf("no PASS line for %s in %s (renamed, skipped, or never ran?)\n%s", name, pkg, out)
		}
	}
}

// TestC1118_001_TransientFatalPaneNeverFastFails — the class fix, negative axis: one
// fatal-shaped frame from a working agent must never preempt the reviewer nor leave
// a fast_failed record, and a non-matching observation (healthy pane, or a Busy
// checkpoint) must RESET the streak so two NON-consecutive lone matches cannot
// accumulate into a kill.
func TestC1118_001_TransientFatalPaneNeverFastFails(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneGate_TransientMatchDoesNotFastFail",
		"TestFatalPaneGate_BusyObservationResetsStreak",
		"TestFatalPaneGate_DisabledPathsNeverAccumulate",
	)
}

// TestC1118_002_PersistentFatalPaneStillFastFailsBounded — the rescue-path bound: a
// pane parked in a fatal state still fast-fails, at exactly the threshold
// observation (not later), with the unchanged ReviewStop + typed-cause verdict and a
// single fast_failed record. Pins the persistence bar to the quota-wall precedent's,
// so a threshold of 1 (the un-gated behavior wearing a gate's name) fails here.
func TestC1118_002_PersistentFatalPaneStillFastFailsBounded(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneGate_PersistentFatalPaneStillFastFails",
		"TestFatalPaneGate_ThresholdMirrorsExhaustionGuard",
	)
}

// TestC1118_003_ShadowEvidenceTracksGatedEnforce — shadow must PREDICT the gated
// enforce action: no would_fast_fail record and no stderr line on an un-persisted
// match, both once the gate crosses. Otherwise the R8.5 would/did parity check
// compares pre-gate shadow semantics against post-gate enforce ones.
func TestC1118_003_ShadowEvidenceTracksGatedEnforce(t *testing.T) {
	runGoTests(t, bridgePkg, "TestFatalPaneGate_ShadowEvidenceOnlyAfterPersistence")
}

// TestC1118_004_CheckpointLoopOwnsExactlyOneGate — the WIRING proof. The gate must
// be loop-scoped state, exactly like checkpointExhaustGate: constructed ONCE in the
// function that owns the stop-review checkpoint loop, and the fatal-pane seam must
// be reached through it. A gate constructed per checkpoint can never accumulate a
// streak, and a bare fatalPaneVerdict call left in the loop keeps production
// un-gated — both leave every behavioural predicate above green.
func TestC1118_004_CheckpointLoopOwnsExactlyOneGate(t *testing.T) {
	path := filepath.Join(goDir(t), "internal", "bridge", "driver_tmux_repl.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	// The owning function is identified by the PRECEDENT it must sit beside — the
	// exhaustion gate's construction — not by name, so a rename of the driver
	// function does not silently skip this predicate.
	var owner *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if countCalls(fn, isIdentCall("newExhaustionGate")) > 0 {
			if owner != nil {
				t.Fatalf("two functions construct newExhaustionGate() — cannot identify the checkpoint loop owner")
			}
			owner = fn
		}
	}
	if owner == nil {
		t.Fatalf("no function in %s constructs newExhaustionGate() — the checkpoint-loop precedent moved; this predicate needs re-anchoring", filepath.Base(path))
	}

	if n := countCalls(owner, isIdentCall("newFatalPaneGate")); n != 1 {
		t.Errorf("%s constructs newFatalPaneGate() %d time(s), want exactly 1 — the gate must be loop-scoped state (a per-checkpoint gate never accumulates a streak, so a transient match still kills)", owner.Name.Name, n)
	}
	if n := countCalls(owner, isIdentCall("fatalPaneVerdict")); n != 0 {
		t.Errorf("%s still calls fatalPaneVerdict directly %d time(s) — the checkpoint must reach the seam through the gate, or production stays un-gated regardless of the gate's unit tests", owner.Name.Name, n)
	}
	if n := countCalls(owner, isMethodCall("verdict")); n != 1 {
		t.Errorf("%s makes %d gate .verdict(...) call(s), want exactly 1 — fatal-pane evidence is recorded per matching call, so a second call would inflate the soak's C2 counts silently", owner.Name.Name, n)
	}
}

// TestC1118_005_ExistingFatalPaneContractUnchanged — regression axis: the gate is
// ADDITIVE state around fatalPaneVerdict, not a change to its contract for a single
// observation, so the pre-existing stage-discipline and durable-evidence cases must
// still pass unmodified.
func TestC1118_005_ExistingFatalPaneContractUnchanged(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneVerdict_EnforcePreemptsWithStop",
		"TestFatalPaneVerdict_ShadowLogsButDoesNotPreempt",
		"TestFatalPaneVerdict_BusyPaneNeverPreempted",
		"TestFatalPaneVerdict_OffSkipsDetection",
		"TestFatalPaneVerdict_ShadowRecordsDurableEvidence",
		"TestFatalPaneVerdict_EnforceRecordsFastFailed",
		"TestFatalPaneVerdict_BusyAndOffRecordNothing",
	)
}

// isIdentCall matches a plain call to a package-level function: `name(...)`.
func isIdentCall(name string) func(*ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		id, ok := call.Fun.(*ast.Ident)
		return ok && id.Name == name
	}
}

// isMethodCall matches a call through a selector: `x.name(...)`.
func isMethodCall(name string) func(*ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel != nil && sel.Sel.Name == name
	}
}

// countCalls counts the call expressions inside fn matching the predicate.
func countCalls(fn *ast.FuncDecl, match func(*ast.CallExpr) bool) int {
	n := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && match(call) {
			n++
		}
		return true
	})
	return n
}
