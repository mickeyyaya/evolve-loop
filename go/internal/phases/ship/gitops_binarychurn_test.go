package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubRunningExecutable(t *testing.T, path string) {
	t.Helper()
	old := osExecutable
	osExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { osExecutable = old })
}

func TestDiscardBinaryChurn_NeverRemovesRunningExecutable(t *testing.T) {
	root := t.TempDir()
	runningBin := filepath.Join(root, "go", "bin", "evolve")
	mustWrite(t, runningBin, "running-binary\n")
	stubRunningExecutable(t, runningBin)

	cap := &stagingCapture{} // every ls-files → exit 0, empty stdout = untracked
	var warn strings.Builder
	opts := &Options{
		ProjectRoot:    root,
		Runner:         cap.runner(),
		Stderr:         &warn,
		ShipBinaryPath: "", // cmd_ship.go never sets it in production.
	}

	if err := discardBinaryChurn(context.Background(), opts, root); err != nil {
		t.Fatalf("discardBinaryChurn: %v", err)
	}
	if _, err := os.Stat(runningBin); err != nil {
		t.Fatalf("RED (2026-07-12 incidents): discardBinaryChurn deleted the currently-executing binary %s: %v — every PreToolUse kernel hook degrades to the stale tracked fallback until rebuild", runningBin, err)
	}
	if !strings.Contains(warn.String(), "currently-executing") {
		t.Errorf("skip must WARN loudly (fail-loudly rule); stderr: %q", warn.String())
	}
}

func TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent(t *testing.T) {
	root := t.TempDir()
	runningBin := filepath.Join(root, "go", "bin", "evolve")
	mustWrite(t, runningBin, "running-binary\n")
	mustWrite(t, filepath.Join(root, "go", "evolve"), "post-audit churn\n")
	stubRunningExecutable(t, runningBin)

	cap := &stagingCapture{}
	opts := &Options{
		Class:          ClassManual,
		ProjectRoot:    root,
		PluginRoot:     root,
		CommitMessage:  "manual: operator commit",
		Runner:         cap.runner(),
		Stderr:         io.Discard,
		ShipBinaryPath: "", // CLI parity: manual ships resolve via os.Executable
	}

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual): %v", err)
	}
	if _, err := os.Stat(runningBin); err != nil {
		t.Errorf("RED: successful manual ship deleted the running go/bin/evolve: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("guard over-reached: untracked go/evolve churn was NOT discarded (unaudited binaries would ride into commits)")
	}
}
