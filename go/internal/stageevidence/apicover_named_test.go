package stageevidence_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func TestAPICover_NamesAllStageEvidenceExports(t *testing.T) {
	t.Parallel()
	var _ stageevidence.ChainReader = stageevidence.FileChain("")
	var _ stageevidence.Scorer = stageevidence.DefaultScorer()

	r1 := stageevidence.Link("", stageevidence.Record{
		Key:     "k",
		Version: "v1",
		Kind:    stageevidence.KindDecision,
		Cycle:   1,
		Seq:     1,
		Fields:  map[string]string{"would_stop": "true"},
	})
	r2 := stageevidence.Link(r1.Hash, stageevidence.Record{
		Key:     "k",
		Version: "v1",
		Kind:    stageevidence.KindLate,
		Cycle:   2,
		Seq:     2,
	})
	c := stageevidence.Chain{
		Records:  []stageevidence.Record{r1, r2},
		Outcomes: []stageevidence.Outcome{{Cycle: 1, Shipped: true}},
	}
	if err := stageevidence.VerifyChain(c); err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
	scorer := stageevidence.ScorerFor("k")
	if scorer.Classify(r1, c.Outcomes[0]) != stageevidence.ClassFalseStop {
		t.Fatal("expected ClassFalseStop")
	}
	if got := scorer.Classify(stageevidence.Record{}, stageevidence.Outcome{}); got != stageevidence.ClassNone {
		t.Fatalf("expected ClassNone, got %v", got)
	}
	if got := scorer.Classify(stageevidence.Record{Fields: map[string]string{"would_skip": "true", "check": "c"}}, stageevidence.Outcome{Failed: []string{"c"}}); got != stageevidence.ClassEscape {
		t.Fatalf("expected ClassEscape, got %v", got)
	}
	if got := scorer.Classify(stageevidence.Record{Fields: map[string]string{"would_hold": "true", "item": "i"}}, stageevidence.Outcome{Routed: []string{"i"}}); got != stageevidence.ClassConflict {
		t.Fatalf("expected ClassConflict, got %v", got)
	}

	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	chainPath := filepath.Join(t.TempDir(), "chain.json")
	if err := os.WriteFile(chainPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var res stageevidence.Result
	res, err = stageevidence.Verify(context.Background(), stageevidence.FileChain(chainPath), "k", "v1", gatecontract.Promotion{MinDecisions: 1}, scorer)
	if err != nil || res.FalseStops != 1 || res.Cases[0] != (stageevidence.Case{Class: stageevidence.ClassFalseStop, Cycle: 1, Seq: 1}) {
		t.Fatalf("Verify: res=%+v err=%v", res, err)
	}
	var scored stageevidence.Result = stageevidence.Score(c, "v1", gatecontract.Promotion{MinDecisions: 1}, scorer)
	if scored.Decisions != 1 {
		t.Fatalf("Score: %+v", scored)
	}

	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module example.com/m\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ver, err := stageevidence.DecisionVersion(modDir, nil, []byte(`{"k":{"stage":"shadow"}}`), "k")
	if err != nil || ver == "" {
		t.Fatalf("DecisionVersion: ver=%q err=%v", ver, err)
	}

	broken := &stageevidence.BrokenChainError{Cycle: 1, Seq: 1}
	if !errors.Is(broken, stageevidence.ErrBrokenChain) || broken.Unwrap() != stageevidence.ErrBrokenChain {
		t.Fatalf("BrokenChainError: %v", broken)
	}
	if other := (&stageevidence.BrokenChainError{Cycle: 2, Seq: 3}); broken.Error() == other.Error() {
		t.Fatalf("BrokenChainError messages do not name the position: %q", other.Error())
	}
	if stageevidence.ErrorCode(stageevidence.ErrEmptyChain) != "empty-chain" {
		t.Fatal("ErrorCode mismatch")
	}
}
