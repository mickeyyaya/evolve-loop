package stageevidence_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	stageKey       = "regression_tia.stage"
	currentVersion = "version-current"
)

func linked(records []stageevidence.Record) []stageevidence.Record {
	out := make([]stageevidence.Record, 0, len(records))
	prev := map[stageevidence.ScopeKey]string{}
	for _, r := range records {
		sealed := stageevidence.Link(prev[r.ScopeKey()], r)
		out = append(out, sealed)
		prev[r.ScopeKey()] = sealed.Hash
	}
	return out
}

func decision(cycle, seq int, version string, fields map[string]string) stageevidence.Record {
	return stageevidence.Record{Key: stageKey, Version: version, Kind: stageevidence.KindDecision, ID: cycle, Seq: seq, Fields: fields}
}

func fixtureChain() stageevidence.Chain {
	return stageevidence.Chain{
		Records: linked([]stageevidence.Record{
			decision(1, 1, currentVersion, map[string]string{"would_skip": "true", "check": "lint"}),
			decision(2, 2, currentVersion, map[string]string{"would_stop": "true"}),
			decision(3, 3, currentVersion, map[string]string{"would_hold": "true", "item": "item-9"}),
			decision(4, 4, currentVersion, map[string]string{"would_stop": "false"}),
			decision(4, 5, currentVersion, map[string]string{"would_skip": "true", "check": "lint"}),
		}),
		Outcomes: []stageevidence.Outcome{
			{ID: 1, Failed: []string{"lint"}},
			{ID: 2, Shipped: true},
			{ID: 3, Routed: []string{"item-9"}},
			{ID: 4, Shipped: true},
		},
	}
}

var strictPromotion = gatecontract.Promotion{MinDecisions: 3}

type silentScorer struct{}

func (silentScorer) Name() string { return "silent" }

func (silentScorer) Classify(stageevidence.Record, stageevidence.Outcome) stageevidence.Class {
	return stageevidence.ClassNone
}

type memoryChain struct {
	chain stageevidence.Chain
	err   error
}

func (m memoryChain) ReadChain(context.Context, string) (stageevidence.Chain, error) {
	return m.chain, m.err
}

func wantCases() []stageevidence.Case {
	return []stageevidence.Case{
		{Class: stageevidence.ClassEscape, Cycle: 1, Seq: 1},
		{Class: stageevidence.ClassFalseStop, Cycle: 2, Seq: 2},
		{Class: stageevidence.ClassConflict, Cycle: 3, Seq: 3},
	}
}

func TestVerdictRecomputesFromFixtureChain(t *testing.T) {
	t.Parallel()
	chain := fixtureChain()
	got := stageevidence.Score(chain, currentVersion, strictPromotion, stageevidence.DefaultScorer())
	again := stageevidence.Score(chain, currentVersion, strictPromotion, stageevidence.DefaultScorer())
	if got.Ready {
		t.Error("a chain with an escape, a false stop and a conflict scored ready")
	}
	if got.Decisions != 5 || got.FalseStops != 1 || got.Escapes != 1 || got.Conflicts != 1 {
		t.Errorf("counts = decisions %d, false stops %d, escapes %d, conflicts %d; want 5, 1, 1, 1", got.Decisions, got.FalseStops, got.Escapes, got.Conflicts)
	}
	if !slices.Equal(got.Cases, wantCases()) {
		t.Errorf("cases = %+v, want %+v", got.Cases, wantCases())
	}
	if got.Version != currentVersion || got.FirstCycle != 1 || got.LastCycle != 4 {
		t.Errorf("version %q range %d to %d, want %q 1 to 4", got.Version, got.FirstCycle, got.LastCycle, currentVersion)
	}
	if got.ChainDigest != chain.Records[len(chain.Records)-1].Hash {
		t.Errorf("chain digest %q is not the hash of the last record", got.ChainDigest)
	}
	if got.Scorer != stageevidence.DefaultScorer().Name() {
		t.Errorf("scorer = %q, want %q", got.Scorer, stageevidence.DefaultScorer().Name())
	}
	if !slices.Equal(got.Cases, again.Cases) || got.ChainDigest != again.ChainDigest || got.Decisions != again.Decisions {
		t.Error("the verdict is not a pure function of the chain")
	}
}

func TestScorerMutationChangesGolden(t *testing.T) {
	t.Parallel()
	chain := fixtureChain()
	golden := stageevidence.Score(chain, currentVersion, strictPromotion, stageevidence.DefaultScorer())
	mutated := stageevidence.Score(chain, currentVersion, strictPromotion, silentScorer{})
	if slices.Equal(golden.Cases, mutated.Cases) || golden.Escapes == mutated.Escapes {
		t.Errorf("a scorer that classifies nothing gave the golden verdict: %+v", mutated)
	}
	if mutated.Scorer != "silent" {
		t.Errorf("the result names scorer %q, want silent", mutated.Scorer)
	}
	if !mutated.Ready {
		t.Error("a scorer that finds no fault left the chain not ready; readiness must follow the scorer")
	}
}

