package repostate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRepo(t *testing.T) (string, func(...string)) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	return root, git
}

func write(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedSet_BindsTrackedExcludesUntrackedAndNested(t *testing.T) {
	root, git := fixtureRepo(t)
	write(t, root, ".evolve/profiles/auditor.json")
	git("add", ".evolve/profiles/auditor.json")
	write(t, root, ".evolve/profiles/minted-stub.json") // untracked
	write(t, root, ".evolve/profiles/archive/scout.json")
	git("add", ".evolve/profiles/archive/scout.json") // nested tracked

	set, err := TrackedSet(root, ".evolve/profiles", ".json")
	if err != nil {
		t.Fatal(err)
	}
	if !set["auditor"] {
		t.Error("tracked top-level profile missing from the binding set")
	}
	if set["minted-stub"] {
		t.Error("untracked stub is runtime state and must not be bound (cd49274beab2 class)")
	}
	if set["scout"] {
		t.Error("nested tracked file must not basename-alias into the top-level binding set")
	}
}

func TestTrackedSet_StagedUncommittedCounts(t *testing.T) {
	root, git := fixtureRepo(t)
	write(t, root, ".evolve/profiles/fresh.json")
	git("add", ".evolve/profiles/fresh.json")
	set, err := TrackedSet(root, ".evolve/profiles", ".json")
	if err != nil {
		t.Fatal(err)
	}
	if !set["fresh"] {
		t.Error("index-staged file must count as tracked (stricter direction, no plane-vs-CI skew)")
	}
}

func TestTrackedFiles_NonRepoErrorCarriesGitStderr(t *testing.T) {
	_, err := TrackedFiles(t.TempDir(), ".evolve/profiles")
	if err == nil {
		t.Fatal("non-repo dir must error so callers can fall back to strict bind-all")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q must carry git's stderr reason, not just the exit status", err)
	}
}

func TestTrackedSet_EmptyDirIsSuccessWithEmptySet(t *testing.T) {
	root, _ := fixtureRepo(t)
	write(t, root, ".evolve/profiles/only-minted.json") // untracked
	set, err := TrackedSet(root, ".evolve/profiles", ".json")
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 0 {
		t.Errorf("set = %v, want empty — callers own the loud empty-set fallback decision", set)
	}
}
