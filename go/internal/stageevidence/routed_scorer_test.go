package stageevidence_test

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	heldProposalKey   = "held_proposals.stage"
	routedItem        = "item-1"
	scorerVersionTag  = "v1"
	routedScorerCycle = 41
)

func heldProposalWorking(t *testing.T, scope stageevidence.ScopeKey) []byte {
	t.Helper()
	dir := t.TempDir()
	rec := stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: dir,
		Scope:     scope,
		Session:   "s1",
		Build:     "rev-abc",
		Lock:      flock.Lock,
		Version:   func(string) (string, error) { return scorerVersionTag, nil },
		OnFault:   func(err error) { t.Errorf("recorder fault: %v", err) },
	})
	rec.Record(heldProposalKey, map[string]string{"would_hold": "true", "item": routedItem})
	rec.Close()
	raw, err := os.ReadFile(stageevidence.WorkingPath(dir, scope))
	if err != nil {
		t.Fatalf("read working file: %v", err)
	}
	return raw
}

func scoreSealedOutcome(t *testing.T, scope stageevidence.ScopeKey, records []stageevidence.Record, seal stageevidence.Seal) stageevidence.Result {
	t.Helper()
	chain := stageevidence.Chain{Records: records, Outcomes: []stageevidence.Outcome{seal.Outcome(scope)}}
	return stageevidence.Score(chain, scorerVersionTag, gatecontract.Promotion{MinDecisions: 1}, stageevidence.DefaultScorer())
}

func TestSealOutcomeRoutedReachesConflictClass(t *testing.T) {
	scope := stageevidence.CycleScope(routedScorerCycle)
	raw := heldProposalWorking(t, scope)
	seal, records, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{Shipped: true, Exit: "normal", Routed: []string{routedItem}})
	if err != nil {
		t.Fatalf("BuildSeal: %v", err)
	}
	res := scoreSealedOutcome(t, scope, records, seal)
	if res.Conflicts != 1 || res.Ready {
		t.Fatalf("sealed routed item must reach the conflict class: conflicts=%d ready=%v", res.Conflicts, res.Ready)
	}
	if len(res.Cases) != 1 || res.Cases[0].Class != stageevidence.ClassConflict {
		t.Fatalf("cases = %+v, want one conflict case", res.Cases)
	}
}

func TestSealOutcomeWithoutRoutedStaysClear(t *testing.T) {
	scope := stageevidence.CycleScope(routedScorerCycle)
	raw := heldProposalWorking(t, scope)
	seal, records, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{Shipped: true, Exit: "normal"})
	if err != nil {
		t.Fatalf("BuildSeal: %v", err)
	}
	if res := scoreSealedOutcome(t, scope, records, seal); res.Conflicts != 0 {
		t.Fatalf("a seal that routed nothing must not conflict: conflicts=%d", res.Conflicts)
	}
}

func TestSealRoutedSurvivesEncodeParseAndVerifyStored(t *testing.T) {
	scope := stageevidence.CycleScope(routedScorerCycle)
	raw := heldProposalWorking(t, scope)
	seal, records, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{Shipped: true, Exit: "normal", Routed: []string{routedItem}})
	if err != nil {
		t.Fatalf("BuildSeal: %v", err)
	}
	if _, err := stageevidence.VerifyStored(scope, raw, seal); err != nil {
		t.Fatalf("VerifyStored of a seal that carries routed items: %v", err)
	}
	parsed, err := stageevidence.ParseSeal(seal.Encode())
	if err != nil {
		t.Fatalf("ParseSeal: %v", err)
	}
	if got := parsed.Outcome(scope).Routed; len(got) != 1 || got[0] != routedItem {
		t.Fatalf("parsed seal Outcome.Routed = %v, want [%s]", got, routedItem)
	}
	if res := scoreSealedOutcome(t, scope, records, parsed); res.Conflicts != 1 {
		t.Fatalf("parsed seal must still reach the conflict class: conflicts=%d", res.Conflicts)
	}
}
