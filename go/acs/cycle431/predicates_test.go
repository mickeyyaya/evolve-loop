//go:build acs

// Package cycle431 materialises the cycle-431 acceptance criteria for slice
// S3 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S2 (prior cycle, #291) shipped panestream.SignalCenter fully built and
// behaviorally tested but with ZERO consumers. S3 (this cycle) wires it as
// the AUTHORITATIVE liveness source in the tmux-REPL driver's stop-review
// checkpoint and retires the reviewer's pre-S3 Progressed/Busy boolean
// fallback (verdict becomes a pure function of StopEvent.State).
//
// Tasks:
//
//	signalcenter-s3-authoritative-driver (Task A — M, P0):
//	  Wire Observe+Aggregate at the checkpoint; retire the boolean fallback;
//	  keep Progressed/Busy populated for fatalpane.go C2 + logging; preserve
//	  the cycle-291 render-wedge override.
//
//	signalcenter-s3-wedge-invariant-corpus (Task B — S, P1, dependsOn Task A):
//	  A behavioral corpus pinning every wedge-incident invariant (311/312,
//	  254/255, 262, 286/288) against the migrated, center-authoritative
//	  verdict path.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	Task A (signalcenter-s3-authoritative-driver):
//	  AC1 driver calls Observe+Aggregate (positive)              → C431_001 RED (compile fail)
//	  AC2 verdict = f(State), boolean fallback retired (negative)→ C431_002 RED (behavioral contradiction TODAY)
//	  AC3 full bridge+panestream -race green (regression)        → C431_003 RED (compile fail)
//	  AC4 apicover -enforce clean on both packages (regression)  → C431_004 RED (compile fail cascades into coverage run)
//	  AC5 negative: center not bypassed                          → C431_005 RED (compile fail)
//	  AC6 edge: cycle-291 render-wedge preserved                 → C431_006 RED (compile fail)
//
//	Task B (signalcenter-s3-wedge-invariant-corpus):
//	  AC1 311/312 producing-not-capped (positive)                → C431_007 RED (compile fail — same package as Task A)
//	  AC2 254/255 bounded busy (positive)                        → C431_008 RED (compile fail)
//	  AC3 262 dead-pane not progress (negative)                  → C431_009 RED (compile fail)
//	  AC4 286/288 evidence survives (edge)                       → C431_010 RED (compile fail)
//	  AC5 corpus green under -race (regression)                  → C431_011 RED (compile fail)
//	  AC6 anti-gaming: names Liveness* + calls Review             → C431_012 RED (compile fail)
//
// RED strategy: C431_001, 003–012 are all RED for the SAME root cause —
// driver_tmux_repl_signalcenter_test.go references the not-yet-existing
// Deps.LivenessCenter field, so the whole internal/bridge test binary fails
// to COMPILE (a hard, non-gameable RED: no implementation can accidentally
// satisfy a compile error, and every test in the package — including Task
// B's, which depends on Task A — inherits the failure). C431_002 is
// independently RED on its own merits: it calls the REAL, ALREADY-COMPILING
// deterministicReviewer.Review and asserts the OPPOSITE of what it returns
// TODAY (Extend, via the still-present boolean fallback), so it fails for a
// behavioral reason even standing alone, with no dependency on the compile
// failure elsewhere.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C431_002 (unset State + Progressed/Busy true must PAUSE, not
//	            extend — contradicts the current fallback), C431_005 (probe
//	            must be CALLED, not just state-coincidence), C431_009 (Hung
//	            must never extend unconditionally like Converging)
//	Edge/OOD:   C431_006 (blank-but-live render wedge), C431_010 (evidence
//	            across a dead-session empty capture), C431_007 (attempt
//	            count far past maxExtends)
//	Semantic:   C431_001 (state SOURCE) vs C431_005 (probe INVOCATION) are
//	            distinct proofs — a state-only check would pass on
//	            accidental coincidence even if the center were bypassed;
//	            C431_007 vs C431_008: unconditional-extend vs
//	            bounded-extend-then-pause are different reviewer behaviors,
//	            not one behavior restated
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C431_001–006) → total=6 ✓
//	Task B: predicate=6 (C431_007–012) → total=6 ✓
package cycle431

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgeImportPath     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	panestreamImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// runBridgeTest shells to `go test -run <runFilter>` for the bridge package.
func runBridgeTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, bridgeImportPath,
	)
	return stdout, stderr, code
}

