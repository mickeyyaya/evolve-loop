package modelcatalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestShadowRoundTrip: WriteShadow persists to ShadowFileName and ReadShadow
// loads it back; the LIVE catalog file is never touched (dispatch reads only
// FileName, so a shadow deploy is byte-identical to off at dispatch), and no
// .prev rollback copy is rotated (retention protects the hand-authored live
// catalog; the shadow file is disposable pipeline output).
func TestShadowRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cat := Catalog{
		FetchedAt: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		CLIs: map[string]CLIEntry{
			"codex": {TierModels: map[string]string{"deep": "gpt-5.5"}, Source: SourceLive, CandidatesHash: "sha256:x"},
		},
	}
	if err := WriteShadow(dir, cat); err != nil {
		t.Fatalf("WriteShadow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ShadowFileName)); err != nil {
		t.Fatalf("shadow file not at ShadowFileName: %v", err)
	}
	got, err := ReadShadow(dir)
	if err != nil {
		t.Fatalf("ReadShadow: %v", err)
	}
	if got.CLIs["codex"].CandidatesHash != "sha256:x" || !got.FetchedAt.Equal(cat.FetchedAt) {
		t.Errorf("round trip lost data: %#v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Error("WriteShadow touched the live catalog file")
	}
	if _, err := os.Stat(filepath.Join(dir, PrevFileName)); !os.IsNotExist(err) {
		t.Error("WriteShadow rotated the .prev rollback copy")
	}
}

// TestReadShadow_MissingIsZero: a missing shadow file yields a zero catalog
// (stale by definition) with no error — first shadow run refreshes
// transparently, mirroring Read's contract for the live file.
func TestReadShadow_MissingIsZero(t *testing.T) {
	t.Parallel()
	got, err := ReadShadow(t.TempDir())
	if err != nil {
		t.Fatalf("ReadShadow(missing): %v", err)
	}
	if !got.Empty() || !got.FetchedAt.IsZero() {
		t.Errorf("missing shadow should be zero catalog, got %#v", got)
	}
}
