package ledger

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func flipOf(key string) stageevidence.Flip {
	return stageevidence.Flip{V: 1, Key: key, Value: "enforce", Origin: stageevidence.OriginGrandfather, PolicyDigest: "d", Reason: "enforced before the contract"}
}

func TestFlipRecordsReadsBothKindsAndIgnoresOtherEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := New(dir)
	ctx := context.Background()
	if err := l.Append(ctx, core.LedgerEntry{Role: "scout", Kind: "phase"}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"a.stage", "b.stage"} {
		if err := l.AppendStageFlip(ctx, flipOf(key)); err != nil {
			t.Fatal(err)
		}
	}
	genesis := stageevidence.Genesis{V: 1, Keys: []string{"a.stage", "b.stage"}, PolicyDigest: "d", Count: 2}
	if err := l.AppendStageGenesis(ctx, genesis); err != nil {
		t.Fatal(err)
	}
	set, err := l.FlipRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Flips) != 2 || set.Genesis == nil || !slices.Equal(set.Genesis.Keys, genesis.Keys) {
		t.Fatalf("set = %+v", set)
	}
	if err := l.Verify(ctx); err != nil {
		t.Fatalf("ledger verify after the records: %v", err)
	}
	for _, e := range readEntries(t, dir) {
		if e.Kind == stageevidence.FlipKind && e.Role != StageFlipRole {
			t.Errorf("flip entry role = %q, want %q", e.Role, StageFlipRole)
		}
	}
}

func TestFlipRecordsWithoutRecordsIsEmpty(t *testing.T) {
	t.Parallel()
	set, err := New(t.TempDir()).FlipRecords(context.Background())
	if err != nil || len(set.Flips) != 0 || set.Genesis != nil {
		t.Fatalf("set = %+v, err %v", set, err)
	}
}

func TestGenesisInCompactedSegmentIsPresent(t *testing.T) {
	t.Parallel()
	l := New(t.TempDir())
	ctx := context.Background()
	if err := l.AppendStageFlip(ctx, flipOf("a.stage")); err != nil {
		t.Fatal(err)
	}
	if err := l.AppendStageGenesis(ctx, stageevidence.Genesis{V: 1, Keys: []string{"a.stage"}, PolicyDigest: "d", Count: 1}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := l.Append(ctx, core.LedgerEntry{Role: "scout", Kind: "phase"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Seal(ctx, 2); err != nil {
		t.Fatal(err)
	}
	set, err := l.FlipRecords(ctx)
	if err != nil || set.Genesis == nil || len(set.Flips) != 1 {
		t.Fatalf("after compaction: set = %+v, err %v", set, err)
	}
}

func TestFlipRecords_TheFirstGenesisRecordIsTheGrandfatherSet(t *testing.T) {
	t.Parallel()
	l := New(t.TempDir())
	ctx := context.Background()
	if err := l.AppendStageGenesis(ctx, stageevidence.Genesis{V: 1, Keys: []string{"a.stage"}, PolicyDigest: "d1", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if err := l.AppendStageGenesis(ctx, stageevidence.Genesis{V: 1, Keys: []string{"a.stage", "b.stage"}, PolicyDigest: "d2", Count: 2}); err != nil {
		t.Fatal(err)
	}
	set, err := l.FlipRecords(ctx)
	if err != nil || set.Genesis == nil {
		t.Fatalf("set = %+v, err %v", set, err)
	}
	if !slices.Equal(set.Genesis.Keys, []string{"a.stage"}) || set.Genesis.PolicyDigest != "d1" {
		t.Fatalf("genesis = %+v, want the first record: a later genesis must not widen the grandfather set", *set.Genesis)
	}
}

func TestFlipRecordsRefusesAMalformedRecord(t *testing.T) {
	t.Parallel()
	l := New(t.TempDir())
	ctx := context.Background()
	if err := l.Append(ctx, core.LedgerEntry{Role: "stage-flip", Kind: stageevidence.FlipKind, Message: "{"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.FlipRecords(ctx); !errors.Is(err, stageevidence.ErrMalformedFlipRecord) {
		t.Fatalf("err = %v, want ErrMalformedFlipRecord", err)
	}
	other := New(t.TempDir())
	if err := other.Append(ctx, core.LedgerEntry{Role: "stage-flip", Kind: stageevidence.GenesisKind, Message: "{"}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.FlipRecords(ctx); !errors.Is(err, stageevidence.ErrMalformedFlipRecord) {
		t.Fatalf("genesis err = %v, want ErrMalformedFlipRecord", err)
	}
}

func TestFlipRecordsReportsAnUnreadableLedger(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := New(dir)
	ctx := context.Background()
	if err := l.AppendStageFlip(ctx, flipOf("a.stage")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/ledger.jsonl", []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.FlipRecords(ctx); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("err = %v, want ErrLedgerBroken", err)
	}
}

func TestFlipRecordsAndAppendsReportAnUnwritableLedger(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(dir+"/ledger.jsonl", 0o700); err != nil {
		t.Fatal(err)
	}
	l := New(dir)
	ctx := context.Background()
	if err := l.AppendStageFlip(ctx, flipOf("a.stage")); err == nil {
		t.Error("AppendStageFlip into an unwritable ledger returned nil")
	}
	if _, err := l.FlipRecords(ctx); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Errorf("FlipRecords err = %v, want ErrLedgerBroken", err)
	}
}

func TestFlipSetOfRefusesALineThatNamesAFlipKindButIsNotAnEntry(t *testing.T) {
	t.Parallel()
	if _, err := flipSetOf([][]byte{[]byte(`not json "stage-flip"`)}); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("err = %v, want ErrLedgerBroken", err)
	}
}
