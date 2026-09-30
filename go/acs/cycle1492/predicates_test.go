//go:build acs

package cycle1492

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runFromGoRoot(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", cmd.Args, err)
	}
	return string(out), code
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestC1492_001_FreshBaseTreeIsNeverALookupKey(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("base content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "seed.txt")
	git(t, repo, "commit", "-q", "-m", "base")

	baseTree := git(t, repo, "rev-parse", "HEAD^{tree}")
	freshTree := git(t, repo, "write-tree")
	if freshTree != baseTree {
		t.Fatalf("fixture invalid: fresh worktree tree %q != base tree %q", freshTree, baseTree)
	}
	if verdictcache.ProbeEligible(baseTree, freshTree) {
		t.Errorf("RED: ProbeEligible(base=%s, fresh=%s) = true — an untouched worktree is a "+
			"lookup key, so every sibling lane from this tip reuses the same stale verdict",
			baseTree, freshTree)
	}

	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression/clean_cached_base_is_suppressed", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: fresh-base suppression oracle failed on the real RunCycle path (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: the fresh-base suppression subtest did not run — the caller proof is vacuous:\n%s", out)
	}
}

func TestC1492_002_ChangedWorktreeStaysEligible(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("base content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "seed.txt")
	git(t, repo, "commit", "-q", "-m", "base")
	baseTree := git(t, repo, "rev-parse", "HEAD^{tree}")

	if err := os.WriteFile(filepath.Join(repo, "built.txt"), []byte("builder output\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	changedTree := git(t, repo, "write-tree")
	if changedTree == baseTree {
		t.Fatalf("fixture invalid: changed worktree tree still equals base tree %q", baseTree)
	}
	if !verdictcache.ProbeEligible(baseTree, changedTree) {
		t.Errorf("RED: ProbeEligible(base=%s, changed=%s) = false — real post-build content is "+
			"no longer cacheable, the guard was widened into a blanket disable",
			baseTree, changedTree)
	}

	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheCollisionRegression/dirty_cache_hit_remains_observable", "./internal/core"))
	if code != 0 {
		t.Errorf("RED: changed-worktree advisory lookup no longer observable on the real RunCycle path (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: the changed-worktree subtest did not run — the anti-gaming proof is vacuous:\n%s", out)
	}

	wire, wcode := runFromGoRoot(t, exec.Command("go", "test", "-tags", "integration", "-count=1",
		"-run", "TestVerdictCacheProbeEligibilityWiring", "./internal/core"))
	if wcode != 0 {
		t.Errorf("RED: orchestrator probe decision diverged from verdictcache.ProbeEligible (rc=%d):\n%s", wcode, wire)
	}
}

func TestC1492_003_LaneACSSuiteIsGreen(t *testing.T) {
	out, code := runFromGoRoot(t, exec.Command("go", "test", "-tags", "acs", "-count=1", "./acs/cycle1488"))
	if code != 0 {
		t.Errorf("RED: the lane's carried ACS suite (acs/cycle1488) is red, so EGPS red_count > 0 "+
			"and the fresh-base guard cannot ship (rc=%d):\n%s", code, out)
	}
	if strings.Contains(out, "[no test files]") {
		t.Errorf("RED: acs/cycle1488 built no tests — the suite check is vacuous:\n%s", out)
	}
}
