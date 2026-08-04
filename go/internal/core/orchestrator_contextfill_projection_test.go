// orchestrator_contextfill_projection_test.go — cycle-1271 RED tests for the
// contextfill → phase-timing wiring (task wire-contextfill-into-phasetiming-entry).
//
// These are the WIRING PROOF: they drive the real production chokepoint,
// (*Orchestrator).recordPhaseOutcome — the ADR-0044 C1 single writer of
// phase-timing.json, the exact seam token-telemetry S4 was wired into — and
// assert the derived fill ratio lands on the timing entry it emits. A test that
// called contextfill.FillRatio directly would pass on dead code and prove
// nothing; these fail until a production path actually derives the ratio.
//
// Tier provenance. PhaseOutcome carries no abstract tier field, but
// ResolvedModel already IS the tier the terminal attempt ran at
// (internal/phases/runner/runner.go:854-859 sets it from tieredRes.Tier,
// falling back to the concrete model id only in the empty-candidates edge case).
// So contextfill.WindowSizeForTier(out.ResolvedModel) is the honest lookup: a
// canonical tier yields the 200k window, anything else yields 0 →
// contextfill.ErrInvalidWindow → both fields stay zero. No tier is ever invented.
package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// recordOne drives the C1 chokepoint once and returns the timing entry it
// appended — the production-path helper every test below shares.
func recordOne(t *testing.T, out recovery.PhaseOutcome) phaseTimingEntry {
	t.Helper()
	o := NewOrchestrator(nil, nil, nil)
	var result CycleResult
	var timings []phaseTimingEntry
	o.recordPhaseOutcome(&result, &timings, t.TempDir(), out)
	if len(timings) != 1 {
		t.Fatalf("recordPhaseOutcome appended %d timing entries, want 1", len(timings))
	}
	return timings[0]
}

// TestRecordPhaseOutcome_ProjectsContextFillWhenTierResolvable is the crux: a
// phase that ran at a canonical tier and burned 190k of its 200k window must be
// recorded with the EXACT contextfill.FillRatio value and flagged hot.
func TestRecordPhaseOutcome_ProjectsContextFillWhenTierResolvable(t *testing.T) {
	t.Parallel()
	tokens := cyclestate.TokenUsage{Input: 120_000, Output: 20_000, CacheRead: 45_000, CacheWrite: 5_000}
	got := recordOne(t, recovery.PhaseOutcome{
		Phase:         "build",
		Verdict:       "PASS",
		DurationMS:    1000,
		AttemptCount:  1,
		ResolvedModel: "deep",
		Tokens:        tokens,
	})

	want, err := contextfill.FillRatio(tokens, contextfill.WindowSizeForTier("deep"))
	if err != nil {
		t.Fatalf("fixture is not tier-resolvable — test setup error: %v", err)
	}
	if got.ContextFillRatio != want {
		t.Errorf("ContextFillRatio = %v, want %v (exactly contextfill.FillRatio, not a re-derived approximation)", got.ContextFillRatio, want)
	}
	if !got.ContextWindowHot {
		t.Errorf("ContextWindowHot = false, want true (ratio %v is at/above contextfill.HotThreshold %v)", got.ContextFillRatio, contextfill.HotThreshold)
	}
}

// TestRecordPhaseOutcome_ColdPhaseIsNotFlaggedHot is the precision guard: the
// wiring must not degenerate into "always hot". A phase that used a fifth of its
// window records a real ratio and ContextWindowHot=false.
func TestRecordPhaseOutcome_ColdPhaseIsNotFlaggedHot(t *testing.T) {
	t.Parallel()
	tokens := cyclestate.TokenUsage{Input: 40_000}
	got := recordOne(t, recovery.PhaseOutcome{
		Phase:         "scout",
		Verdict:       "PASS",
		AttemptCount:  1,
		ResolvedModel: "balanced",
		Tokens:        tokens,
	})

	want, err := contextfill.FillRatio(tokens, contextfill.WindowSizeForTier("balanced"))
	if err != nil {
		t.Fatalf("fixture is not tier-resolvable — test setup error: %v", err)
	}
	if got.ContextFillRatio != want {
		t.Errorf("ContextFillRatio = %v, want %v", got.ContextFillRatio, want)
	}
	if got.ContextWindowHot {
		t.Errorf("ContextWindowHot = true for a %.2f-full window — the wiring must not flag every phase hot", got.ContextFillRatio)
	}
}

