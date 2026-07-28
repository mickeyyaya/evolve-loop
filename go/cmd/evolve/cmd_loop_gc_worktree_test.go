package main

// cmd_loop_gc_worktree_test.go — RED tests for workspace-hygiene S5
// (cycle 1159, task workspace-hygiene-s5-wire-gc-hook). S4 built
// gc.PlanWorktrees/gc.ApplyWorktrees fully tested but with ZERO non-test
// callers, and runGCHook still defaults an absent gc.mode to "off" — so the
// worktree+branch sweep can never fire in production. These tests call the
// unexported runGCHook against a REAL temp git repo and assert on observable
// side effects (manifest published / branch actually deleted / tree untouched).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - runGCHook: an ABSENT gc.mode resolves to "shadow", not "off"
//     (docs/plans/workspace-hygiene-2026-07.md §S5). Explicit "off" still
//     returns immediately.
//   - In shadow and enforce, runGCHook additionally runs gc.PlanWorktrees over
//     cfg.ProjectRoot and publishes the manifest to
//     <workspace>/workspace-gc-manifest.json (tmp+rename, valid
//     gc.WorktreeManifest JSON).
//   - In enforce ONLY, it then calls gc.ApplyWorktrees — a merged, worktree-less
//     cycle-* branch is really deleted. Shadow must mutate nothing.
//   - Fail-open throughout: a non-git ProjectRoot warns and skips the worktree
//     sweep without breaking the run-dir GC that already works.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

// workspaceGCManifestName is the S5 worktree-sweep manifest, distinct from the
// run-dir gc-shadow-manifest.json the L3.4 hook already writes.
const workspaceGCManifestName = "workspace-gc-manifest.json"

// gcOrphanBranch is a merged cycle-* branch with no worktree — PlanWorktrees'
// "merged orphan branch (no worktree)" delete-branch case, reachable without
// provisioning real worktrees.
const gcOrphanBranch = "cycle-777"

// gcGit runs git in dir with a hermetic identity/config so the test never
// depends on (or mutates) the operator's global git config.
func gcGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=evolve-test", "GIT_AUTHOR_EMAIL=test@evolve.local",
		"GIT_COMMITTER_NAME=evolve-test", "GIT_COMMITTER_EMAIL=test@evolve.local",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// gcWorktreeEnv builds a real single-commit git repo whose .evolve/ carries the
// requested gc policy JSON (mode omitted entirely when mode == ""), plus one
// merged orphan cycle-* branch the sweep must plan for deletion. Returns
// (projectRoot, evolveDir, workspace).
func gcWorktreeEnv(t *testing.T, mode string) (projectRoot, evolveDir, workspace string) {
	t.Helper()
	projectRoot = t.TempDir()
	workspace = t.TempDir()
	gcGit(t, projectRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(projectRoot, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcGit(t, projectRoot, "add", "seed.txt")
	gcGit(t, projectRoot, "commit", "-m", "seed")
	// A merged, worktree-less cycle-* branch: exactly the backlog PlanWorktrees
	// is meant to reap and ApplyWorktrees is meant to `git branch -d`.
	gcGit(t, projectRoot, "branch", gcOrphanBranch)

	evolveDir = filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	pol := `{"gc":{"runs":{"keep_full":1,"delete_after_days":1}}}`
	if mode != "" {
		pol = `{"gc":{"mode":"` + mode + `","runs":{"keep_full":1,"delete_after_days":1}}}`
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectRoot, evolveDir, workspace
}

// gcReadWorktreeManifest decodes <workspace>/workspace-gc-manifest.json.
func gcReadWorktreeManifest(t *testing.T, workspace string) (gc.WorktreeManifest, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspace, workspaceGCManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return gc.WorktreeManifest{}, false
		}
		t.Fatal(err)
	}
	var m gc.WorktreeManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s is not a valid gc.WorktreeManifest: %v (raw=%s)", workspaceGCManifestName, err, raw)
	}
	return m, true
}

// gcBranchExists reports whether branch is still present in the repo.
func gcBranchExists(t *testing.T, projectRoot, branch string) bool {
	t.Helper()
	return strings.TrimSpace(gcGit(t, projectRoot, "branch", "--list", branch)) != ""
}

