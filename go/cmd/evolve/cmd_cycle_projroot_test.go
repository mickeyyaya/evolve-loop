package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRunCycleReset_RelativeProjectRootAbsolutized(t *testing.T) {
	tmp := t.TempDir()
	tmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("EvalSymlinks(tmp): %v", err)
	}
	evolveDir := filepath.Join(tmp, ".evolve")
	runsDir := filepath.Join(evolveDir, "runs")
	workspace := filepath.Join(runsDir, "cycle-5")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	csJSON := `{"cycle_id":5,"phase":"build","workspace_path":` + strconv.Quote(workspace) + `}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csJSON), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"lastCycleNumber":4}`), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

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