// TestRecordPhaseOutcome_UnresolvableTierLeavesContextFillZero is the negative
// test — the strongest anti-fabrication signal. When no canonical tier is
// resolvable (a concrete model id, an empty ResolvedModel), contextfill returns
// ErrInvalidWindow and the chokepoint must leave BOTH fields zero: absent, never
// a guessed window, never a propagated error that would abort the record.
func TestRecordPhaseOutcome_UnresolvableTierLeavesContextFillZero(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		resolvedModel string
	}{
		{"concrete model id, not a tier", "claude-opus-5"},
		{"empty provenance (legacy / advisor-less path)", ""},
		{"unknown tier string", "turbo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := recordOne(t, recovery.PhaseOutcome{
				Phase:         "build",
				Verdict:       "PASS",
				AttemptCount:  1,
				ResolvedModel: tc.resolvedModel,
				Tokens:        cyclestate.TokenUsage{Input: 190_000, Output: 5_000},
			})
			if got.ContextFillRatio != 0 {
				t.Errorf("ContextFillRatio = %v for ResolvedModel=%q, want 0 — an unknown window must never yield a fabricated ratio", got.ContextFillRatio, tc.resolvedModel)
			}
			if got.ContextWindowHot {
				t.Errorf("ContextWindowHot = true for ResolvedModel=%q, want false — unknown fill is not a hot claim", tc.resolvedModel)
			}
			// The rest of the record must be unaffected: the degrade is per-field,
			// never a dropped or truncated timing entry.
			if got.Phase != "build" || got.Verdict != "PASS" {
				t.Errorf("timing entry damaged by the degrade path: %+v", got)
			}
		})
	}
}

// TestRecordPhaseOutcome_ZeroTokensRecordsZeroFillNotHot is the edge case: a
// tier-resolvable phase that reported no tokens (a legacy or headless path) has
// a genuine 0.0 fill and must not be flagged hot.
func TestRecordPhaseOutcome_ZeroTokensRecordsZeroFillNotHot(t *testing.T) {
	t.Parallel()
	got := recordOne(t, recovery.PhaseOutcome{
		Phase:         "audit",
		Verdict:       "PASS",
		AttemptCount:  1,
		ResolvedModel: "top",
	})
	if got.ContextFillRatio != 0 {
		t.Errorf("ContextFillRatio = %v, want 0 for zero token usage", got.ContextFillRatio)
	}
	if got.ContextWindowHot {
		t.Errorf("ContextWindowHot = true for zero token usage, want false")
	}
}

// TestPhaseOutcomeFrom_CarriesTierProvenanceAndTokens guards the INPUT half of
// the wiring: the derivation is only possible because phaseOutcomeFrom copies
// both ResolvedModel (the tier) and Tokens off the PhaseResponse. Pre-existing
// GREEN — a regression fence, so a future refactor cannot silently starve the
// fill derivation of its inputs.
func TestPhaseOutcomeFrom_CarriesTierProvenanceAndTokens(t *testing.T) {
	t.Parallel()
	tokens := cyclestate.TokenUsage{Input: 7, Output: 3}
	out := phaseOutcomeFrom(Phase("build"), PhaseResponse{
		Verdict:       "PASS",
		ResolvedModel: "deep",
		ModelSource:   "advisor",
		Tokens:        tokens,
	}, 1, "", "2026-08-04T00:00:00Z")
	if out.ResolvedModel != "deep" {
		t.Errorf("PhaseOutcome.ResolvedModel = %q, want %q — the tier provenance the fill derivation reads", out.ResolvedModel, "deep")
	}
	if out.Tokens != tokens {
		t.Errorf("PhaseOutcome.Tokens = %+v, want %+v", out.Tokens, tokens)
	}
}
