package acsrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteVerdict_UnwritableVerdictDirFailsAtTempFileAndLeavesNoVerdict(t *testing.T) {
	evolveDir := t.TempDir()
	runDir := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(runDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o755) })
	if f, err := os.CreateTemp(runDir, "probe"); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced for this user")
	}

	path, err := WriteVerdict(evolveDir, Verdict{Cycle: 7})
	if err == nil || !strings.Contains(err.Error(), "verdict tmp") {
		t.Fatalf("want a verdict tmp error, got path=%q err=%v", path, err)
	}
	if path != "" {
		t.Errorf("no path may be returned on failure, got %q", path)
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "acs-verdict.json")); !os.IsNotExist(statErr) {
		t.Errorf("no verdict file may exist after the failure: %v", statErr)
	}
}
