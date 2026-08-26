//go:build acs

// Package cycle1568 encodes the cycle-1568 acceptance criteria for the
// `retrospective-delivery-relaunch` lane: the two production-path defects on
// the Retro dispatch route recorded in
// .evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json —
//
//	retro-delivery-failure-relaunch  a typed, verified delivery failure
//	                                 (driver-classified submit_wedged, zero
//	                                 tokens, prompt parked at the pane) must
//	                                 trigger exactly one fresh Retro dispatch,
//	                                 while a generic artifact timeout must not.
//	retro-model-auto-normalization   Retro must never dispatch the literal
//	                                 "auto" model sentinel to the bridge.
//
// Every predicate here is BEHAVIORAL: it runs the retro phase's real
// Phase.Run → core.Bridge route through `go test` and asserts on the named
// PASS marker, so adding a magic string to a source file cannot green it.
package cycle1568

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// retroPkg is the single named package under test — never a /... sweep.
const retroPkg = "./internal/phases/retro"

// runRetroTest runs one named test in the retro package from the worktree's go
// module root (`go -C <root>/go`, never a cwd-relative invocation) and returns
// the combined output plus the exit code.
func runRetroTest(t *testing.T, name string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", retroPkg, "-run", "^"+name+"$", "-count=1", "-v")
	return stdout + stderr, code
}

// assertNamedPass fails unless the named test both RAN and reported PASS —
// exit 0 alone is gameable by deleting or renaming the test.
func assertNamedPass(t *testing.T, name string) {
	t.Helper()
	out, code := runRetroTest(t, name)
	if !strings.Contains(out, "=== RUN   "+name) {
		t.Fatalf("%s never ran (deleted, renamed, or filtered out)\n%s", name, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) || code != 0 {
		t.Errorf("%s did not PASS (exit=%d)\n%s", name, code, out)
	}
}

// AC1 (retro-delivery-failure-relaunch): a zero-token, prompt-visible terminal
// delivery failure triggers exactly one fresh Retro dispatch. The test drives
// retro.Phase.Run with a fake core.Bridge that returns the production-shaped
// submit_wedged ErrArtifactTimeout first and succeeds second; it asserts
// Launch was called exactly twice.
func TestC1568_001_submit_wedged_relaunches_retro_once(t *testing.T) {
	assertNamedPass(t, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
}

// AC2 (retro-delivery-failure-relaunch, NEGATIVE): a generic artifact timeout
// carrying no typed delivery classification must NOT relaunch — exactly one
// Launch and a FAIL verdict. This is the anti-over-fix guard: a blanket
// "retry every ErrArtifactTimeout" implementation reds here.
func TestC1568_002_generic_artifact_timeout_does_not_relaunch(t *testing.T) {
	assertNamedPass(t, "TestRun_GenericArtifactTimeout_DoesNotRelaunch")
}

// AC3 (retro-delivery-failure-relaunch): the regression rides the ACTUAL
// Core→Bridge/Retro route, not a diagnostic serializer. Proven two ways: the
// reproducer is git-TRACKED (an untracked test is dropped at ship, cycle-93),
// and it drives retro.Phase.Run through the core.Bridge port.
func TestC1568_003_relaunch_test_drives_production_retro_route(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "phases", "retro", "retro_test.go")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s untracked — may be gitignored and dropped at ship", rel)
	}
	// Auxiliary only (the load-bearing check is the executed test below):
	// the reproducer must exercise the phase entry point and the bridge port.
	for _, marker := range []string{"phase.Run(", "core.BridgeRequest", "core.ErrArtifactTimeout"} {
		if !acsassert.FileContains(t, abs, marker) {
			t.Errorf("RED: %s does not reference %q — not the production route", rel, marker)
		}
	}
	assertNamedPass(t, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
}

// AC4 (retro-model-auto-normalization): Retro never sends
// BridgeRequest.Model == "auto". With Config.Model unset, the dispatched
// request must carry the retrospective profile's resolved tier.
func TestC1568_004_retro_never_dispatches_auto_model(t *testing.T) {
	assertNamedPass(t, "TestRun_AutoModel_ResolvedBeforeDispatch")
}

// AC5 (retro-model-auto-normalization): an explicitly configured tier survives
// unchanged — the normalization must not rewrite operator-pinned models.
func TestC1568_005_explicit_model_tier_unchanged(t *testing.T) {
	assertNamedPass(t, "TestRun_ExplicitModel_PassesThroughUnchanged")
}

// AC6 (retro-model-auto-normalization, EDGE/NEGATIVE): with a profile that
// carries no model_tier_default, and with no profile at all, the sentinel must
// still never reach the bridge — resolve to the established default or fail
// loudly, never forward "auto".
func TestC1568_006_sentinel_never_survives_degraded_resolution(t *testing.T) {
	assertNamedPass(t, "TestRun_AutoModel_ProfileWithoutTier_ResolvesToDefaultNotAuto")
	assertNamedPass(t, "TestRun_AutoModel_NoProfile_NeverDispatchesSentinel")
}

// AC7 (both tasks, regression): the whole retro phase package stays green —
// the existing SKIPPED/PASS/FAIL verdict mapping, the profile-CLI dispatch
// pins (cycle-107 class), and the new relaunch/model contract must hold
// together, not one at the cost of another.
func TestC1568_007_retro_package_suite_green(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", retroPkg, "-count=1")
	if code != 0 {
		t.Errorf("go test %s exited %d — retro package regression\nstdout:\n%s\nstderr:\n%s",
			retroPkg, code, stdout, stderr)
	}
}
