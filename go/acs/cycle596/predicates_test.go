//go:build acs

// Package cycle596 materialises the cycle-596 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - token-telemetry-s2-collector-chain (inbox 0.94): a usage collector chain
//     in go/internal/tokenusage composes tiers in fidelity order
//     (transcript > eventsResult > scrollbackPeak), returns the first NON-empty
//     tier and records its source; the eventsResult tier reuses the SAME
//     *-events.ndjson result-envelope extraction as cyclecost.parseEventsLog (no
//     duplication); the scrollbackPeak tier wraps panestream.ExtractResponseTokens
//     as an output-only floor.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…594 precedent).
// Each predicate shells `go test -run` over the internal-package unit tests in
// go/internal/tokenusage/chain_test.go that materialise the criteria — every
// test exercises the real Chain / collector functions (their return value +
// recorded Source), none is a source grep. RED today: chain_test.go references
// the not-yet-implemented Chain/Collector API, so package tokenusage fails to
// compile and every `go test` below exits non-zero. Builder makes them GREEN by
// implementing the chain (do NOT modify the tests).
package cycle596

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	cyclecostPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or
// assertion failure in the target package surfaces as a non-zero exit — the RED
// signal a regression (or the not-yet-built feature) produces.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2) must flow through as
	// ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC596_001_FidelityOrderFirstNonEmptyWins — AC-1: the chain composes tiers in
// fidelity order and returns the first NON-empty tier with its source recorded,
// both on collector literals and on the real assembled adapters. Drives the
// design-doc-named RED test plus the real-adapter ordering test.
func TestC596_001_FidelityOrderFirstNonEmptyWins(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestCollectorChain_FidelityOrderFirstNonEmptyWins|TestChain_RealAdaptersPreferHigherFidelity")
	if !ok {
		t.Errorf("collector chain does not return the first non-empty tier in fidelity order (transcript>eventsResult>scrollbackPeak):\n%s", out)
	}
}

// TestC596_002_AllEmptyYieldsNone — AC-1 negative / anti-no-op: an all-empty
// chain must yield SourceNone with zero usage, not spuriously return the first
// tier. This is the strongest anti-no-op signal — it fails a degenerate
// `return collectors[0]()` implementation.
func TestC596_002_AllEmptyYieldsNone(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestCollectorChain_AllEmptyYieldsNone")
	if !ok {
		t.Errorf("all-empty chain does not yield SourceNone (degenerate 'first tier always wins' impl):\n%s", out)
	}
}

// TestC596_003_EventsResultReusesCyclecostExtraction — AC-2: the eventsResult tier
// recovers the exact token counts cyclecost.parseEventsLog reads from the same
// result envelope (shared extraction, no duplicated parser), and an envelope-less
// log is treated as empty. Drives the two EventsResultCollector tests.
func TestC596_003_EventsResultReusesCyclecostExtraction(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestEventsResultCollector_ExtractsResultEnvelopeTokens|TestEventsResultCollector_NoResultEnvelopeIsEmpty")
	if !ok {
		t.Errorf("eventsResult tier does not reuse the cyclecost result-envelope extraction (duplicated/forked parser or wrong tokens):\n%s", out)
	}
}

// TestC596_004_ScrollbackPeakOutputOnlyFloor — AC-3: the scrollbackPeak tier wraps
// panestream.ExtractResponseTokens as an output-only floor (Output == extracted
// peak, input/cache fields stay zero) and reports empty when the pane has no
// token marker. Drives the two ScrollbackPeakCollector tests.
func TestC596_004_ScrollbackPeakOutputOnlyFloor(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg,
		"TestScrollbackPeakCollector_OutputOnlyFloorFromPane|TestScrollbackPeakCollector_NoTokensIsEmpty")
	if !ok {
		t.Errorf("scrollbackPeak tier is not an output-only floor over ExtractResponseTokens:\n%s", out)
	}
}

// TestC596_005_CyclecostRegressionGreen — AC-4 (second half): extracting the
// shared result-envelope parser must not regress cyclecost — its full suite must
// stay green. Pre-existing GREEN today (regression guard): it fails only if the
// S2 refactor breaks cyclecost.parseEventsLog / SummarizeCycle.
func TestC596_005_CyclecostRegressionGreen(t *testing.T) {
	ok, out := runGoTest(t, cyclecostPkg, ".*")
	if !ok {
		t.Errorf("cyclecost suite regressed after the shared-extraction refactor:\n%s", out)
	}
}
