package clihealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStoreNilClockDefaultsToTimeNow(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	if s.now == nil {
		t.Fatal("NewStore(root, nil) left now nil")
	}
	if got := s.Active(); len(got) != 0 {
		t.Fatalf("new nil-clock store active entries = %v, want empty", got)
	}
}

func TestStoreWriteMkdirFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	notDir := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(notDir, []byte("file blocks mkdir"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Store{path: filepath.Join(notDir, "cli-health.json"), now: fixedNow(t0)}
	err := s.write(map[string]Entry{"codex": {Family: "codex", Reason: "rate_limit"}})
	if err == nil {
		t.Fatal("write through file parent succeeded, want mkdir error")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("error = %v, want mkdir context", err)
	}
}

func TestStoreWriteTempFileFailure(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write through read-only directory permissions")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	s := &Store{path: filepath.Join(dir, "cli-health.json"), now: fixedNow(t0)}
	err := s.write(map[string]Entry{"codex": {Family: "codex", Reason: "rate_limit"}})
	if err == nil {
		t.Fatal("write in read-only directory succeeded, want temp-file error")
	}
	if !strings.Contains(err.Error(), "write temp") {
		t.Fatalf("error = %v, want write temp context", err)
	}
}

func TestStoreWriteRenameFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(dir, "cli-health.json", "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &Store{path: filepath.Join(dir, "cli-health.json"), now: fixedNow(t0)}
	err := s.write(map[string]Entry{"codex": {Family: "codex", Reason: "rate_limit"}})
	if err == nil {
		t.Fatal("rename over non-empty directory succeeded, want rename error")
	}
	if !strings.Contains(err.Error(), "rename") {
		t.Fatalf("error = %v, want rename context", err)
	}
}

func TestStoreLoadReadErrorDegradesToEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dirPath := filepath.Join(root, ".evolve", "cli-health.json")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewStore(root, fixedNow(t0))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load read error must degrade, got error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load read error entries = %v, want empty", got)
	}
}

func TestStoreLoadMissingBenchesDegradesToEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(root, ".evolve", "cli-health.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"schema_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(root, fixedNow(t0))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load valid schema with missing benches returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load missing benches entries = %v, want empty", got)
	}
}
