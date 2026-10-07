package ledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func ledgerLines(t *testing.T, dir string) [][]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return splitLines(raw)
}

func TestVerifyScope_ASealedLedgerResumesFromTheNewestSegmentsLastLine(t *testing.T) {
	l, dir := seedLedger(t, 12)
	sealedLast := ledgerLines(t, dir)[7]
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}

	scope, err := l.VerifyScope(context.Background())

	if err != nil {
		t.Fatalf("VerifyScope of a sealed, untampered ledger: %v", err)
	}
	want := VerifiedScope{AnchorLineSHA: sha256Hex(sealedLast), AnchorSeq: 7, FromSealedSegment: true}
	if scope != want {
		t.Errorf("scope = %+v, want %+v", scope, want)
	}
}

func TestVerifyScope_AnEpochAnchorSealedIntoASegmentStillVerifiesTheLiveTail(t *testing.T) {
	l, dir := seedLedger(t, 12)
	writeSidecarAnchor(t, dir, 3, sha256Hex(ledgerLines(t, dir)[3]))
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}

	scope, err := l.VerifyScope(context.Background())

	if err != nil {
		t.Fatalf("VerifyScope with the sidecar's anchor line sealed away: %v", err)
	}
	if !scope.FromSealedSegment || scope.AnchorSeq != 7 {
		t.Errorf("scope = %+v, want the newest segment's last line (seq 7)", scope)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep of the same ledger: %v", err)
	}
}

func TestVerifyScope_AnEpochAnchorStillLiveIsTheScopeOnASealedLedger(t *testing.T) {
	l, dir := seedLedger(t, 12)
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	liveAnchor := ledgerLines(t, dir)[1]
	writeSidecarAnchor(t, dir, 9, sha256Hex(liveAnchor))

	scope, err := l.VerifyScope(context.Background())

	if err != nil {
		t.Fatalf("VerifyScope: %v", err)
	}
	if want := (VerifiedScope{AnchorLineSHA: sha256Hex(liveAnchor), AnchorSeq: 9}); scope != want {
		t.Errorf("scope = %+v, want the live epoch anchor %+v", scope, want)
	}
}

func TestVerify_ASealThatCrashedBeforeTruncationVerifiesFromThePreviousSegment(t *testing.T) {
	ctx := context.Background()
	l, dir := seedLedger(t, 12)
	if err := l.Seal(ctx, 6); err != nil {
		t.Fatal(err)
	}
	live := ledgerLines(t, dir)
	var prefix []byte
	for _, line := range live[:3] {
		prefix = append(append(prefix, line...), '\n')
	}
	if err := writeSegment(filepath.Join(dir, segmentsDirName, "seg-0002.jsonl.gz"), prefix); err != nil {
		t.Fatal(err)
	}

	if err := l.Verify(ctx); err != nil {
		t.Fatalf("Verify with the newest segment still live (a crash before truncation): %v", err)
	}
	if err := l.VerifyDeep(ctx); err == nil {
		t.Fatal("VerifyDeep must still report the untruncated segment as residue")
	}
}
