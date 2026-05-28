package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestRunCycleReset_RelativeProjectRootAbsolutized encodes the cycle-120 reset
// refusal that motivated Workstream A's shared helper.
//
// The fixed loop writes an ABSOLUTE workspace_path into cycle-state.json. But
// `evolve cycle reset` still defaulted --project-root to "." and never
// absolutized it, so SealCycle's containment guard compared a relative root
// (".") against the absolute workspace_path — pathWithin() returned false for
// both roots and reset REFUSED with "outside evolveDir/projectRoot". The
// operator had to pass an explicit absolute --project-root as a workaround.
//
// This test reproduces that exact shape (relative --project-root, absolute
// workspace_path) and asserts the seal now SUCCEEDS — proving the root is
// absolutized before the containment check, so the comparison is
// absolute-to-absolute. It runs --force to skip the dispatcher-lock check and
// must NOT be parallel (it os.Chdir's to make "." resolve to the temp project).
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
	// cycle-state.json with an absolute workspace_path — the cycle-120 signature.
	csJSON := `{"cycle_id":5,"phase":"build","workspace_path":` + strconv.Quote(workspace) + `}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csJSON), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	// Minimal state.json so SealCycle can advance lastCycleNumber.
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"lastCycleNumber":4}`), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	// Make "." resolve to the temp project root, then run reset with the
	// RELATIVE default --project-root that broke cycle-120.
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
	// The cycle-state.json (the abandon commit point) must be gone after a seal.
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json still present after seal (err=%v)", err)
	}
	// And the workspace must have been archived (renamed away).
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Errorf("workspace %q still present; expected it renamed to a .reset-* archive", workspace)
	}
}
