//go:build acs

// Package cycle1562 materialises the cycle-1562 acceptance criteria for the
// two fleet-scoped tasks `retrospective-delivery-relaunch` and
// `retrospective-delivery-evidence-contract`.
//
// The defect (.evolve/runs/cycle-1510/retrospective-launch-error.txt +
// retrospective-interactions.ndjson): a retrospective launch logged "prompt
// delivered", produced zero tokens and zero cost, and then burned two full
// 900-second stop-review intervals before dying with ExitArtifactTimeout. The
// tmux driver's submit-verify guard (go/internal/bridge/driver_tmux_submitverify.go)
// had ALREADY classified that pane as `submit_wedged` within milliseconds —
// but both consumer sites in driver_tmux_repl.go (the prompt-paste site and
// the one-shot nudge site) pipe verifySubmitted's outcome straight into
// recordSubmitVerify, which appends to the ndjson ledger and returns nothing
// usable for control flow. The classification is produced and never consumed:
// at the control-flow level a detected delivery failure is indistinguishable
// from a healthy launch that simply never speaks, and the run spends the whole
// silence budget before the dispatcher's already-present one-relaunch recovery
// (cyclerun_dispatch.go, IsInfraTeardownError) ever gets a turn.
//
// Task 1 makes the evidenced delivery failure short-circuit into the EXISTING
// bounded ExitArtifactTimeout outcome — no new retry loop, no new sentinel.
// Task 2 carries the classified cause through the bridge boundary into the
// terminal <phase>-failure-diag.json as a machine-readable field, so an
// exhausted retro leaves durable evidence instead of a bare artifact timeout
// whose root cause exists only in discarded stderr.
//
// Predicate strategy — every seam this cycle touches (runTmuxREPL,
// verifySubmitted, artifactTimeoutSummary, writePhaseFailureDiag) is
// UNEXPORTED inside package bridge / package core, so a leaf acs package
// cannot call it. These predicates therefore use the sanctioned
// behavioural-via-subprocess shape (cycle-987/997/1532/1544/1550 precedent):
// a `-run`-narrowed, single-named-package `go test -v` that must print
// `--- PASS: <name>` for each binding test the Builder makes pass. Asserting
// on the PASS line (never on exit 0) is load-bearing: `go test -run` against a
// pattern matching no test exits 0 with "no tests to run", so a deleted or
// renamed binding test would otherwise false-GREEN. The `-run` narrowing is
// also what keeps ./internal/core inside the ACS lane's wall-clock budget.
package cycle1562

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// bridgePkg owns the driver/engine seam: the submit-verify consumer sites
	// and the artifact-timeout marker the classified cause must ride.
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	// corePkg owns the dispatch/failure-learning seam: the one bounded
	// relaunch and the terminal failure diagnostic.
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to print a
// `--- PASS: <name>` line. -count=1 defeats the test cache, so a stale cached
// result from an earlier phase can never stand in for a live run.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC1562_001_WedgedPromptShortCircuitsTheSilenceBudget — AC1-001, the
// cycle-1510 reproduction. A verified `submit_wedged` prompt must return
// ExitArtifactTimeout IMMEDIATELY, before the driver enters a single two-second
// artifact-wait poll. Today the outcome is discarded and the run consumes the
// entire silence budget first; the binding test counts the polls, so a fix that
// merely relabels the eventual timeout cannot satisfy it.
func TestC1562_001_WedgedPromptShortCircuitsTheSilenceBudget(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget",
	)
}

// TestC1562_002_CleanAndSilentPanesAreNeverDeliveryFailures — AC1-002, the
// false-negative guard the scout's criteria name explicitly, and the highest-
// leverage negative in this cycle. A verified-clean submission must still exit
// ExitOK with no timeout marker at all, and a generically silent pane (input
// line clear, agent simply quiet) must still burn the normal silence budget and
// die with the GENERIC stop-review reason. A short-circuit that fires without an
// evidenced submit-verification failure would convert every ordinary slow phase
// into a bridge relaunch — strictly worse than the bug being fixed.
func TestC1562_002_CleanAndSilentPanesAreNeverDeliveryFailures(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_CleanSubmit_NeverClassifiesDeliveryFailure",
		"TestTmuxREPL_SilentPaneTimeout_NotClassifiedAsDeliveryFailure",
	)
}

