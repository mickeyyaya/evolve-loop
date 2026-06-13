package verdictcache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC) }

// TestPutThenLookup_RoundTrips: a put verdict is found by its tree SHA.
func TestPutThenLookup_RoundTrips(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root, fixedNow)
	want := Entry{TreeSHA: "abc123", Cycle: 42, Verdict: "PASS", ArtifactSHA256: "sha", ArtifactPath: "/p/audit-report.md"}
	if err := s.Put(want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := s.Lookup("abc123")
	if !ok {
		t.Fatal("Lookup miss after Put")
	}
	if got.Cycle != 42 || got.Verdict != "PASS" || got.ArtifactSHA256 != "sha" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.CachedAt.IsZero() {
		t.Error("CachedAt not stamped by Put")
	}
}

// TestLookup_Miss: an unknown tree SHA misses.
func TestLookup_Miss(t *testing.T) {
	s := NewStore(t.TempDir(), fixedNow)
	if _, ok := s.Lookup("nope"); ok {
		t.Error("expected miss on empty store")
	}
}

// TestPut_EmptyTreeSHA_NoOp: a verdict with no content key cannot be cached
// (content-addressed by definition) — Put is a no-op, never an error, and the
// entry is not retrievable under the empty key.
func TestPut_EmptyTreeSHA_NoOp(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root, fixedNow)
	if err := s.Put(Entry{TreeSHA: "", Cycle: 1, Verdict: "PASS"}); err != nil {
		t.Fatalf("Put empty key must not error: %v", err)
	}
	if _, ok := s.Lookup(""); ok {
		t.Error("empty-key entry must not be retrievable")
	}
}

// TestPut_Persists: a put survives a fresh Store over the same root (atomic
// file write), and the on-disk file is the documented path.
func TestPut_Persists(t *testing.T) {
	root := t.TempDir()
	if err := NewStore(root, fixedNow).Put(Entry{TreeSHA: "t1", Cycle: 7, Verdict: "WARN"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "verdict-cache.json")); err != nil {
		t.Fatalf("cache file not at .evolve/verdict-cache.json: %v", err)
	}
	got, ok := NewStore(root, fixedNow).Lookup("t1")
	if !ok || got.Cycle != 7 {
		t.Errorf("did not persist across Store instances: %+v ok=%v", got, ok)
	}
}

// TestLoad_CorruptFile_DegradesToEmpty: a corrupt cache file degrades to an
// empty store (advisory, self-invalidating — a miss only costs a full run,
// never correctness; same contract as clihealth).
func TestLoad_CorruptFile_DegradesToEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "verdict-cache.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(root, fixedNow)
	if _, ok := s.Lookup("anything"); ok {
		t.Error("corrupt file must degrade to empty (no hit)")
	}
	// And a Put over a corrupt file recovers (starts fresh).
	if err := s.Put(Entry{TreeSHA: "recovered", Cycle: 1, Verdict: "PASS"}); err != nil {
		t.Fatalf("Put over corrupt file: %v", err)
	}
	if _, ok := s.Lookup("recovered"); !ok {
		t.Error("Put did not recover the corrupt store")
	}
}

func TestLoad_NilVerdicts_DegradesToEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "verdict-cache.json"), []byte(`{"schema_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(root, fixedNow).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load nil verdict map = %v, want empty", got)
	}
}

func TestNewStore_NilNow_DefaultsToTimeNow(t *testing.T) {
	s := NewStore(t.TempDir(), nil)
	if err := s.Put(Entry{TreeSHA: "nil-now", Cycle: 1, Verdict: "PASS"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := s.Lookup("nil-now")
	if !ok {
		t.Fatal("Lookup miss after Put")
	}
	if got.CachedAt.IsZero() {
		t.Fatal("nil now function must default to time.Now and stamp CachedAt")
	}
}

func TestLoad_ReadError_DegradesToEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(root, fixedNow).Load()
	if err != nil {
		t.Fatalf("Load must degrade read errors to nil error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load read error = %v, want empty cache", got)
	}
}

func TestWrite_MkdirError_ReturnsError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := NewStore(root, fixedNow).Put(Entry{TreeSHA: "mkdir-error", Cycle: 1, Verdict: "PASS"})
	if err == nil {
		t.Fatal("Put with .evolve occupied by a file must fail")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("error = %v, want mkdir context", err)
	}
}

func TestWrite_WriteError_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write through directory permissions")
	}
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	err := NewStore(root, fixedNow).Put(Entry{TreeSHA: "write-error", Cycle: 1, Verdict: "PASS"})
	if err == nil {
		t.Fatal("Put with unwritable .evolve dir must fail")
	}
	if !strings.Contains(err.Error(), "write temp") {
		t.Fatalf("error = %v, want write temp context", err)
	}
}
