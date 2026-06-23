//go:build acs

// Package cycle295 materializes the cycle-295 acceptance criteria for the two
// committed top_n tasks (scout-report.md):
//
//	T1  checkpoint-clobber-fix             — FilesystemStorage.WriteCycleState
//	    whole-struct-replaces cycle-state.json, and core.CycleState has no
//	    "checkpoint" field, so every pre-dispatch write ERASES the "checkpoint"
//	    block PhaseBoundaryCheckpointer wrote after the prior phase. A crash
//	    mid-phase then leaves no checkpoint and `evolve loop --resume` fails
//	    (live incident: host reboot during cycle-294 mutation-gate). Fix: make
//	    WriteCycleState a read-merge-write that carries "checkpoint" through.
//	T2  core-worktree-relative-base-guard  — core/gitWorktree.Create() MkdirAll's
//	    the worktree base with NO filepath.IsAbs check (swarm/provision.go got
//	    that guard in cycle 294). A relative base silently creates dirs under cwd.
//	    Fix: add the same absolute-path guard before MkdirAll.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test:
//   - C295_001/002 call the REAL storage.FilesystemStorage.WriteCycleState and
//     assert on the REAL cycle-state.json bytes (checkpoint preserved / not
//     duplicated). A magic string in a .go file cannot make the on-disk JSON
//     keep a key the production write erases.
//   - C295_003/004 run the REAL core package tests (`go test -v -run ...`) that
//     drive the unexported gitWorktree.Create with a relative base and assert on
//     the real `--- PASS:` line. gitWorktree is unexported, so a subprocess test
//     run is the behavioral seam; a source string cannot produce a named PASS.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1 checkpoint preserved        → C295_001 (direct WriteCycleState call)
//	T1 no spurious / not dup'd     → C295_002 (direct WriteCycleState call)
//	T1 storage suite green         → manual+checklist (auditor)
//	T2 relative env base refused   → C295_003 (go test -run, PASS line)
//	T2 relative projectRoot refused→ C295_004 (go test -run, PASS line)
//	T2 core suite green            → manual+checklist (auditor)
package cycle295

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the module dir so `go test -C <goDir>` is cwd-independent (the
// audit lane may run from the worktree root or from go/).
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

// seedCheckpointedState writes a cycle-state.json carrying both CycleState
// fields and a "checkpoint" block, returning the .evolve dir. This is the
// pre-dispatch shape PhaseBoundaryCheckpointer leaves behind.
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

// ===================== T1 — checkpoint-clobber-fix ===========================

// --- C295_001 (T1.preserve): WriteCycleState carries the checkpoint through ---
//
// Behavioral: seeds a checkpointed cycle-state.json, calls the REAL
// storage.WriteCycleState for the next phase, and asserts the on-disk
// "checkpoint" block survives unchanged AND the new phase was written. The
// erase-vs-preserve outcome is a real file mutation no source string can fake.
//
// RED baseline: WriteCycleState whole-struct-replaces (no Checkpoint field) →
// the "checkpoint" key is gone after the call → this fails.
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

// --- C295_002 (T1.idempotent): the checkpoint key is never dropped or dup'd ---
//
// Behavioral: two REAL WriteCycleState calls over a seeded checkpointed state,
// then count the "checkpoint" key in the on-disk bytes. Exactly 1 is correct;
// 0 = erased (RED today), >1 = a splicing bug.
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

// ===================== T2 — core-worktree-relative-base-guard ================

// runCoreTest runs ONE named core test verbose, returning combined output.
// The core package tests drive the unexported gitWorktree.Create against a
// relative base inside an isolated non-git temp cwd, so this neither touches
// the live repo nor depends on a grep of source.
func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", name, "./internal/core/")
	return stdout + "\n" + stderr
}

// --- C295_003 (T2.envbase): a relative EVOLVE_WORKTREE_BASE is refused --------
//
// Gates on a real `--- PASS: TestGitWorktree_RelativeBaseRefused` line. That
// test calls gitWorktree.Create with a relative base and asserts the guard
// rejects it with an "absolute" error before any MkdirAll. RED: no guard → the
// test fails (git error lacks "absolute") → no PASS line.
func TestC295_003_RelativeWorktreeBaseRefused(t *testing.T) {
	out := runCoreTest(t, "TestGitWorktree_RelativeBaseRefused")
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: TestGitWorktree_RelativeBaseRefused FAILs — relative-base guard absent:\n%s", tail(out, 30))
	}
	if !topLevelPassed(out, "TestGitWorktree_RelativeBaseRefused") {
		t.Errorf("RED: no `--- PASS: TestGitWorktree_RelativeBaseRefused` — the IsAbs guard is not in gitWorktree.Create")
	}
}

// --- C295_004 (T2.rootbase): a relative projectRoot (no env) is refused -------
//
// Gates on a real `--- PASS: TestGitWorktree_RelativeProjectRootRefused` line —
// the live-default path (EVOLVE_WORKTREE_BASE unset, base = <root>/.evolve/
// worktrees) that silently created dirs in cwd. RED: no guard → no PASS line.
func TestC295_004_RelativeProjectRootRefused(t *testing.T) {
	out := runCoreTest(t, "TestGitWorktree_RelativeProjectRootRefused")
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: TestGitWorktree_RelativeProjectRootRefused FAILs — relative-base guard absent:\n%s", tail(out, 30))
	}
	if !topLevelPassed(out, "TestGitWorktree_RelativeProjectRootRefused") {
		t.Errorf("RED: no `--- PASS: TestGitWorktree_RelativeProjectRootRefused` — the IsAbs guard is not in gitWorktree.Create")
	}
}
