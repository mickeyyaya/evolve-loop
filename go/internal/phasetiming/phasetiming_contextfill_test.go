// phasetiming_contextfill_test.go — cycle-1271 RED tests for the contextfill
// telemetry wiring (tasks wire-contextfill-into-phasetiming-entry and
// contextfill-rollup-hot-phase-summary).
//
// Entry gains ContextFillRatio/ContextWindowHot — the durable per-phase record
// of how full the model's context window got — and Summary gains the cycle-level
// twin (HotPhaseCount/HotPhases), exactly mirroring the Tokens (S4) / TotalTokens
// (S6) precedent already in this file's package.
//
// Legacy compat is the same contract the Tokens field documents: a timing log
// written before these fields existed must still parse, leaving both at their
// zero value — "absent", never a fabricated ratio, never an error.
package phasetiming

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
)

// TestEntry_ContextFillFieldsRoundTripJSON pins the on-disk contract (ADR-0044
// C1): the two new fields marshal under the snake_case keys the dossier and the
// `evolve cycle timing` CLI will read, and survive a marshal→unmarshal round
// trip with their values intact.
func TestEntry_ContextFillFieldsRoundTripJSON(t *testing.T) {
	t.Parallel()
	want := Entry{
		Phase:            "build",
		DurationMS:       1000,
		Verdict:          "PASS",
		ContextFillRatio: 0.91,
		ContextWindowHot: true,
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal Entry: %v", err)
	}
	for _, key := range []string{`"context_fill_ratio"`, `"context_window_hot"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshalled Entry missing key %s — on-disk contract not emitted:\n%s", key, raw)
		}
	}
	var got Entry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal Entry: %v", err)
	}
	if got.ContextFillRatio != want.ContextFillRatio {
		t.Errorf("round-tripped ContextFillRatio = %v, want %v", got.ContextFillRatio, want.ContextFillRatio)
	}
	if got.ContextWindowHot != want.ContextWindowHot {
		t.Errorf("round-tripped ContextWindowHot = %v, want %v", got.ContextWindowHot, want.ContextWindowHot)
	}
}

// TestEntry_LegacyLogWithoutContextFillParsesToZero is the degrade contract:
// a phase-timing.json written before this cycle must still parse (never an
// error), leaving both new fields zero — absent, not fabricated.
func TestEntry_LegacyLogWithoutContextFillParsesToZero(t *testing.T) {
	t.Parallel()
	legacy := `[{"phase":"scout","duration_ms":500,"verdict":"PASS","cost_usd":0.1,"attempt_count":1,
	             "tokens":{"input":10,"output":5,"cache_read":0,"cache_write":0}}]`
	var entries []Entry
	if err := json.Unmarshal([]byte(legacy), &entries); err != nil {
		t.Fatalf("legacy timing log must parse: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].ContextFillRatio != 0 {
		t.Errorf("legacy ContextFillRatio = %v, want 0 (absent, never fabricated)", entries[0].ContextFillRatio)
	}
	if entries[0].ContextWindowHot {
		t.Errorf("legacy ContextWindowHot = true, want false (absent, never fabricated)")
	}
	if entries[0].Phase != "scout" {
		t.Errorf("legacy Phase = %q, want scout (the rest of the record must be unaffected)", entries[0].Phase)
	}
}

// TestEntry_ContextFillOmittedWhenUnknown is the omitempty half of the degrade
// contract: an entry whose tier was unresolvable emits NEITHER key, so a reader
// can tell "unknown" from a genuine 0.0 fill.
func TestEntry_ContextFillOmittedWhenUnknown(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Entry{Phase: "audit", Verdict: "PASS"})
	if err != nil {
		t.Fatalf("marshal Entry: %v", err)
	}
	for _, key := range []string{`"context_fill_ratio"`, `"context_window_hot"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("zero-valued Entry emitted %s — unknown fill must be ABSENT, not 0:\n%s", key, raw)
		}
	}
}

