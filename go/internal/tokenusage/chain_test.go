package tokenusage

// chain_test.go — RED contract for token-telemetry S2 (collector-chain;
// inbox token-telemetry-s2-collector-chain, weight 0.94; scout-report Task 1).
//
// The collector chain composes several usage sources in fidelity order —
// transcript > eventsResult > scrollbackPeak — and returns the FIRST non-empty
// tier, recording which source produced the figure. Every symbol these tests
// reference (Chain, Collector, TranscriptCollector, EventsResultCollector,
// ScrollbackPeakCollector, SourceEventsResult, SourceScrollbackPeak) is
// undefined today, so package tokenusage fails to compile — the intended RED
// signal (identical strategy to scanner_test.go's S1 contract). Builder
// implements the chain against this contract; DO NOT modify these tests.
//
// Contract summary the Builder must satisfy:
//   - A Collector is `func() Result`. It returns the zero Result (Source ==
//     SourceNone) when it has no data — the chain treats SourceNone as "empty".
//   - Chain(collectors...) runs them in the given (fidelity) order and returns
//     the first Result whose Source != SourceNone; all-empty yields SourceNone.
//   - EventsResultCollector(logPath) reuses the SAME *-events.ndjson result-
//     envelope extraction as cyclecost.parseEventsLog (shared func, no
//     duplication): it must recover the exact token counts cyclecost would.
//   - ScrollbackPeakCollector(pane) wraps panestream.ExtractResponseTokens as
//     an OUTPUT-ONLY floor: Usage.Output == ExtractResponseTokens(pane) and the
//     input/cache fields it cannot know stay zero.
//   - TranscriptCollector(root, w) wraps ScanConfigRoot (highest fidelity tier).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestCollectorChain_FidelityOrderFirstNonEmptyWins — AC-1 (design-doc-named
// RED test). An empty highest tier falls through to the next tier; the first
// NON-empty tier wins and its source is recorded in the output. Uses collector
// literals to isolate the chain ordering semantics from any adapter.
func TestCollectorChain_FidelityOrderFirstNonEmptyWins(t *testing.T) {
	empty := func() Result { return Result{Source: SourceNone} }
	events := func() Result {
		return Result{Usage: cyclestate.TokenUsage{Input: 500, Output: 50}, Source: SourceEventsResult}
	}
	scrollback := func() Result {
		return Result{Usage: cyclestate.TokenUsage{Output: 9}, Source: SourceScrollbackPeak}
	}

	// transcript tier empty → fall through to eventsResult (higher fidelity than
	// scrollbackPeak) which is non-empty and must win.
	got := Chain(empty, events, scrollback)

	if got.Source != SourceEventsResult {
		t.Fatalf("chain must return the first NON-EMPTY tier: want source %q, got %q", SourceEventsResult, got.Source)
	}
	if got.Usage.Input != 500 || got.Usage.Output != 50 {
		t.Errorf("winning tier usage not recorded: got %+v", got.Usage)
	}
}

// TestCollectorChain_AllEmptyYieldsNone — AC-1 negative / anti-no-op. When every
// tier is empty the chain must yield SourceNone with zero usage, NOT spuriously
// return the first tier. Forbids a degenerate `return collectors[0]()` impl.
func TestCollectorChain_AllEmptyYieldsNone(t *testing.T) {
	empty := func() Result { return Result{Source: SourceNone} }

	got := Chain(empty, empty, empty)

	if got.Source != SourceNone {
		t.Fatalf("all-empty chain must yield SourceNone, got %q", got.Source)
	}
	if got.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("all-empty chain must yield zero usage, got %+v", got.Usage)
	}
}

// TestChain_RealAdaptersPreferHigherFidelity — AC-1 binding on the REAL adapters:
// with an empty transcript root, the assembled chain of the three production
// collectors must prefer eventsResult over the lower-fidelity scrollbackPeak
// even though both carry data. This is what proves production wires the tiers in
// the transcript > eventsResult > scrollbackPeak order (not just that Chain
// works on literals).
func TestChain_RealAdaptersPreferHigherFidelity(t *testing.T) {
	dir := t.TempDir()
	// No projects/ subdir under dir → the transcript tier is empty.
	log := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, log, `{"kind":"result","data":{"tokens":{"in":700,"out":70,"cache_r":5,"cache_c":2}}}`+"\n")
	pane := "boot output\n↓ 5k tokens\n"

	got := Chain(
		TranscriptCollector(dir, Window{}),
		EventsResultCollector(log),
		ScrollbackPeakCollector(pane),
	)

	if got.Source != SourceEventsResult {
		t.Fatalf("with an empty transcript, eventsResult (higher fidelity than scrollbackPeak) must win: got source %q", got.Source)
	}
	if got.Usage.Input != 700 || got.Usage.Output != 70 {
		t.Errorf("eventsResult usage not propagated: got %+v", got.Usage)
	}
}

