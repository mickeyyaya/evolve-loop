//go:build acs

// Package cycle1005 materialises the cycle-1005 acceptance criteria for the sole
// fleet-scoped item of this lane, telemetry-coverage-tripwire-nonclaude-success.
// fleet_scope pins this lane to that one id, so per R9.3 no predicate binds to any
// other lane's work. Scout split the item into two triage-committed (top_n) tasks
// sharing one file (go/internal/bridge/engine.go) and one behavioral surface
// (recordTokenUsage): Task 1 telemetry-tripwire-nonclaude-exit0-warn and Task 2
// telemetry-tripwire-llm-calls-record.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001–004 run the in-package behavioral tests (package bridge, which can reach
//     the deliberately-UNEXPORTED recordTokenUsage method) as a SUBPROCESS and
//     require an explicit "--- PASS:" marker for each named test. A bare exit-0 is
//     rejected, so a renamed/skipped/deleted test cannot green the gate. The
//     in-package tests (engine_tripwire_test.go) are the RED contract authored this
//     TDD phase; the subprocess makes them the cycle's audit-gating predicate.
//
// Adversarial axes (SKILL §6): NEGATIVE — 002 requires the quota-abort (exit 85,
// short), exit-0 sub-threshold, claude-baseline, and covered launches to stay
// silent; each is an input that must NOT trip. EDGE — 003 requires the tripwire to
// fire fail-open when the cycle is not derivable from the workspace path. SEMANTIC
// — 001 (fires on unmeasured success), 002 (four distinct silence reasons), 003
// (fail-open), and 004 (queryable ndjson field) are distinct behaviors, not one
// restated.
package cycle1005

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// runBridgeTest runs `go test -run <pattern> <bridgePkg>` as a subprocess
// (verbose, no cache) and fails unless every named test reports an explicit PASS.
// RED today: the escalation does not exist, so the positive in-package tests fail
// their assertions and `go test` exits non-zero.
func runBridgeTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, bridgePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, bridgePkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

// TestC1005_001_tripwire_fires_on_nonclaude_exit0_success — AC1+AC2. A non-claude
// launch that exited 0, ran >60s, and resolved to source=none emits a distinct
// TRIPWIRE stderr line naming the CLI, the agent, and the cycle.
func TestC1005_001_tripwire_fires_on_nonclaude_exit0_success(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns$",
		"TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns")
}

// TestC1005_002_tripwire_stays_silent_on_non_signals — AC3 + semantic negatives.
// Quota-abort (exit 85, short), exit-0 sub-threshold, claude-baseline, and covered
// (source=events_result) launches must all stay silent — no false positives.
func TestC1005_002_tripwire_stays_silent_on_non_signals(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_(QuotaAbortShortLaunch|Exit0ShortDuration|ClaudeBaseline|CoveredNonClaude)_Silent$",
		"TestRecordTokenUsage_Tripwire_QuotaAbortShortLaunch_Silent",
		"TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent",
		"TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent",
		"TestRecordTokenUsage_Tripwire_CoveredNonClaude_Silent")
}

// TestC1005_003_tripwire_fail_open_when_cycle_absent — EDGE. When the cycle is not
// derivable from the workspace path, the tripwire still fires (naming CLI+agent),
// degrading gracefully rather than erroring or suppressing the warning.
func TestC1005_003_tripwire_fail_open_when_cycle_absent(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen$",
		"TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen")
}

// TestC1005_004_tripwire_record_field_is_queryable — AC4 (Task 2). The
// llm-calls.ndjson record carries a queryable "tripwire" boolean — true for the
// escalating launch, false for the quota-abort launch — so the future
// tokens-report CLI can select tripwire records without re-parsing stderr.
func TestC1005_004_tripwire_record_field_is_queryable(t *testing.T) {
	runBridgeTest(t,
		"^TestRecordTokenUsage_TripwireRecord_FieldFlips$",
		"TestRecordTokenUsage_TripwireRecord_FieldFlips")
}
