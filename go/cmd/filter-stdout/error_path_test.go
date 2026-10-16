package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ProcessError_ExitsWithCode1(t *testing.T) {
	ws := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })

	rawPath := filepath.Join(ws, "build-stdout.log")
	if err := os.WriteFile(rawPath, []byte("{\"type\":\"text\"}\n"), 0o644); err != nil {
		t.Fatalf("write raw log: %v", err)
	}

	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatalf("chmod workspace: %v", err)
	}

	rc := run([]string{ws, "build"})

	if rc != 1 {
		t.Errorf("rc=%d, want 1 (filter error on read-only workspace)", rc)
	}
}

func TestRun_ThreeArgs_ExitsWithCode2(t *testing.T) {
	rc := run([]string{"a", "b", "c"})
	if rc != 2 {
		t.Errorf("rc=%d, want 2 (usage error when 3 args given)", rc)
	}
}
