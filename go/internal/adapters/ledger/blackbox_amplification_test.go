package ledger_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func appendAmplificationEntries(t *testing.T, l *ledger.FileLedger, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{
			TS:             "2026-06-14T00:00:00Z",
			Cycle:          331,
			Role:           "test-amplifier",
			Kind:           "blackbox-edge",
			ArtifactPath:   "artifact.txt",
			ArtifactSHA256: strings.Repeat("a", 64),
			ChallengeToken: "challenge",
		}); err != nil {
			t.Fatalf("append entry %d: %v", i, err)
		}
	}
}

func TestAnchorMissingSequenceLeavesNoAnchorFileAndLedgerVerifies(t *testing.T) {
	dir := t.TempDir()
	l := ledger.New(dir)
	appendAmplificationEntries(t, l, 2)

	if err := l.Anchor(context.Background(), 999, "missing seq"); err == nil {
		t.Fatal("Anchor with a missing sequence returned nil")
	}
	if _, err := os.Stat(filepath.Join(dir, "ledger-anchor.json")); !os.IsNotExist(err) {
		t.Fatalf("failed Anchor left an anchor file behind: %v", err)
	}
	if err := l.Verify(context.Background()); err != nil {
		t.Fatalf("ledger should remain verifiable after failed Anchor: %v", err)
	}
}

func TestSealKeepTailZeroStillProducesDeepVerifiableLedger(t *testing.T) {
	dir := t.TempDir()
	l := ledger.New(dir)
	appendAmplificationEntries(t, l, 3)

	if err := l.Seal(context.Background(), 0); err != nil {
		t.Fatalf("Seal with keepTail=0 returned error: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after Seal(0): %v", err)
	}
	segments, err := os.ReadDir(filepath.Join(dir, "ledger-segments"))
	if err != nil {
		t.Fatalf("reading segments dir after Seal(0): %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("Seal(0) wrote %d segments, want 1", len(segments))
	}
	if !strings.HasSuffix(segments[0].Name(), ".jsonl.gz") {
		t.Fatalf("segment name %q does not use .jsonl.gz suffix", segments[0].Name())
	}
}

func TestSealSingleEntryNoopLeavesNoSegmentDirectory(t *testing.T) {
	dir := t.TempDir()
	l := ledger.New(dir)
	appendAmplificationEntries(t, l, 1)

	if err := l.Seal(context.Background(), 1); err != nil {
		t.Fatalf("Seal single entry returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ledger-segments")); !os.IsNotExist(err) {
		t.Fatalf("single-entry Seal created segment state: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after single-entry Seal no-op: %v", err)
	}
}