func singleFault(fields map[string]string, outcome stageevidence.Outcome) stageevidence.Chain {
	outcome.ID = 4
	return stageevidence.Chain{
		Records: linked([]stageevidence.Record{
			decision(1, 1, currentVersion, map[string]string{"would_stop": "false"}),
			decision(2, 2, currentVersion, map[string]string{"would_stop": "false"}),
			decision(3, 3, currentVersion, map[string]string{"would_stop": "false"}),
			decision(4, 4, currentVersion, fields),
		}),
		Outcomes: []stageevidence.Outcome{outcome},
	}
}

func TestPromotionDataDecidesReadiness(t *testing.T) {
	t.Parallel()
	clean := singleFault(map[string]string{"would_stop": "false"}, stageevidence.Outcome{Shipped: true})
	escape := singleFault(map[string]string{"would_skip": "true", "check": "lint"}, stageevidence.Outcome{Failed: []string{"lint"}})
	falseStop := singleFault(map[string]string{"would_stop": "true"}, stageevidence.Outcome{Shipped: true})
	conflict := singleFault(map[string]string{"would_hold": "true", "item": "item-9"}, stageevidence.Outcome{Routed: []string{"item-9"}})
	loose := gatecontract.Promotion{MinDecisions: 1, MaxEscapes: 5, MaxFalseStops: 5}
	cases := []struct {
		name      string
		chain     stageevidence.Chain
		promotion gatecontract.Promotion
		want      bool
	}{
		{"enough clean decisions", clean, gatecontract.Promotion{MinDecisions: 4}, true},
		{"fewer decisions than the minimum", clean, gatecontract.Promotion{MinDecisions: 5}, false},
		{"no promotion data still needs one decision", clean, gatecontract.Promotion{}, true},
		{"an escape within the limit", escape, loose, true},
		{"an escape above the limit", escape, gatecontract.Promotion{MinDecisions: 1, MaxEscapes: 0, MaxFalseStops: 5}, false},
		{"a false stop within the limit", falseStop, loose, true},
		{"a false stop above the limit", falseStop, gatecontract.Promotion{MinDecisions: 1, MaxFalseStops: 0, MaxEscapes: 5}, false},
		{"a conflict always blocks", conflict, loose, false},
	}
	for _, c := range cases {
		got := stageevidence.Score(c.chain, currentVersion, c.promotion, stageevidence.DefaultScorer())
		if got.Ready != c.want {
			t.Errorf("%s: ready = %v, want %v (%+v)", c.name, got.Ready, c.want, got)
		}
	}
}

func TestEvidenceOfOtherVersionNotCounted(t *testing.T) {
	t.Parallel()
	chain := stageevidence.Chain{
		Records: linked([]stageevidence.Record{
			decision(1, 1, "version-old", map[string]string{"would_stop": "true"}),
			decision(2, 2, "version-old", map[string]string{"would_stop": "true"}),
			decision(3, 3, "version-old", map[string]string{"would_stop": "true"}),
			decision(4, 4, currentVersion, map[string]string{"would_stop": "false"}),
			decision(5, 5, currentVersion, map[string]string{"would_stop": "false"}),
		}),
		Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}, {ID: 2, Shipped: true}, {ID: 3, Shipped: true}},
	}
	got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 3}, stageevidence.DefaultScorer())
	if got.Decisions != 2 {
		t.Errorf("decisions = %d, want 2: only the current version counts", got.Decisions)
	}
	if got.FalseStops != 0 || len(got.Cases) != 0 {
		t.Errorf("a record of another version counted as a case: %+v", got.Cases)
	}
	if got.OtherVersions["version-old"] != 3 {
		t.Errorf("other versions = %v, want version-old reported with 3 records", got.OtherVersions)
	}
	if got.Ready {
		t.Error("five records over two versions reached a minimum of three: the old version counted")
	}
}

