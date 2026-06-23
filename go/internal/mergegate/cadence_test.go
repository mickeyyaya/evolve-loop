package mergegate_test

// DecideCadence — the merge-cadence advisor. These tests pin two things:
//  1. DEFER-WINS: any single safety violation (audit not passed, CI not green,
//     ledger unverified, conflicts present, severity blocks) forces Fire=false /
//     Cadence=defer regardless of how much progress has accumulated. This is the
//     deterministic floor the LLM gate can only ever be MORE conservative than.
//  2. The cadence Strategy when the floor is clear: feature-complete (all waves
//     done) > anti-starvation flush > per-wave (batch<=1) > batch-threshold > keep
//     accumulating.
// Pure + table-driven: zero I/O, same input → same output.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/mergegate"
)

// clear is the all-green safety baseline: every floor predicate satisfied, so the
// cadence Strategy (not the floor) decides. Cases override only what they probe.
func clearInput() mergegate.DecisionInput {
	return mergegate.DecisionInput{
		WavesDone:       1,
		WavesTotal:      4,
		PendingWaves:    1,
		ChurnLOC:        100,
		CarryoverAgeMax: 0,
		AuditPassed:     true,
		CIGreen:         true,
		LedgerVerified:  true,
		SeverityBlocks:  false,
		Conflicts:       0,
	}
}

func perWave() mergegate.Thresholds {
	return mergegate.Thresholds{BatchWaveCount: 1, BatchChurnLOC: 800, CarryoverStallCycles: 8}
}

func TestDecideCadence_DeferWins(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*mergegate.DecisionInput)
	}{
		{"audit-not-passed", func(in *mergegate.DecisionInput) { in.AuditPassed = false }},
		{"ci-not-green", func(in *mergegate.DecisionInput) { in.CIGreen = false }},
		{"ledger-unverified", func(in *mergegate.DecisionInput) { in.LedgerVerified = false }},
		{"conflicts-present", func(in *mergegate.DecisionInput) { in.Conflicts = 2 }},
		{"severity-blocks", func(in *mergegate.DecisionInput) { in.SeverityBlocks = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := clearInput()
			in.WavesDone, in.WavesTotal = 4, 4 // even "all waves done" must not override the floor
			tc.mutate(&in)
			got := mergegate.DecideCadence(in, perWave())
			if got.Fire || got.Cadence != mergegate.CadenceDefer {
				t.Fatalf("DecideCadence(%s) = %+v, want Fire=false Cadence=defer (defer-wins)", tc.name, got)
			}
		})
	}
}

func TestDecideCadence_Strategy(t *testing.T) {
	cases := []struct {
		name string
		in   mergegate.DecisionInput
		th   mergegate.Thresholds
		want mergegate.Decision
	}{
		{
			name: "all-waves-done-feature-complete",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.WavesDone, in.WavesTotal = 4, 4 }),
			th:   mergegate.Thresholds{BatchWaveCount: 9, BatchChurnLOC: 800, CarryoverStallCycles: 8}, // high batch must NOT suppress feature-complete
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadenceFeatureComplete},
		},
		{
			name: "per-wave-when-batch-one",
			in:   clearInput(),
			th:   perWave(),
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadencePerWave},
		},
		{
			// WavesTotal==0 means "unknown/non-campaign": the feature-complete branch
			// must NOT fire (0 >= 0 is true but total==0 guards it off), and per-wave
			// cadence applies.
			name: "unknown-total-does-not-feature-complete",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.WavesDone, in.WavesTotal = 0, 0 }),
			th:   perWave(),
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadencePerWave},
		},
		{
			name: "batch-accumulating-defers",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.PendingWaves = 1 }),
			th:   mergegate.Thresholds{BatchWaveCount: 3, BatchChurnLOC: 800, CarryoverStallCycles: 8},
			want: mergegate.Decision{Fire: false, Cadence: mergegate.CadenceDefer},
		},
		{
			name: "batch-threshold-reached",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.PendingWaves = 3 }),
			th:   mergegate.Thresholds{BatchWaveCount: 3, BatchChurnLOC: 800, CarryoverStallCycles: 8},
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadenceBatched},
		},
		{
			name: "batch-churn-override-fires-early",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.PendingWaves = 1; in.ChurnLOC = 1200 }),
			th:   mergegate.Thresholds{BatchWaveCount: 3, BatchChurnLOC: 800, CarryoverStallCycles: 8},
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadenceBatched},
		},
		{
			name: "anti-starvation-flush",
			in:   mut(clearInput(), func(in *mergegate.DecisionInput) { in.PendingWaves = 1; in.CarryoverAgeMax = 8 }),
			th:   mergegate.Thresholds{BatchWaveCount: 5, BatchChurnLOC: 800, CarryoverStallCycles: 8},
			want: mergegate.Decision{Fire: true, Cadence: mergegate.CadenceBatched},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergegate.DecideCadence(tc.in, tc.th)
			if got.Fire != tc.want.Fire || got.Cadence != tc.want.Cadence {
				t.Fatalf("DecideCadence() = %+v, want Fire=%v Cadence=%v (reason=%q)", got, tc.want.Fire, tc.want.Cadence, got.Reason)
			}
			if got.Reason == "" {
				t.Errorf("DecideCadence() Reason is empty; every decision must explain itself")
			}
		})
	}
}

// mut applies f to a copy of in and returns it — keeps the table literals terse.
func mut(in mergegate.DecisionInput, f func(*mergegate.DecisionInput)) mergegate.DecisionInput {
	f(&in)
	return in
}