// TestEventsResultCollector_ExtractsResultEnvelopeTokens — AC-2 (shared
// extraction, no duplication). The eventsResult tier must recover the exact
// token counts cyclecost.parseEventsLog reads from the same result envelope. If
// the two share one extraction func they agree by construction; a divergent
// figure here is the "duplicated/forked parser" regression.
func TestEventsResultCollector_ExtractsResultEnvelopeTokens(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, log, `{"kind":"result","data":{"cost_usd":0.5,"tokens":{"in":1200,"out":340,"cache_r":80,"cache_c":16}}}`+"\n")

	got := EventsResultCollector(log)()

	if got.Source != SourceEventsResult {
		t.Fatalf("source must be %q, got %q", SourceEventsResult, got.Source)
	}
	want := cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}
	if got.Usage != want {
		t.Errorf("events-result tokens mismatch (must match cyclecost extraction): got %+v want %+v", got.Usage, want)
	}
}

// TestEventsResultCollector_NoResultEnvelopeIsEmpty — AC-2 edge. A log carrying
// no kind==result envelope yields an empty tier (SourceNone) so the chain falls
// through, rather than reporting a spurious zero-with-source.
func TestEventsResultCollector_NoResultEnvelopeIsEmpty(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, log, `{"kind":"progress","data":{}}`+"\n")

	got := EventsResultCollector(log)()

	if got.Source != SourceNone {
		t.Errorf("a log with no result envelope must be empty (SourceNone), got %q", got.Source)
	}
}

// TestScrollbackPeakCollector_OutputOnlyFloorFromPane — AC-3. The scrollbackPeak
// tier wraps panestream.ExtractResponseTokens as an output-only floor: Output
// equals the extracted peak, and the input/cache fields it cannot observe stay
// zero (it must not fabricate them).
func TestScrollbackPeakCollector_OutputOnlyFloorFromPane(t *testing.T) {
	pane := "some output\n↓ 12k tokens\nmore lines\n"

	got := ScrollbackPeakCollector(pane)()

	if got.Source != SourceScrollbackPeak {
		t.Fatalf("source must be %q, got %q", SourceScrollbackPeak, got.Source)
	}
	if got.Usage.Output != 12000 {
		t.Errorf("Output must equal ExtractResponseTokens(pane)=12000, got %d", got.Usage.Output)
	}
	if got.Usage.Input != 0 || got.Usage.CacheRead != 0 || got.Usage.CacheWrite != 0 {
		t.Errorf("scrollbackPeak is an output-only floor; non-output fields must be zero, got %+v", got.Usage)
	}
}

// TestScrollbackPeakCollector_NoTokensIsEmpty — AC-3 edge. A pane with no
// "↓ N tokens" marker yields an empty tier (peak 0 → SourceNone).
func TestScrollbackPeakCollector_NoTokensIsEmpty(t *testing.T) {
	got := ScrollbackPeakCollector("no token marker in this pane")()

	if got.Source != SourceNone {
		t.Errorf("a pane with no token marker must be empty (SourceNone), got source %q", got.Source)
	}
}

// TestTranscriptCollector_EmptyRootFallsThroughAsNone — AC-1 (transcript tier).
// The highest-fidelity tier wraps ScanConfigRoot; a root with no projects/
// directory is empty, so the collector reports SourceNone and the chain falls
// through to the next tier.
func TestTranscriptCollector_EmptyRootFallsThroughAsNone(t *testing.T) {
	got := TranscriptCollector(t.TempDir(), Window{})()

	if got.Source != SourceNone {
		t.Errorf("transcript collector over an empty root must be empty (SourceNone), got %q", got.Source)
	}
}

// writeFile is a tiny helper so the RED contract stays self-contained.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
