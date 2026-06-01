package modelcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadMissingFileIsZeroNoError(t *testing.T) {
	// First run: no cache file. Must yield a zero (always-stale) catalog, not
	// an error, so the cycle-start hook treats it as "refresh needed".
	c, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("Read missing file: unexpected error %v", err)
	}
	if !c.Empty() || !c.IsStale(time.Now(), DefaultTTL) {
		t.Fatalf("missing file should be empty+stale, got %+v", c)
	}
}

func TestWriteThenReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fetched := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	want := sampleCatalog(fetched)

	if err := Write(dir, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !got.FetchedAt.Equal(fetched) {
		t.Fatalf("FetchedAt round-trip: got %v, want %v", got.FetchedAt, fetched)
	}
	if m, ok := got.Lookup("claude", "balanced"); !ok || m != "claude-sonnet-4-6" {
		t.Fatalf("Lookup after round-trip = (%q,%v)", m, ok)
	}
	if m, ok := got.Lookup("codex", "balanced"); !ok || m != "gpt-5.4" {
		t.Fatalf("codex Lookup after round-trip = (%q,%v)", m, ok)
	}
}

func TestWriteCreatesEvolveDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", ".evolve")
	if err := Write(dir, sampleCatalog(time.Now())); err != nil {
		t.Fatalf("Write into non-existent dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatalf("catalog file not created: %v", err)
	}
}

func TestWriteIsAtomicNoTmpLeft(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, sampleCatalog(time.Now())); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("expected exactly %s, got %v", FileName, entries)
	}
}

func TestReadMalformedJSONErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir); err == nil {
		t.Fatal("Read malformed JSON: expected error, got nil")
	}
}
