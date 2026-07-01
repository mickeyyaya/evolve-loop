//go:build acs

// Package cycle434 materialises the cycle-434 acceptance criteria for the
// completion of slice S4 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S2 (#291), S3 (cycle-431), and S4's checkpoint migration (cycle-432) are
// all landed on main. S4's own charter — "remove [driver/reviewer] direct
// call sites so consumers no longer parse CLI chrome" — is INCOMPLETE
// against itself: two direct panestream.PaneBusy consumers survive
// (autorespond.go:282 auto-responder busy-gate; driver_tmux_repl.go:587
// idle_reached busy/idle bracket). This cycle's sole task closes that gap.
//
// Task (top_n):
//
//	s4-complete-residual-busy-callsites (M, P0):
//	  Add a STATELESS panestream.SignalCenter.BusyOf(rendered, profile) bool
//	  projection (no Observe, no per-session state — delegates to the SAME
//	  PaneBusy definition the registered Busy(sessionKey) handler already
//	  uses) and route both residual call sites through it, so no `bridge`
//	  consumer parses CLI chrome directly anymore.
//
// AC map (1:1, R9.3 floor-binding; predicates for the ## top_n task only):
//
//	AC1 auto-responder busy-gate preserved (positive)             → C434_001 pre-existing GREEN (value already correct; migration is behavior-preserving) + C434_002 (idle counterpart pin)
//	AC2 idle_reached fires once on busy→idle via facade (positive)→ C434_003 pre-existing GREEN (TestChannelE2E_RealFixtures_ClaudeSpan, channel_e2e_test.go — unaffected by BusyOf delegating to the identical PaneBusy definition)
//	AC3 no direct panestream.PaneBusy( at either residual site
//	    (negative, discriminating)                                → C434_004 RED today + C434_005 RED today
//	AC4 empty pane / unknown profile → not-busy, no panic; BusyOf
//	    is stateless (no session-state mutation), nil-receiver-safe
//	    (edge/OOD)                                                 → C434_006 RED today (compile fail) + C434_007 RED today (compile fail)
//	AC5 -race green + apicover -enforce 0 uncovered (regression)  → C434_008 RED today (compile fail cascade) + C434_009 RED today (compile fail cascade)
//
// RED strategy: C434_004/005 are independently RED on their own merits (the
// two call sites genuinely still call panestream.PaneBusy( inline today —
// verified by direct `go test -run` against the CURRENT tree before this
// cycle's test files existed). C434_001/002/003 currently PASS standalone
// (the migration is designed to be behavior-preserving, H1) but the whole
// panestream test binary — and therefore every predicate that shells into
// a package sharing a build with signalcenter_busyof_test.go — fails to
// COMPILE once BusyOf is referenced and does not yet exist; C434_006-009
// are RED for that same root cause (a hard, non-gameable RED: no
// implementation can accidentally satisfy a compile error).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C434_004/005 (the two-site "keep the direct call AND also
//	            route through the center" cheapest fake — a source-region
//	            scan defeats a redundant call, not just a value check)
//	Edge/OOD:   C434_006 (empty pane / unknown-profile zero-value, and a
//	            nil *SignalCenter receiver)
//	Semantic:   C434_007 (BusyOf must NOT create session state — a distinct
//	            property from "returns the right bool", proven by asserting
//	            Aggregate()/Busy()/Changed() stay at their unobserved
//	            defaults after BusyOf-only calls)
//
// 1:1 enforcement:
//
//	predicate=9 (C434_001-009) → total AC = 5 (each AC gets >=1 predicate) ✓
package cycle434

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

// runPanestreamTest shells to `go test -run <runFilter>` for the panestream
// package.
func runPanestreamTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, panestreamImportPath,
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