// hotFixture is the shared Rollup fixture: one hot phase, one cold phase, one
// tier-unresolved phase (both fields zero).
func hotFixture() []Entry {
	return []Entry{
		{Phase: "scout", DurationMS: 100, Verdict: "PASS", ContextFillRatio: 0.20},
		{Phase: "build", DurationMS: 200, Verdict: "PASS", ContextFillRatio: 0.93, ContextWindowHot: true},
		{Phase: "audit", DurationMS: 300, Verdict: "PASS"}, // tier unresolved — ratio absent
	}
}

// TestRollup_HotPhaseCountAndNames is the task-2 crux: the cycle-level rollup
// names WHICH phases ran hot, and a tier-unresolved entry (ratio 0) is never
// counted — absence is not coldness fabricated into a claim, but it is likewise
// never reported as hot.
func TestRollup_HotPhaseCountAndNames(t *testing.T) {
	t.Parallel()
	s := Rollup(hotFixture())
	if s.HotPhaseCount != 1 {
		t.Errorf("HotPhaseCount = %d, want 1 (only build exceeded the threshold)", s.HotPhaseCount)
	}
	if len(s.HotPhases) != 1 || s.HotPhases[0] != "build" {
		t.Errorf("HotPhases = %v, want [build]", s.HotPhases)
	}
	for _, p := range s.HotPhases {
		if p == "audit" {
			t.Errorf("HotPhases contains %q — a tier-unresolved entry (ratio 0) must never be reported hot", p)
		}
	}
	// The rollup must not disturb the existing duration aggregate.
	if s.TotalMS != 600 || s.PhaseCount != 3 {
		t.Errorf("TotalMS/PhaseCount = %d/%d, want 600/3 — the hot rollup must be additive", s.TotalMS, s.PhaseCount)
	}
}

// TestRollup_NoHotPhasesReportsEmpty is the anti-no-op negative: a cycle where
// nothing ran hot must report zero and an empty list, not a fabricated entry.
func TestRollup_NoHotPhasesReportsEmpty(t *testing.T) {
	t.Parallel()
	s := Rollup([]Entry{
		{Phase: "scout", DurationMS: 10, ContextFillRatio: 0.10},
		{Phase: "build", DurationMS: 20, ContextFillRatio: 0.50},
		{Phase: "audit", DurationMS: 30},
	})
	if s.HotPhaseCount != 0 {
		t.Errorf("HotPhaseCount = %d, want 0", s.HotPhaseCount)
	}
	if len(s.HotPhases) != 0 {
		t.Errorf("HotPhases = %v, want empty", s.HotPhases)
	}
}

// TestRollup_HotThresholdBoundaryIsInclusive pins the rollup to
// contextfill.IsHot's documented inclusive boundary rather than a hand-rolled
// strictly-greater comparison — an entry sitting exactly at HotThreshold is hot.
func TestRollup_HotThresholdBoundaryIsInclusive(t *testing.T) {
	t.Parallel()
	s := Rollup([]Entry{
		{Phase: "build", DurationMS: 10, ContextFillRatio: contextfill.HotThreshold, ContextWindowHot: true},
		{Phase: "scout", DurationMS: 10, ContextFillRatio: contextfill.HotThreshold - 0.01},
	})
	if s.HotPhaseCount != 1 {
		t.Errorf("HotPhaseCount = %d, want 1 — the HotThreshold boundary is INCLUSIVE (contextfill.IsHot)", s.HotPhaseCount)
	}
	if len(s.HotPhases) != 1 || s.HotPhases[0] != "build" {
		t.Errorf("HotPhases = %v, want [build]", s.HotPhases)
	}
}

// TestRollup_EmptyEntriesNoHotFields is the edge case: no entries at all must
// produce a zero-valued hot rollup without panicking.
func TestRollup_EmptyEntriesNoHotFields(t *testing.T) {
	t.Parallel()
	s := Rollup(nil)
	if s.HotPhaseCount != 0 || len(s.HotPhases) != 0 {
		t.Errorf("Rollup(nil) hot fields = %d/%v, want 0/empty", s.HotPhaseCount, s.HotPhases)
	}
}