func gcManifestPlansBranchDelete(m gc.WorktreeManifest, branch string) bool {
	for _, it := range m.Items {
		if it.Branch == branch && it.Action == gc.WorktreeActionDeleteBranch {
			return true
		}
	}
	return false
}

// TestRunGCHook_DefaultModeIsShadow: policy.json carries NO gc.mode. The S5
// default flip means the hook must run in shadow — it publishes both manifests
// and mutates nothing. RED today: mode "" resolves to "off" and the hook
// returns before writing anything.
func TestRunGCHook_DefaultModeIsShadow(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "") // gc.mode absent

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, &buf)

	if _, ok := gcReadManifest(t, workspace); !ok {
		t.Errorf("absent gc.mode must default to shadow and write %s; stderr=%q", gcManifestName, buf.String())
	}
	if _, ok := gcReadWorktreeManifest(t, workspace); !ok {
		t.Errorf("absent gc.mode must default to shadow and write %s; stderr=%q", workspaceGCManifestName, buf.String())
	}
	if !gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("shadow default must NOT mutate: branch %s was deleted", gcOrphanBranch)
	}
}

// TestRunGCHook_ShadowWritesWorkspaceManifest: shadow mode plans the worktree/
// branch sweep and publishes it, without touching the repo. The manifest must
// actually name the merged orphan branch — an empty-but-present manifest would
// pass a mere existence check while proving PlanWorktrees never really ran.
func TestRunGCHook_ShadowWritesWorkspaceManifest(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "shadow")

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, &buf)

	m, ok := gcReadWorktreeManifest(t, workspace)
	if !ok {
		t.Fatalf("shadow mode must write %s; stderr=%q", workspaceGCManifestName, buf.String())
	}
	if !gcManifestPlansBranchDelete(m, gcOrphanBranch) {
		t.Errorf("manifest must plan delete-branch for merged orphan %s; items=%+v", gcOrphanBranch, m.Items)
	}
	if !gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("shadow mode must NOT mutate: branch %s was deleted", gcOrphanBranch)
	}
}

// TestRunGCHook_EnforceAppliesWorktreeSweep: enforce mode publishes the same
// manifest AND applies it — the merged orphan branch is really gone.
func TestRunGCHook_EnforceAppliesWorktreeSweep(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "enforce")

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, &buf)

	if _, ok := gcReadWorktreeManifest(t, workspace); !ok {
		t.Errorf("enforce mode must also write %s; stderr=%q", workspaceGCManifestName, buf.String())
	}
	if gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("enforce mode must DELETE merged orphan branch %s; stderr=%q", gcOrphanBranch, buf.String())
	}
}

// TestRunGCHook_ExplicitOffSkipsWorktreeSweep is the negative case: an operator
// who pins gc.mode="off" keeps the old no-op behavior. Without this, "always
// sweep" would pass every positive test above.
func TestRunGCHook_ExplicitOffSkipsWorktreeSweep(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "off")

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: projectRoot}, workspace, &buf)

	if _, ok := gcReadWorktreeManifest(t, workspace); ok {
		t.Errorf("explicit gc.mode=off must NOT write %s", workspaceGCManifestName)
	}
	if _, ok := gcReadManifest(t, workspace); ok {
		t.Errorf("explicit gc.mode=off must NOT write %s", gcManifestName)
	}
	if !gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("explicit gc.mode=off must not mutate: branch %s was deleted", gcOrphanBranch)
	}
}

// TestRunGCHook_NonGitProjectRootIsFailOpen: the worktree sweep is best-effort.
// A ProjectRoot that is not a git repo must warn and leave the pre-existing
// run-dir GC intact — never panic, never abort the batch-end hook.
func TestRunGCHook_NonGitProjectRootIsFailOpen(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "shadow")

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}, workspace, &buf) // must not panic

	if _, ok := gcReadManifest(t, workspace); !ok {
		t.Errorf("run-dir GC must still publish %s when the worktree sweep cannot run; stderr=%q", gcManifestName, buf.String())
	}
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("shadow mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
}