// runRaceSuite shells to `go test -race` (optionally -run filtered) over the
// given import paths.
func runRaceSuite(t *testing.T, runFilter string, pkgs ...string) (string, string, int) {
	t.Helper()
	args := []string{"test", "-race", "-count=1"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

// ── Task A: signalcenter-s3-authoritative-driver ─────────────────────────────

// TestC431_001_DriverRoutesStateThroughCenter (AC1, positive, RED — compile
// fail): the checkpoint's StopEvent.State must come from a
// panestream.SignalCenter.Observe/Aggregate call the driver makes, not the
// bare per-run detectorFor(lp) probe.
func TestC431_001_DriverRoutesStateThroughCenter(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_SignalCenterStateWins")
	if code != 0 {
		t.Errorf("C431_001: driver-uses-SignalCenter test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_002_BooleanFallbackRetired (AC2, negative, RED — behavioral):
// StopEvent.State must be the reviewer's SOLE liveness source; the pre-S3
// Progressed/Busy fallback must be retired, not merely shadowed.
func TestC431_002_BooleanFallbackRetired(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestStopEvent_BooleanFallbackRetired")
	if code != 0 {
		t.Errorf("C431_002: boolean-fallback-retired test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_003_BridgeAndPanestreamRaceGreen (AC3, regression, RED — compile
// fail): the full bridge + panestream suite must stay green under -race
// after the migration (H1: byte-identical verdicts on every fixture).
func TestC431_003_BridgeAndPanestreamRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C431_003: bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_004_ApicoverEnforceClean (AC4, regression, RED — compile fail
// cascades into the coverage run): apicover -enforce must report 0
// uncovered / 0 false-green symbols on both touched packages — this is the
// recurring CI-break class from cycles 413/426/430.
func TestC431_004_ApicoverEnforceClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover431")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C431_004: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage431.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C431_004: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C431_004: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage431.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C431_004: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C431_004: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C431_004: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C431_004: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

// TestC431_005_CenterProbeActuallyInvoked (AC5, negative, RED — compile
// fail): a registered SignalCenter probe must actually be CALLED — the
// anti-bypass guard AC1 alone cannot rule out.
func TestC431_005_CenterProbeActuallyInvoked(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_SignalCenterProbeActuallyInvoked")
	if code != 0 {
		t.Errorf("C431_005: center-probe-invoked test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_006_RenderWedgeOverridePreserved (AC6, edge, RED — compile
// fail): the cycle-291 blank-but-live render-wedge override must still
// promote Idle→BusyButStagnant after the migration to the
// center-authoritative State source.
func TestC431_006_RenderWedgeOverridePreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant")
	if code != 0 {
		t.Errorf("C431_006: render-wedge-preserved test exit=%d\nstderr=%s", code, stderr)
	}
}

// ── Task B: signalcenter-s3-wedge-invariant-corpus ───────────────────────────

// TestC431_007_ConvergingProducingNeverCapped (AC1, positive, RED — compile
// fail): cycle-311/312 — a producing agent is never capped.
func TestC431_007_ConvergingProducingNeverCapped(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_Converging_ProducingNeverCapped")
	if code != 0 {
		t.Errorf("C431_007: converging-never-capped test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_008_BusyStagnantBoundedThenPause (AC2, positive, RED — compile
// fail): cycle-254/255 — bounded busy extends then pauses at maxExtends.
func TestC431_008_BusyStagnantBoundedThenPause(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_BusyStagnant_BoundedThenPause")
	if code != 0 {
		t.Errorf("C431_008: busy-stagnant-bounded test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_009_DeadPaneHungNotConverging (AC3, negative, RED — compile
// fail): cycle-262 — a dead/echoing pane must classify Hung, never
// Converging.
func TestC431_009_DeadPaneHungNotConverging(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_DeadPane_HungIsNotConverging")
	if code != 0 {
		t.Errorf("C431_009: dead-pane-hung test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_010_EvidenceSurvivesEmptyCapture (AC4, edge, RED — compile
// fail): cycle-286/288 — non-empty pane evidence survives a dead-session
// empty capture.
func TestC431_010_EvidenceSurvivesEmptyCapture(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_EvidenceSurvivesEmptyCapture")
	if code != 0 {
		t.Errorf("C431_010: evidence-survives test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC431_011_WedgeCorpusRaceGreen (AC5, regression, RED — compile fail):
// the full wedge-invariant corpus must stay green under -race.
func TestC431_011_WedgeCorpusRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "Wedge|Converging|BusyStagnant|DeadPane|Evidence", bridgeImportPath)
	if code != 0 {
		t.Errorf("C431_011: wedge corpus -race exit non-zero\nstderr=%s", stderr)
	}
}

// TestC431_012_CorpusUsesRealReviewer (AC6, anti-gaming, RED — compile
// fail): the corpus must exercise the production deterministicReviewer,
// not a stub standing in for it.
func TestC431_012_CorpusUsesRealReviewer(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_UsesRealDeterministicReviewer")
	if code != 0 {
		t.Errorf("C431_012: uses-real-reviewer test exit=%d\nstderr=%s", code, stderr)
	}
}
