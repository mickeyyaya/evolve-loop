package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// A fleet lane's cycle state lives in <project-root>/.evolve/runs/cycle-N, and its override applies
// only inside the evolve dir storage resolves against; any other --evolve-dir would silently put the
// lane back on the shared cycle-state.json.
func TestRunCycleRun_AFleetLaneRefusesAnEvolveDirOutsideItsProjectRoot(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	rc := runCycleRun([]string{
		"--project-root", root, "--evolve-dir", filepath.Join(t.TempDir(), ".evolve"),
		"--goal-hash", "abcd1234", "--simulate",
	}, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10 (refused)\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--evolve-dir") {
		t.Errorf("the refusal must name --evolve-dir; stderr=%s", stderr.String())
	}
}
