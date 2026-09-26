package tokenusage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestCollectorChain_FidelityOrderFirstNonEmptyWins(t *testing.T) {
	empty := func() Result { return Result{Source: SourceNone} }
	events := func() Result {
		return Result{Usage: cyclestate.TokenUsage{Input: 500, Output: 50}, Source: SourceEventsResult}
	}
	scrollback := func() Result {
		return Result{Usage: cyclestate.TokenUsage{Output: 9}, Source: SourceScrollbackPeak}
	}

	got := Chain(empty, events, scrollback)

	if got.Source != SourceEventsResult {
		t.Fatalf("chain must return the first NON-EMPTY tier: want source %q, got %q", SourceEventsResult, got.Source)
	}
	if got.Usage.Input != 500 || got.Usage.Output != 50 {
		t.Errorf("winning tier usage not recorded: got %+v", got.Usage)
	}
}

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

func TestEventsResultCollector_NoResultEnvelopeIsEmpty(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "scout-events.ndjson")
	writeFile(t, log, `{"kind":"progress","data":{}}`+"\n")

	got := EventsResultCollector(log)()

	if got.Source != SourceNone {
		t.Errorf("a log with no result envelope must be empty (SourceNone), got %q", got.Source)
	}
}

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

func TestScrollbackPeakCollector_NoTokensIsEmpty(t *testing.T) {
	got := ScrollbackPeakCollector("no token marker in this pane")()

	if got.Source != SourceNone {
		t.Errorf("a pane with no token marker must be empty (SourceNone), got source %q", got.Source)
	}
}

func TestTranscriptCollector_EmptyRootFallsThroughAsNone(t *testing.T) {
	got := TranscriptCollector(t.TempDir(), Window{})()

	if got.Source != SourceNone {
		t.Errorf("transcript collector over an empty root must be empty (SourceNone), got %q", got.Source)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
