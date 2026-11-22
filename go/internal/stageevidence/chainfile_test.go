package stageevidence_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func writeChainFile(t *testing.T, chain stageevidence.Chain) string {
	t.Helper()
	raw, err := json.Marshal(chain)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "chain.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func twoKeyChain() stageevidence.Chain {
	other := decision(2, 2, currentVersion, map[string]string{"would_stop": "false"})
	other.Key = "held_proposals.stage"
	return stageevidence.Chain{
		Records: linked([]stageevidence.Record{
			decision(1, 1, currentVersion, map[string]string{"would_stop": "false"}),
			other,
			decision(3, 3, currentVersion, map[string]string{"would_stop": "true"}),
		}),
		Outcomes: []stageevidence.Outcome{{ID: 3, Shipped: true}},
	}
}

func TestFileChainReadsOneKeyOfAnIntactChain(t *testing.T) {
	t.Parallel()
	reader := stageevidence.FileChain(writeChainFile(t, twoKeyChain()))
	got, err := reader.ReadChain(context.Background(), stageKey)
	if err != nil {
		t.Fatalf("ReadChain: %v", err)
	}
	if len(got.Records) != 2 || got.Records[0].Seq != 1 || got.Records[1].Seq != 3 {
		t.Errorf("records of %s = %+v, want seq 1 and 3", stageKey, got.Records)
	}
	if len(got.Outcomes) != 1 || got.Outcomes[0].ID != 3 {
		t.Errorf("outcomes = %+v, want the one outcome of cycle 3", got.Outcomes)
	}
}

func TestFileChainChecksTheWholeChainBeforeItFilters(t *testing.T) {
	t.Parallel()
	chain := twoKeyChain()
	chain.Records[1].Fields = map[string]string{"would_stop": "true"}
	reader := stageevidence.FileChain(writeChainFile(t, chain))
	_, err := reader.ReadChain(context.Background(), stageKey)
	if !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Errorf("ReadChain = %v: an edit of another key's record must break this key's chain", err)
	}
}

func TestFileChainWithNoRecordForTheKeyIsEmpty(t *testing.T) {
	t.Parallel()
	reader := stageevidence.FileChain(writeChainFile(t, twoKeyChain()))
	if _, err := reader.ReadChain(context.Background(), "swarm.stage"); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Errorf("ReadChain of a key with no record = %v, want ErrEmptyChain", err)
	}
}

func TestFileChainReportsAnUnreadableFileAsNotAChainError(t *testing.T) {
	t.Parallel()
	_, err := stageevidence.FileChain(filepath.Join(t.TempDir(), "missing.json")).ReadChain(context.Background(), stageKey)
	if err == nil || errors.Is(err, stageevidence.ErrBrokenChain) || errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Errorf("ReadChain of a missing file = %v, want an I/O error that is neither broken nor empty", err)
	}
}

func TestErrorCodeNamesTheTypedErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]error{
		"broken-chain": &stageevidence.BrokenChainError{Scope: stageevidence.CycleScope(2), Seq: 2},
		"empty-chain":  stageevidence.ErrEmptyChain,
		"":             nil,
	}
	for want, err := range cases {
		if got := stageevidence.ErrorCode(err); got != want {
			t.Errorf("ErrorCode(%v) = %q, want %q", err, got, want)
		}
	}
	if got := stageevidence.ErrorCode(errors.New("disk gone")); got != "error" {
		t.Errorf("ErrorCode of an untyped error = %q, want error", got)
	}
}