// TestC434_001_AutoResponderBusyGatePreserved (AC1, positive, pre-existing
// GREEN): a pane matching an escalate-policy prompt while ALSO carrying a
// live-turn affordance must not escalate through autoResponder.tick — the
// value is already correct today (the migration to BusyOf is designed to be
// behavior-preserving); this predicate pins it so a future refactor cannot
// silently regress it.
func TestC434_001_AutoResponderBusyGatePreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate")
	if code != 0 {
		t.Errorf("C434_001: busy-gate-preserved test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_002_AutoResponderIdleStillEscalates (AC1, positive counterpart —
// discriminates a gate that always suppresses from one that reads the real
// busy signal): the same escalate-matching text on an idle pane must still
// escalate.
func TestC434_002_AutoResponderIdleStillEscalates(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_IdleGateViaCenter_Escalates")
	if code != 0 {
		t.Errorf("C434_002: idle-still-escalates test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_003_IdleReachedFiresOnceViaFacade (AC2, positive, pre-existing
// GREEN): the correlation-span bracket must fire idle_reached exactly once
// on a real busy→idle transition, driven end-to-end against real captured
// claude frames (TestChannelE2E_RealFixtures_ClaudeSpan). Unaffected by the
// BusyOf migration because BusyOf delegates to the identical PaneBusy
// definition (verdict-identical).
func TestC434_003_IdleReachedFiresOnceViaFacade(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestChannelE2E_RealFixtures_ClaudeSpan")
	if code != 0 {
		t.Errorf("C434_003: idle_reached-via-facade e2e test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_004_NoDirectChromeParseAtAutoResponderTick (AC3, negative — RED
// today): the autoResponder.tick busy-gate region must no longer call
// panestream.PaneBusy( directly.
func TestC434_004_NoDirectChromeParseAtAutoResponderTick(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_NoDirectChromeParse")
	if code != 0 {
		t.Errorf("C434_004: no-direct-chrome-parse (autorespond) test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_005_NoDirectChromeParseAtIdleReachedBracket (AC3, negative — RED
// today): the idle_reached correlation-span bracket must no longer call
// panestream.PaneBusy( directly.
func TestC434_005_NoDirectChromeParseAtIdleReachedBracket(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_NoDirectChromeParseAtIdleReachedBracket")
	if code != 0 {
		t.Errorf("C434_005: no-direct-chrome-parse (idle_reached bracket) test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_006_BusyOfEdgeCasesNoPanic (AC4, edge/OOD — RED today, compile
// fail): BusyOf must read not-busy (never panic) for an empty pane, an
// unknown/zero-value profile, and a nil *SignalCenter receiver.
func TestC434_006_BusyOfEdgeCasesNoPanic(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestSignalCenter_BusyOf_EmptyPaneUnknownProfileNoPanic|TestSignalCenter_BusyOf_NilReceiverSafe")
	if code != 0 {
		t.Errorf("C434_006: BusyOf edge-case tests exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_007_BusyOfStatelessNoSessionMutation (AC4, semantic/edge — RED
// today, compile fail): BusyOf must delegate to the SAME PaneBusy
// definition AND must never mutate SignalCenter session state — a distinct
// property from "returns the right bool" (F3, scout finding: routing these
// residual sites through the stateful Observe path would pollute the
// checkpoint's interval baseline).
func TestC434_007_BusyOfStatelessNoSessionMutation(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestSignalCenter_BusyOf_MatchesStandalonePaneBusy|TestSignalCenter_BusyOf_StatelessNoSessionMutation")
	if code != 0 {
		t.Errorf("C434_007: BusyOf stateless/delegation tests exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_008_BridgeAndPanestreamRaceGreen (AC5, regression — RED today,
// compile fail cascade): the full bridge + panestream suite must stay green
// under -race after the migration.
func TestC434_008_BridgeAndPanestreamRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C434_008: bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC434_009_ApicoverEnforceClean (AC5, regression — RED today, compile
// fail cascades into the coverage run): apicover -enforce must report 0
// uncovered / 0 false-green symbols on both touched packages — this is the
// recurring CI-break class from cycles 413/426/430.
func TestC434_009_ApicoverEnforceClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover434")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C434_009: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage434.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C434_009: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C434_009: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage434.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C434_009: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C434_009: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C434_009: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C434_009: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}
