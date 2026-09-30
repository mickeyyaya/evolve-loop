//go:build acs

package cycle295

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func seedCheckpointedState(t *testing.T, cp map[string]any) string {
	t.Helper()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir evolveDir: %v", err)
	}
	state := map[string]any{"cycle_id": 294, "phase": "mutation-gate", "checkpoint": cp}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return evolveDir
}

func TestC295_001_WriteCycleStatePreservesCheckpoint(t *testing.T) {
	cp := map[string]any{"resume_from_phase": "tdd", "phase_completed": "tdd"}
	evolveDir := seedCheckpointedState(t, cp)

	next := core.CycleState{CycleID: 294, Phase: "audit", WorkspacePath: "/ws"}
	if err := storage.New(evolveDir).WriteCycleState(context.Background(), next); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(evolveDir, "cycle-state.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["phase"] != "audit" {
		t.Errorf("phase = %v, want audit (new cycle state not written)", got["phase"])
	}
	block, ok := got["checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("RED: \"checkpoint\" block erased by WriteCycleState (got %T) — resume impossible after a crash", got["checkpoint"])
	}
	for k, want := range cp {
		if block[k] != want {
			t.Errorf("RED: checkpoint[%q] = %v, want %v (block mutated, not preserved)", k, block[k], want)
		}
	}
}

func TestC295_002_WriteCycleStateCheckpointNotDroppedOrDuplicated(t *testing.T) {
	evolveDir := seedCheckpointedState(t, map[string]any{"resume_from_phase": "build"})
	s := storage.New(evolveDir)
	for i := 0; i < 2; i++ {
		if err := s.WriteCycleState(context.Background(), core.CycleState{CycleID: 294, Phase: "audit"}); err != nil {
			t.Fatalf("WriteCycleState #%d: %v", i, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "cycle-state.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if n := strings.Count(string(raw), `"checkpoint"`); n != 1 {
		t.Errorf("RED: \"checkpoint\" key count = %d after two writes, want exactly 1 (0 = erased, >1 = duplicated)", n)
	}
}

func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", name, "./internal/core/")
	return stdout + "\n" + stderr
}

func TestC295_003_RelativeWorktreeBaseRefused(t *testing.T) {
	out := runCoreTest(t, "TestGitWorktree_RelativeBaseRefused")
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: TestGitWorktree_RelativeBaseRefused FAILs — relative-base guard absent:\n%s", tail(out, 30))
	}
	if !topLevelPassed(out, "TestGitWorktree_RelativeBaseRefused") {
		t.Errorf("RED: no `--- PASS: TestGitWorktree_RelativeBaseRefused` — the IsAbs guard is not in gitWorktree.Create")
	}
}

func TestC295_004_RelativeProjectRootRefused(t *testing.T) {
	out := runCoreTest(t, "TestGitWorktree_RelativeProjectRootRefused")
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: TestGitWorktree_RelativeProjectRootRefused FAILs — relative-base guard absent:\n%s", tail(out, 30))
	}
	if !topLevelPassed(out, "TestGitWorktree_RelativeProjectRootRefused") {
		t.Errorf("RED: no `--- PASS: TestGitWorktree_RelativeProjectRootRefused` — the IsAbs guard is not in gitWorktree.Create")
	}
}
