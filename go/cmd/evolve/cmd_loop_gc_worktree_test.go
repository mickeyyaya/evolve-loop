package main

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

const workspaceGCManifestName = "workspace-gc-manifest.json"

// gcOrphanBranch is a merged cycle-* branch with no worktree — PlanWorktrees'
// "merged orphan branch (no worktree)" delete-branch case, reachable without
// provisioning real worktrees.
const gcOrphanBranch = "cycle-777"

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

func gcWorktreeEnv(t *testing.T, mode string) (projectRoot, evolveDir, workspace string) {
	t.Helper()
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("GOCACHE", t.TempDir())
	projectRoot = t.TempDir()
	workspace = t.TempDir()
	gcGit(t, projectRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(projectRoot, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcGit(t, projectRoot, "add", "seed.txt")
	gcGit(t, projectRoot, "commit", "-m", "seed")
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

func TestRunGCHook_DefaultModeIsShadow(t *testing.T) {
	projectRoot, evolveDir, workspace := gcWorktreeEnv(t, "")

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

// The manifest must name the merged orphan branch: an empty-but-present
// manifest would pass a mere existence check without PlanWorktrees running.
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

// The negative case: without it, an "always sweep" implementation would pass
// every positive test above.
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

func TestRunGCHook_NonGitProjectRootIsFailOpen(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "shadow")

	var buf bytes.Buffer
	runGCHook(loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}, workspace, &buf)

	if _, ok := gcReadManifest(t, workspace); !ok {
		t.Errorf("run-dir GC must still publish %s when the worktree sweep cannot run; stderr=%q", gcManifestName, buf.String())
	}
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("shadow mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
}