// TestC1562_003_WedgedNudgeCarriesItsClassifiedCause — AC1-003, the SECOND
// consumer site. The one-shot nudge fires from inside the stop-review pause
// branch, so it cannot skip a budget it has already spent — but its wedged
// outcome is the same evidence, and the terminal artifact-timeout marker must
// name the site and the classification instead of reporting the generic stall
// reason. Without this, cycle-1510's ndjson (`"result":"no_effect"` on every
// nudge) remains the only place the cause exists.
func TestC1562_003_WedgedNudgeCarriesItsClassifiedCause(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker",
	)
}

// TestC1562_004_RecoveryStaysBoundedAtOneRelaunch — AC1-004, the boundedness
// floor. The task is "reach the EXISTING one bounded relaunch sooner", never
// "add a retry loop": the re-send budget must stay capped at three and the
// dispatcher must still relaunch exactly once and then abort. These four names
// are the pre-existing production coverage of both bounds; naming them here
// makes the cheapest wrong fix (loosen a bound to green AC1-001) RED, because a
// deleted or weakened test prints no PASS line.
func TestC1562_004_RecoveryStaysBoundedAtOneRelaunch(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_NudgeUnsubmitted_ResendBounded",
		"TestTmuxREPL_NudgeSubmitted_NoResend",
	)
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers",
		"TestOrchestrator_PhaseArtifactTimeout_AbortsAfterCap",
	)
}

// TestC1562_005_ClassifiedCauseSurvivesTheBridgeBoundary — AC2-001, the wiring
// proof for the evidence contract at its first hop. Engine.Launch discards
// driver stderr past the launch-error file, so a short-circuit that skipped the
// artifactTimeoutMarker shape would silently drop the cause on the floor. The
// binding test drives the real Engine.Launch and requires the returned phase
// error to carry BOTH the classified cause (site + `submit_wedged` + resend
// count) AND the core.ErrArtifactTimeout sentinel — the latter is what
// IsInfraTeardownError matches, so reclassifying the failure to any other
// sentinel would remove the very relaunch task 1 exists to reach.
func TestC1562_005_ClassifiedCauseSurvivesTheBridgeBoundary(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause",
	)
}

// TestC1562_006_TerminalDiagnosticIsMachineReadable — AC2-002. An exhausted
// retro must leave the classified delivery-failure cause as its OWN field in
// <phase>-failure-diag.json, not merely as a substring of the flat
// error_message. "Undelivered prompt" and "agent went quiet" share an exit code
// and a marker shape but have opposite remedies (relaunch the pane vs. raise the
// phase artifact budget), so the distinction has to survive as data.
func TestC1562_006_TerminalDiagnosticIsMachineReadable(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable",
	)
}

// TestC1562_007_NoFalseDeliveryFailureAttribution — AC2-003, the negative half
// of the evidence contract and the one that gives the new field its meaning. A
// generic silent-agent timeout (same exit 81, same marker shape, different
// reason) and a non-timeout failure (plain non-zero exit) must both leave the
// delivery-failure field EMPTY, with the pre-existing flat cause preserved
// verbatim. A field populated on every failure carries no information.
func TestC1562_007_NoFalseDeliveryFailureAttribution(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_GenericSilence_NoDeliveryFailureAttribution",
		"TestWritePhaseFailureDiag_NonTimeoutFailure_NoDeliveryFailureAttribution",
	)
}

// TestC1562_008_ExistingTimeoutEvidenceNotWeakened is the anti-weakening floor
// for the surfaces this cycle edits. Both tasks touch the artifact-timeout
// cause-selection path (driver marker → artifactTimeoutSummary → phaseErr →
// failure diagnostic), and the cheapest way to green the new assertions is to
// relax the pre-existing ones: the marker's waited/extends fields, the
// non-timeout exit whose cause must stay unchanged, the silent pane that must
// stay non-transient, and the end-to-end guarantee that the timeout diagnostic
// never mutates retry or failure-learning behaviour.
func TestC1562_008_ExistingTimeoutEvidenceNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestEngineLaunch_ArtifactTimeout_ErrorCarriesWaitAndExtends",
		"TestEngineLaunch_NonTimeoutExit_CauseUnchanged",
		"TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient",
	)
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning",
	)
}
