package budgethistory

// budgethistory_tokens_test.go — S8 (token-telemetry) RED contract:
// Throughput gains MedianTokensPerCycle, the NATIVE-unit twin of
// MedianCycleDurationMS. The raw signal already exists per cycle as
// phasetiming.Rollup(entries).TotalTokens (S6); Collect must thread it through
// with the SAME median + absent-evidence discipline the duration path uses.
//
// Definition pinned here (the only ambiguity in the slice): "tokens for a
// cycle" = the GROSS sum of all four TokenUsage fields
// (Input+Output+CacheRead+CacheWrite). The fixtures below are deliberately
// chosen so an Input+Output-only implementation produces a DIFFERENT median and
// fails — the definition is asserted, not assumed.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// writeRunWithTokens materializes one cycle workspace whose phase-timing.json
// carries per-phase token usage alongside duration — the on-disk shape S4
// projects and S6 rolls up. Separate from writeRun so the existing duration/cost
// fixtures stay untouched (they are the zero-behavior-change regression pins).
func writeRunWithTokens(t *testing.T, root string, cycle int, durationMS int64, tokens []cyclestate.TokenUsage) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir run ws: %v", err)
	}
	entries := make([]phasetiming.Entry, 0, len(tokens))
	for i, tok := range tokens {
		entries = append(entries, phasetiming.Entry{
			Phase:      fmt.Sprintf("phase%d", i),
			DurationMS: durationMS / int64(len(tokens)),
			Verdict:    "PASS",
			Tokens:     tok,
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal timing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatalf("write timing: %v", err)
	}
}

// TestCollect_MedianTokensPerCycle is the verbatim S8 RED test named in
// docs/plans/token-telemetry-2026-07.md. Three cycles whose GROSS token totals
// are 220, 1110 and 2400 ⇒ median 1110.
//
// Discrimination: the same fixtures under an Input+Output-only definition total
// 110, 220 and 330 ⇒ median 220, so a cache-dropping implementation fails here
// rather than silently shipping a different metric.
func TestCollect_MedianTokensPerCycle(t *testing.T) {
	root := t.TempDir()
	// cycle 20: 100+10+0+0 = 110 in/out, +0 cache ⇒ gross 110... split across two
	// phases below to prove the roll-up sums PHASES, not just reads the first.
	writeRunWithTokens(t, root, 20, 600_000, []cyclestate.TokenUsage{
		{Input: 100, Output: 10, CacheRead: 0, CacheWrite: 0},
		{Input: 100, Output: 10, CacheRead: 0, CacheWrite: 0},
	}) // gross = 220
	writeRunWithTokens(t, root, 21, 600_000, []cyclestate.TokenUsage{
		{Input: 100, Output: 10, CacheRead: 1000, CacheWrite: 0},
	}) // gross = 1110
	writeRunWithTokens(t, root, 22, 600_000, []cyclestate.TokenUsage{
		{Input: 200, Output: 30, CacheRead: 2000, CacheWrite: 170},
	}) // gross = 2400

	got := Collect(root, []int{20, 21, 22})

	if got.SampleCount != 3 {
		t.Fatalf("SampleCount = %d, want 3", got.SampleCount)
	}
	if got.MedianTokensPerCycle != 1110 {
		t.Errorf("MedianTokensPerCycle = %d, want 1110 (gross median of 220/1110/2400; "+
			"220 would mean cache tokens were dropped)", got.MedianTokensPerCycle)
	}
	// The new field is ADDITIVE: the duration path must be untouched.
	if got.MedianCycleDurationMS != 600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 600000 (duration path unchanged)", got.MedianCycleDurationMS)
	}
}

// TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian is the absent-evidence
// (negative) axis: a legacy phase-timing.json written before the S4 Tokens field
// existed must yield MedianTokensPerCycle == 0 — "unknown", never a fabricated
// count — while still contributing its duration to the pace estimate. A
// implementation that back-fills tokens from cost or duration fails here.
func TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, 30, oneHour(), 0) // no Tokens field at all
	writeRun(t, root, 31, oneHour(), 0)

	got := Collect(root, []int{30, 31})

	if got.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want 2 (legacy cycles still carry duration)", got.SampleCount)
	}
	if got.MedianTokensPerCycle != 0 {
		t.Errorf("MedianTokensPerCycle = %d, want 0 (legacy log ⇒ absent, not fabricated)", got.MedianTokensPerCycle)
	}
	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000", got.MedianCycleDurationMS)
	}
}

// TestCollect_NoEvidenceYieldsZeroTokenMedian pins the empty-sample edge: no
// readable cycle at all ⇒ the zero-value Throughput, token field included.
func TestCollect_NoEvidenceYieldsZeroTokenMedian(t *testing.T) {
	got := Collect(t.TempDir(), []int{99})

	if got.SampleCount != 0 || got.MedianTokensPerCycle != 0 {
		t.Errorf("Collect(no evidence) = %+v, want zero-value Throughput", got)
	}
}
