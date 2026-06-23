package guardcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func writeBuildCycleState(t *testing.T, evolveDir, worktree string) {
	t.Helper()
	s := storage.New(evolveDir)
	if err := s.WriteCycleState(context.Background(), core.CycleState{
		CycleID:        20,
		Phase:          "build",
		ActiveAgent:    "builder",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(evolveDir, "runs", "cycle-20"),
	}); err != nil {
		t.Fatal(err)
	}
}

// TestRunGuard_RoleControlPlaneEdit_DeniesAndAlarms is the end-to-end cycle-20
// regression at the hook layer: a build-phase Edit of a protected gate file is
// DENIED (exit 2) AND writes a CRITICAL record to .evolve/integrity-alarm.jsonl.
func TestRunGuard_RoleControlPlaneEdit_DeniesAndAlarms(t *testing.T) {
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(dir, "wt", "cycle-20")
	writeBuildCycleState(t, evolveDir, worktree)

	gate := filepath.Join(worktree, "go/acs/regression/flagreaders/readers_test.go")
	input := `{"tool_name":"Edit","tool_input":{"file_path":"` + gate + `"}}`
	var stdout, stderr bytes.Buffer
	rc := RunGuard([]string{"--evolve-dir", evolveDir, "role"}, strings.NewReader(input), &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("expected exit 2 (deny) for control-plane edit, got %d; stderr=%s", rc, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(evolveDir, "integrity-alarm.jsonl"))
	if err != nil {
		t.Fatalf("integrity-alarm.jsonl not written: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &rec); err != nil {
		t.Fatalf("alarm record not valid JSON: %v (%s)", err, data)
	}
	if rec["severity"] != "CRITICAL" || rec["kind"] != "integrity-violation" {
		t.Errorf("alarm record wrong severity/kind: %v", rec)
	}
	if rec["path"] != gate {
		t.Errorf("alarm path = %v, want %s", rec["path"], gate)
	}
}

// TestRunGuard_RoleNormalSourceEdit_NoAlarm confirms a legit worktree source
// write allows (exit 0) and writes NO integrity alarm — the boundary must not
// over-block ordinary build work.
func TestRunGuard_RoleNormalSourceEdit_NoAlarm(t *testing.T) {
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(dir, "wt", "cycle-20")
	writeBuildCycleState(t, evolveDir, worktree)

	src := filepath.Join(worktree, "go/internal/core/orchestrator.go")
	input := `{"tool_name":"Edit","tool_input":{"file_path":"` + src + `"}}`
	var stdout, stderr bytes.Buffer
	rc := RunGuard([]string{"--evolve-dir", evolveDir, "role"}, strings.NewReader(input), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("expected exit 0 (allow) for normal source, got %d; stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "integrity-alarm.jsonl")); !os.IsNotExist(err) {
		t.Errorf("no integrity alarm should be written for a legit source edit (err=%v)", err)
	}
}
