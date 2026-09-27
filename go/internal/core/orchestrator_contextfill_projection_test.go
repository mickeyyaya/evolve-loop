package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

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
			if got.Phase != "build" || got.Verdict != "PASS" {
				t.Errorf("timing entry damaged by the degrade path: %+v", got)
			}
		})
	}
}

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
