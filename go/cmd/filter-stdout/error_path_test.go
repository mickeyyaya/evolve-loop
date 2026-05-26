// error_path_test.go — tests the run() error branch (rc=1 when
// logfilter.Process returns a non-nil error).
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRun_ProcessError_ExitsWithCode1 verifies that when logfilter.Process
// returns an error, run() returns exit code 1.
//
// Trigger: write a raw log file into a workspace directory, then make the
// workspace read-only so logfilter.Process cannot create the temp output
// file, guaranteeing a non-nil error return.
func TestRun_ProcessError_ExitsWithCode1(t *testing.T) {
	// Use os.MkdirTemp so we control cleanup order (must restore perms before removal).
	ws, err := os.MkdirTemp("", "filter-stdout-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer func() {
		// Restore write permission so os.RemoveAll (called by testing cleanup)
		// can delete the directory and its contents.
		_ = os.Chmod(ws, 0o755)
		_ = os.RemoveAll(ws)
	}()

	// Arrange: a raw log file must exist so Process opens it and then tries
	// to create a temp file — which is the operation we block.
	rawPath := filepath.Join(ws, "build-stdout.log")
	if err := os.WriteFile(rawPath, []byte("{\"type\":\"text\"}\n"), 0o644); err != nil {
		t.Fatalf("write raw log: %v", err)
	}

	// Make the workspace directory read-only: os.CreateTemp will fail.
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatalf("chmod workspace: %v", err)
	}

	// Act
	rc := run([]string{ws, "build"})

	// Assert: logfilter.Process returned an error → run() must return 1.
	if rc != 1 {
		t.Errorf("rc=%d, want 1 (filter error on read-only workspace)", rc)
	}
}

// TestRun_ThreeArgs_ExitsWithCode2 ensures that extra arguments are also
// treated as a usage error (argument count must be exactly 2).
func TestRun_ThreeArgs_ExitsWithCode2(t *testing.T) {
	rc := run([]string{"a", "b", "c"})
	if rc != 2 {
		t.Errorf("rc=%d, want 2 (usage error when 3 args given)", rc)
	}
}
