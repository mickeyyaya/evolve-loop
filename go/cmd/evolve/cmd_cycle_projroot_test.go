package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Not parallel: it os.Chdir's so the relative "." resolves to the temp project.
func TestRunCycleReset_RelativeProjectRootAbsolutized(t *testing.T) {
	tmp := t.TempDir()
	// macOS /var → /private/var symlink: resolve so the post-seal path the
	// command prints (derived from an absolutized cwd) matches our fixture.
	tmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("EvalSymlinks(tmp): %v", err)
	}
	evolveDir := filepath.Join(tmp, ".evolve")
	runsDir := filepath.Join(evolveDir, "runs")
	workspace := filepath.Join(runsDir, "cycle-5") // ABSOLUTE, as the fixed loop writes
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	// An absolute workspace_path against a relative --project-root is the failing shape.
	csJSON := `{"cycle_id":5,"phase":"build","workspace_path":` + strconv.Quote(workspace) + `}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csJSON), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	// Minimal state.json so SealCycle can advance lastCycleNumber.
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"lastCycleNumber":4}`), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	// Make "." resolve to the temp project root.
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()

	var stdout, stderr bytes.Buffer
	rc := runCycleReset([]string{"--project-root", ".", "--force"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runCycleReset rc=%d, want 0 (a relative --project-root must be absolutized before the containment check)\nstderr=%q", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json still present after seal (err=%v)", err)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Errorf("workspace %q still present; expected it renamed to a .reset-* archive", workspace)
	}
}