func TestIncompleteAndLateEvidenceNeverCount(t *testing.T) {
	t.Parallel()
	late := decision(3, 3, currentVersion, map[string]string{"would_stop": "true"})
	late.Kind = stageevidence.KindLate
	chain := stageevidence.Chain{
		Records: linked([]stageevidence.Record{
			decision(1, 1, currentVersion, map[string]string{"would_stop": "true"}),
			decision(2, 2, currentVersion, map[string]string{"would_stop": "false"}),
			late,
			decision(4, 4, currentVersion, map[string]string{"would_stop": "false"}),
		}),
		Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true, Incomplete: true}, {ID: 3, Shipped: true}},
	}
	got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.DefaultScorer())
	if got.Decisions != 2 {
		t.Errorf("decisions = %d, want 2: an incomplete cycle and a late record never count", got.Decisions)
	}
	if !slices.Equal(got.IncompleteCycles, []int{1}) {
		t.Errorf("incomplete cycles = %v, want [1]", got.IncompleteCycles)
	}
	if len(got.Late) != 1 || got.Late[0].ID != 3 || got.Late[0].Seq != 3 {
		t.Errorf("late = %+v, want the one record of cycle 3 seq 3 listed", got.Late)
	}
	if got.FalseStops != 0 {
		t.Errorf("false stops = %d: a record that never counts produced a case", got.FalseStops)
	}
}

func TestBrokenChainGivesNotReadyAndTypedError(t *testing.T) {
	t.Parallel()
	mutations := []struct {
		name      string
		mutate    func([]stageevidence.Record) []stageevidence.Record
		wantCycle int
		wantSeq   int
	}{
		{"edit", func(r []stageevidence.Record) []stageevidence.Record {
			r[1].Fields = map[string]string{"would_stop": "false"}
			return r
		}, 2, 2},
		{"delete", func(r []stageevidence.Record) []stageevidence.Record { return slices.Delete(r, 3, 4) }, 4, 5},
		{"reorder", func(r []stageevidence.Record) []stageevidence.Record {
			r[3], r[4] = r[4], r[3]
			return r
		}, 4, 5},
	}
	for _, m := range mutations {
		chain := fixtureChain()
		chain.Records = m.mutate(slices.Clone(chain.Records))
		err := stageevidence.VerifyChain(chain)
		var broken *stageevidence.BrokenChainError
		if !errors.Is(err, stageevidence.ErrBrokenChain) || !errors.As(err, &broken) {
			t.Errorf("%s: VerifyChain = %v, want a typed broken-chain error", m.name, err)
			continue
		}
		if broken.Scope != stageevidence.CycleScope(m.wantCycle) || broken.Seq != m.wantSeq {
			t.Errorf("%s: names %v seq %d, want cycle %d seq %d", m.name, broken.Scope, broken.Seq, m.wantCycle, m.wantSeq)
		}
		res, verifyErr := stageevidence.Verify(context.Background(), memoryChain{chain: chain, err: err}, stageKey, currentVersion, strictPromotion, stageevidence.DefaultScorer())
		if res.Ready || !errors.Is(verifyErr, stageevidence.ErrBrokenChain) || res.Error != "broken-chain" {
			t.Errorf("%s: Verify ready=%v err=%v code=%q, want not ready, broken-chain", m.name, res.Ready, verifyErr, res.Error)
		}
		if res.Decisions != 0 {
			t.Errorf("%s: a broken chain gave a partial verdict with %d decisions", m.name, res.Decisions)
		}
	}
}

func TestIntactChainVerifiesAndEmptyChainIsNotReady(t *testing.T) {
	t.Parallel()
	if err := stageevidence.VerifyChain(fixtureChain()); err != nil {
		t.Fatalf("VerifyChain of an intact chain = %v", err)
	}
	if err := stageevidence.VerifyChain(stageevidence.Chain{}); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Errorf("VerifyChain of an empty chain = %v, want ErrEmptyChain", err)
	}
	res, err := stageevidence.Verify(context.Background(), memoryChain{err: stageevidence.ErrEmptyChain}, stageKey, currentVersion, strictPromotion, stageevidence.DefaultScorer())
	if res.Ready || !errors.Is(err, stageevidence.ErrEmptyChain) || res.Error != "empty-chain" || res.Key != stageKey {
		t.Errorf("Verify of an empty chain = ready %v, err %v, code %q, key %q", res.Ready, err, res.Error, res.Key)
	}
	good, err := stageevidence.Verify(context.Background(), memoryChain{chain: fixtureChain()}, stageKey, currentVersion, strictPromotion, stageevidence.DefaultScorer())
	if err != nil || good.Decisions != 5 || good.Error != "" {
		t.Errorf("Verify of an intact chain = %+v, %v", good, err)
	}
}

func TestEveryKeyGetsAScorer(t *testing.T) {
	t.Parallel()
	for _, key := range []string{stageKey, "held_proposals.stage", "workflow.retry_ladder.stage", "a.key.nobody.registered"} {
		scorer := stageevidence.ScorerFor(key)
		if scorer == nil || scorer.Name() == "" {
			t.Errorf("ScorerFor(%q) = %v, want a named scorer", key, scorer)
		}
	}
}
