//go:build acs

// Package cycle593 materialises the cycle-593 acceptance criteria for the
// single triage-committed top_n task (triage-report.md `## top_n`; this
// cycle's fleet_scope assigns exactly one item — every other scout-selected
// candidate was routed to `## dropped` as out-of-scope-fleet, so no predicate
// binds to them here, per the AC-Materialization Contract's
// "predicates bind ONLY to triage-committed work" rule):
//
//   - token-telemetry-s1-transcript-scanner (inbox 0.95) — new leaf package
//     internal/tokenusage; RED now (package does not compile, every symbol
//     undefined). TestC593_001..004.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574
// precedent). Each predicate shells `go test -run` over the RED unit tests
// authored this cycle in internal/tokenusage. None is a source-grep — every
// one exercises the system under test and asserts on its result.
package cycle593

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict, so it fails the predicate loudly rather than being read as RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC593_001_TranscriptScanSumsUsageWithinWindow — AC1a (RED): sums
// per-message usage across a synthetic transcript into cyclestate.TokenUsage.
func TestC593_001_TranscriptScanSumsUsageWithinWindow(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_SumsUsageWithinWindow")
	if !ok {
		t.Errorf("internal/tokenusage transcript-sum scan missing or failing:\n%s", out)
	}
}

// TestC593_002_TranscriptScanDedupsStreamedUsage — AC1b (RED): streamed usage
// deltas sharing one message id must collapse to the last delta, not sum.
func TestC593_002_TranscriptScanDedupsStreamedUsage(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_DeduplicatesStreamedUsageByMessageID")
	if !ok {
		t.Errorf("internal/tokenusage streamed-usage dedup missing or failing:\n%s", out)
	}
}

// TestC593_003_TranscriptScanContentVerifiesConcurrentSessions — AC1c (RED):
// concurrent same-dir sessions must be disambiguated by content (unique
// artifact path), never by cwd match alone.
func TestC593_003_TranscriptScanContentVerifiesConcurrentSessions(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted")
	if !ok {
		t.Errorf("internal/tokenusage concurrent-session content verification missing or failing:\n%s", out)
	}
}

// TestC593_004_TranscriptScanMissingDirYieldsSourceNone — AC1d (RED): absent
// transcript directory is best-effort (SourceNone, no error), never fatal.
func TestC593_004_TranscriptScanMissingDirYieldsSourceNone(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestTranscriptScan_MissingDirYieldsSourceNone")
	if !ok {
		t.Errorf("internal/tokenusage missing-dir SourceNone handling missing or failing:\n%s", out)
	}
}
