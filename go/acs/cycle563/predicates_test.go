//go:build acs

// Package cycle563 materialises the cycle-563 acceptance criteria for the
// scout-committed task `fix-memo-phase-dispatch` (see
// .evolve/runs/cycle-563/.evolve/evals/fix-memo-phase-dispatch.md).
//
// Root cause (fault-localization-report.md, confidence 0.97): the router
// legitimately plans "memo" after "ship" (cycle-561 routing-decision-12.json),
// but the runner-registration loop in wireOrchestratorDeps
// (go/cmd/evolve/cmd_cycle.go:406) validates the real .evolve/phases/memo
// overlay with the non-catalog-aware phasespec.ValidateUserSpec instead of
// ValidateUserSpecWithCatalog (the variant ApplyUserRouting already uses three
// lines above for the routing decision itself), so the single-word "memo"
// name is rejected and NO PhaseRunner is ever registered for it. The
// dispatcher then silently WARNs and advances past memo without ever running
// it (cyclerun_dispatch.go's missing-runner escape hatch) — explaining why
// completed_phases stops at "ship" across every PASS cycle sampled
// (555/557/558/559/561).
//
// Criterion 2 ("a real cycle produces memo artifacts end-to-end") is
// dispositioned manual+checklist in test-report.md, not a predicate here: it
// requires a live LLM-CLI subprocess dispatch (the real memo specrunner
// through the real bridge), which is neither deterministic nor safe to shell
// out to from an audit-gating unit predicate.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/561
// precedent) for criteria 1+3 — each shells `go test -run '^Name$' <pkg>` over
// the real RED unit tests authored this cycle. Criterion 4 shells the exact
// `evolve doctor` invocation the eval file specifies.
package cycle563

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	routerPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// runGoTest shells `go test -run '^<pattern>$' -count=1 <pkg>` and returns
// whether it exited cleanly plus the combined output for diagnostics. -count=1
// defeats the test cache so the predicate always exercises current source.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

// TestC563_001_MemoRunnerRegistrationRegression — criterion 1 [code]: "Regression
// test proves the routing→dispatch handoff actually launches memo". Drives
// TestWireOrchestrator_MemoRunnerRegistered, which wires the ACTUAL
// composition root (wireOrchestratorDeps) against the real registry + real
// memo overlay and asserts a PhaseRunner was registered — not just that
// Route() names "memo". RED until Builder swaps cmd_cycle.go:406 to
// ValidateUserSpecWithCatalog (and adds the HasRunner test seam); GREEN
// thereafter.
func TestC563_001_MemoRunnerRegistrationRegression(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestWireOrchestrator_MemoRunnerRegistered")
	if !ok {
		t.Errorf("wireOrchestratorDeps does not register a PhaseRunner for \"memo\" — the routing→dispatch handoff is still silently dropping it:\n%s", out)
	}
}

// TestC563_002_MemoDisabledClampsSafely — criterion 3 [code]: "Negative case —
// memo disabled still degrades safely". Drives both
// TestRoute_PostShip_MemoEnabled_RoutesToMemo (contrast/positive — proves the
// input genuinely reaches memo when enabled) and
// TestRoute_PostShip_MemoDisabled_ClampsSafely (the negative: the same
// post-ship input, with PhaseEnable["memo"]=off, must clamp to "end" and must
// NEVER return "memo"). Pre-existing GREEN today (router.go's walk/shouldRun
// already respect PhaseEnable correctly; the fault-localization report ruled
// this file a non-suspect) — kept as a permanent regression lock so a future
// fix attempt that force-runs memo unconditionally cannot silently regress it.
func TestC563_002_MemoDisabledClampsSafely(t *testing.T) {
	ok, out := runGoTest(t, routerPkg, "TestRoute_PostShip_MemoEnabled_RoutesToMemo|TestRoute_PostShip_MemoDisabled_ClampsSafely")
	if !ok {
		t.Errorf("post-ship memo enable/disable routing regressed (either memo never reaches Route() when enabled, or a disabled memo still gets routed):\n%s", out)
	}
}

// TestC563_003_DoctorBootNoMemoWarning — criterion 4 [code]: "Edge case —
// mid-fix boot warnings are gone or escalated correctly". Runs the exact
// command the eval file specifies (`evolve doctor 2>&1 | grep -i memo`) and
// asserts boot never prints an "invalid"/"clashes with a built-in" warning
// for memo while ALSO failing to route it — a silent WARN-and-drop is the
// class of bug this whole task fixes. NOTE (test-report.md): this criterion
// alone is not sufficient proof of the fix (a fake could suppress the log
// line without registering the runner) — it is a guard, not a substitute for
// criterion 1.
func TestC563_003_DoctorBootNoMemoWarning(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "run", cmdEvolvePkg, "doctor", "probe", "claude")
	if code == -1 {
		// -1 means the subprocess never launched at all (binary/module
		// resolution failure) — anything else (probe found/didn't find the
		// tool) is a normal exit code, not a launch failure, and still carries
		// the stdout/stderr this predicate scans.
		t.Fatalf("go run %s doctor probe claude failed to launch: %v\nstderr:\n%s", cmdEvolvePkg, err, stderr)
	}
	combined := stdout + stderr
	for _, line := range strings.Split(combined, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "memo") {
			continue
		}
		if strings.Contains(lower, "invalid") || strings.Contains(lower, "clashes with a built-in") || strings.Contains(lower, "not routed") {
			t.Errorf("evolve doctor printed a memo invalid/clash/not-routed warning while memo is also never dispatched — the silent WARN-and-drop class this task fixes:\n%s", line)
		}
	}
}
