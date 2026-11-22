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
		ID:      1,
		Seq:     1,
		Fields:  map[string]string{"would_stop": "true"},
	})
	r2 := stageevidence.Link("", stageevidence.Record{
		Key:     "k",
		Version: "v1",
		Kind:    stageevidence.KindLate,
		ID:      2,
		Seq:     2,
	})
	c := stageevidence.Chain{
		Records:  []stageevidence.Record{r1, r2},
		Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}},
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

	broken := &stageevidence.BrokenChainError{Scope: stageevidence.CycleScope(1), Seq: 1}
	if !errors.Is(broken, stageevidence.ErrBrokenChain) || broken.Unwrap() != stageevidence.ErrBrokenChain {
		t.Fatalf("BrokenChainError: %v", broken)
	}
	if other := (&stageevidence.BrokenChainError{Scope: stageevidence.CycleScope(2), Seq: 3}); broken.Error() == other.Error() {
		t.Fatalf("BrokenChainError messages do not name the position: %q", other.Error())
	}
	if stageevidence.ErrorCode(stageevidence.ErrEmptyChain) != "empty-chain" {
		t.Fatal("ErrorCode mismatch")
	}
}

func TestAPICover_NamesTheEvidenceRecorderAndSealExports(t *testing.T) {
	t.Parallel()
	scope := stageevidence.CycleScope(1)
	kinds := []stageevidence.ScopeKind{stageevidence.ScopeCycle, stageevidence.ScopePool, stageevidence.ScopeFleet, stageevidence.ScopeCurator}
	for _, kind := range kinds {
		key := stageevidence.ScopeKey{Kind: kind, ID: 2}
		parsed, ok := stageevidence.ParseScope(key.String())
		if !ok || parsed != key {
			t.Fatalf("ParseScope(%q) = %v %v", key.String(), parsed, ok)
		}
	}
	for _, bad := range []string{"", "cycle", "cycle-x", "cycle--1", "moon-3"} {
		if _, ok := stageevidence.ParseScope(bad); ok {
			t.Errorf("ParseScope(%q) accepted", bad)
		}
	}
	dir := t.TempDir()
	var recorder *stageevidence.Recorder = stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: dir, Scope: scope, Session: "s", Build: "b",
		Lock:    func(string) (func(), error) { return func() {}, nil },
		Version: func(string) (string, error) { return "v", nil },
		OnFault: func(err error) { t.Errorf("fault: %v", err) },
	})
	recorder.Record("k", map[string]string{"would_stop": "false"})
	recorder.Close()
	raw, err := os.ReadFile(stageevidence.WorkingPath(dir, scope))
	if err != nil || filepath.Base(filepath.Dir(stageevidence.WorkingPath(dir, scope))) != stageevidence.WorkingDirName {
		t.Fatalf("working file: %v", err)
	}
	var seal stageevidence.Seal
	seal, records, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{})
	if err != nil || seal.V != stageevidence.SealVersion {
		t.Fatalf("BuildSeal: %+v %v", seal, err)
	}
	if _, err := stageevidence.VerifyStored(scope, raw, seal); err != nil {
		t.Fatal(err)
	}
	message := seal.Encode()
	if parsed, err := stageevidence.ParseSeal(message); err != nil || parsed.Count != seal.Count {
		t.Fatalf("ParseSeal: %+v %v", parsed, err)
	}
	if out := seal.Outcome(scope); out.ScopeKey() != scope || records[0].ScopeKey() != scope {
		t.Fatalf("scope keys differ: %+v", out)
	}
	var chain stageevidence.Chain = stageevidence.PreverifiedChain(records, nil, "d")
	if chain.Digest != "d" {
		t.Fatal("PreverifiedChain lost the digest")
	}
	causes := []string{stageevidence.CauseWriteFailed, stageevidence.CauseOpenSession, stageevidence.CauseGap, stageevidence.CauseTornTail, stageevidence.FaultWriteFailed}
	for _, c := range causes {
		if c == "" {
			t.Fatal("empty cause code")
		}
	}
	if stageevidence.ErrorCode(stageevidence.ErrLedgerBroken) != "ledger-broken" || stageevidence.ErrorCode(stageevidence.ErrCorruptStore) != "corrupt-store" {
		t.Fatal("ErrorCode does not name the ledger and store faults")
	}
	var _ stageevidence.SealInput
	_ = seal.IsPrefixOf(records)
	_, _ = stageevidence.ParseWorkingName(filepath.Base(stageevidence.WorkingPath(dir, scope)))
	_ = []stageevidence.Kind{stageevidence.KindOpen, stageevidence.KindClose}
}
