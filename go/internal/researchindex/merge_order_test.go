package researchindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func branchWithEntry(t *testing.T, repo *gittest.Repo, branch, name, body string) {
	t.Helper()
	repo.Git("checkout", "-q", "-b", branch, "main")
	repo.Write(EntriesDir+"/"+name, body)
	if _, err := Generate(repo.Dir); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "docs: "+name)
	repo.Git("checkout", "-q", "main")
}

func mergeInOrder(t *testing.T, first, second string) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(IndexPath, staticHead)
	repo.Write(EntriesDir+"/2026-10-20-base.md", "## 2026-10-20 — base\n")
	if _, err := Generate(repo.Dir); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	branchWithEntry(t, repo, "a", "2026-10-21-a.md", entryA)
	branchWithEntry(t, repo, "b", "2026-10-21-b.md", entryB)
	repo.Git("merge", "-q", "--no-edit", first)
	conflicted := exec.Command("git", "-C", repo.Dir, "merge", "-q", "--no-edit", second).Run() != nil
	merged, err := os.ReadFile(filepath.Join(repo.Dir, IndexPath))
	if err != nil {
		t.Fatal(err)
	}
	if conflicted && !derived.IsDerivedConflict(IndexPath, merged) {
		t.Fatalf("merging %s after %s left a conflict that is not inside the generated region:\n%s", second, first, merged)
	}
	if _, err := Generate(repo.Dir); err != nil {
		t.Fatal(err)
	}
	if err := Check(repo.Dir); err != nil {
		t.Fatalf("Check after regeneration = %v", err)
	}
	return readIndex(t, repo.Dir)
}

func TestTwoEntryPRs_MergeInEitherOrderToOneIndexWithNoHandMerge(t *testing.T) {
	ab := mergeInOrder(t, "a", "b")
	ba := mergeInOrder(t, "b", "a")
	if ab != ba {
		t.Fatalf("the merge order changed the index:\na then b:\n%s\nb then a:\n%s", ab, ba)
	}
}
