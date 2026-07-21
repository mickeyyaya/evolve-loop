//go:build acs

// Package cycle1025 materializes the cycle-1025 acceptance criteria for the sole
// fleet lane this cycle is pinned to: adr0072-s5-task-quarantine (weight 0.92).
// Per R9.3 no predicate here binds to any other lane's item.
//
// Task nature: LANDING. The ADR-0072 S5 task-quarantine feature is already
// implemented, CI-green, and CANONICAL in PR #346 (branch cycle-21f9f7ae-1019)
// but is UNLANDED — `grep -rl ShouldQuarantine go/` on this cycle's tree returns
// nothing. The Builder's deliverable is to rebase PR #346 onto current main
// (preserving the cycle-1018..1023 dossiers the stale branch would delete) and
// land it. When it lands, the API + tests below arrive with it:
//
//	internal/inboxmover:  ShouldQuarantine, ReleaseFromQuarantine,
//	                      ReleaseCycleProcessingWithQuarantine (+ bumpFailureCount)
//	cmd/evolve:           runInboxQuarantine (operator surface), isTaskLevelFailure
//	                      (loop failure-drain classifier), and the runLoop drain
//	                      call-site swapped from ReleaseCycleProcessing to
//	                      ReleaseCycleProcessingWithQuarantine.
//
// Predicate strategy — behavioural-via-subprocess (the cycle-987 / cycle-563
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1 pkg`
// over the DEFAULT build suite (no -tags) for the tests that land WITH PR #346
// and requires that every named test printed a `--- PASS: <name>` line. This
// genuinely exercises the landed system-under-test (the real functions run
// against real temp-dir inbox state):
//
//   - RED now: the feature is unlanded, so `go test -run` matches no test and
//     exits 0 with "no tests to run" — NO `--- PASS:` line → the predicate fails.
//   - GREEN only once PR #346 lands: every named test exists AND passes in the
//     default suite.
//   - It ALSO fails (correctly) if a landed test is hidden behind a build tag the
//     default suite skips, or regresses.
//
// A source-grep (acsassert.FileContains) predicate is deliberately AVOIDED as the
// load-bearing assertion (the cycle-85 degenerate-predicate ban): a magic-string
// match on production source would pass the moment the symbol appears, even if
// the behaviour were broken or the call-site never wired. Asserting on the
// `--- PASS:` line (not merely a zero exit code) is essential: `go test -run` on
// a pattern that matches no test exits 0, so absence would otherwise false-GREEN.
package cycle1025

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	inboxmoverPkg = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	cmdEvolvePkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed a
// `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate
// always exercises current source. Asserting on the PASS line (not merely a zero
// exit code) is essential: `go test -run` on a pattern that matches no test exits
// 0 with "no tests to run", so a still-missing binding test would false-GREEN.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 means the subprocess never launched (toolchain/module resolution
		// failure) — a genuine harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing — feature unlanded, failing, or hidden behind a build tag the "+
				"default suite skips). exit=%d\ncombined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC1025_001_TaskQuarantineDrainBehaviour — AC1, AC2, AC3, AC4 + operator
// escape hatch. Drives the landed inboxmover quarantine surface end-to-end
// against real temp-dir inbox state:
//
//   - AtCeiling…Quarantines (AC1): a task failing task_retry_ceiling times routes
//     to .evolve/inbox/quarantine/ and is GONE from the inbox root — quarantine/
//     is a flat sibling dir LoadDir never walks, so the item is invisible to the
//     next cycle's triage.
//   - BelowCeilingReleasesAndCounts (AC2): below the ceiling the item returns to
//     the inbox root with an incremented failure_count — other (non-poison) items
//     keep flowing; only a task AT the ceiling quarantines.
//   - AtCeiling… also asserts the quarantined item's failure_count == 2 (AC3): the
//     quarantined item carries its failure diagnostic.
//   - SystemLevelNeverQuarantines (AC4): a system-level failure past the ceiling
//     still releases to root — the S3 floor halt takes precedence over task
//     quarantine.
//   - ReleaseFromQuarantine_RoundTrips: the operator escape hatch returns an item
//     to the inbox root and resets its failure budget to 0.
//   - ShouldQuarantine_NamesThePredicate: the pure S5 decision over its whole
//     contract (at/over ceiling for task-level only; system-level never; ceiling
//     <= 0 disables).
//
// These land in internal/inboxmover/inboxmover_quarantine_test.go with PR #346
// (default suite, no build tag).
func TestC1025_001_TaskQuarantineDrainBehaviour(t *testing.T) {
	assertDefaultSuiteTestsPass(t, inboxmoverPkg,
		"TestReleaseWithQuarantine_AtCeilingQuarantines",          // AC1 + AC3 (diagnostic count)
		"TestReleaseWithQuarantine_BelowCeilingReleasesAndCounts", // AC2 (others keep flowing)
		"TestReleaseWithQuarantine_SystemLevelNeverQuarantines",   // AC4 (S3 precedence)
		"TestReleaseFromQuarantine_RoundTrips",                    // operator escape hatch
		"TestShouldQuarantine_NamesThePredicate",                  // pure S5 decision contract
	)
}

// TestC1025_002_LoopDrainAndCLIWiring — AC6 (no inert API) + the operator CLI
// surface. Drives the landed cmd/evolve wiring in the DEFAULT suite:
//
//   - TestIsTaskLevelFailure_AllClassifications: the loop failure-drain
//     classifier that computes the `systemLevel` input runLoop passes to
//     ReleaseCycleProcessingWithQuarantine. Build/audit/ship-gate-config classify
//     as task-level (quarantine-eligible); transient-infra / system / kernel
//     breaches classify as system-level (never quarantine — AC4/AC6 in the
//     composed production path).
//   - TestRunInbox_DispatchesQuarantine: proves `evolve inbox quarantine` is wired
//     into the inbox root command's dispatch — the operator surface is NOT
//     orphaned.
//   - TestRunInboxQuarantine_{List…,Release…,UsageAndUnknown}: the list/release/
//     usage behaviour of the operator surface itself.
//
// These land in cmd/evolve/cmd_inbox_quarantine_test.go with PR #346 (default
// suite, no build tag). Together they demonstrate the failure-drain call-site
// invokes the quarantine path (the classifier it feeds is live and correct) and
// the CLI escape hatch is reachable — closing AC6's "no inert API" bar
// behaviourally, not by a source grep.
func TestC1025_002_LoopDrainAndCLIWiring(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdEvolvePkg,
		"TestIsTaskLevelFailure_AllClassifications", // AC6/AC4 loop-drain classifier
		"TestRunInbox_DispatchesQuarantine",         // CLI dispatch wired, not orphaned
		"TestRunInboxQuarantine_ListEmptyPopulatedAndJSON",
		"TestRunInboxQuarantine_ReleaseSuccessAndMissing",
		"TestRunInboxQuarantine_UsageAndUnknown",
	)
}
