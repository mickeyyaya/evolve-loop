package rawgitratchet

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

func rawGitTest(inits int) string {
	return "package p\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) {\n\t_ = []string{" + gitLit + strings.Repeat(", "+initLit, inits) + "}\n}\n"
}

func commitAll(t *testing.T, repo *gittest.Repo, message string) string {
	t.Helper()
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", message)
	return repo.Git("rev-parse", "HEAD")
}

func TestTightenModule_TwoLanesThatShrinkSitesInDifferentPackagesRebaseIdentically(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("p/p_test.go", rawGitTest(2))
	repo.Write("q/q_test.go", rawGitTest(2))
	repo.Write(BaselineRelPath, "{\n  \"p/p_test.go\": 2,\n  \"q/q_test.go\": 2\n}\n")
	base := commitAll(t, repo, "base")
	repo.Write("p/p_test.go", rawGitTest(1))
	laneA := commitAll(t, repo, "lane a removes a raw git site in p")
	if _, err := Scan(repo.Dir); err != nil {
		t.Fatalf("lane a passes without an edit to baseline.json: %v", err)
	}
	repo.Git("checkout", "-q", "-b", "lane-b", base)
	repo.Write("q/q_test.go", passTest)
	laneB := commitAll(t, repo, "lane b migrates q off raw git")
	if _, err := Scan(repo.Dir); err != nil {
		t.Fatalf("lane b passes without an edit to baseline.json: %v", err)
	}
	repo.Git("cherry-pick", laneA)
	rebased := repo.Git("rev-parse", "HEAD")
	git := func(_ context.Context, _ string, args ...string) (string, int, error) {
		return repo.Git(args...), 0, nil
	}

	audited, composed, ok, err := treedelta.Identical(context.Background(), git, repo.Dir, base, laneA, laneB, rebased)

	if err != nil || !ok {
		t.Fatalf("the rebase of lane a over lane b is not identical (err=%v):\naudited:\n%s\ncomposed:\n%s", err, audited, composed)
	}
	changes, encoded, err := TightenModule(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"p/p_test.go: 2 -> 1", "q/q_test.go: 2 removed"}; !reflect.DeepEqual(changes, want) {
		t.Errorf("the boundary tighten = %v, want %v", changes, want)
	}
	if want := "{\n  \"p/p_test.go\": 1\n}\n"; string(encoded) != want {
		t.Errorf("the boundary tighten encodes %q, want %q", encoded, want)
	}
}

func TestTightenModule_EachStageFailureIsReturned(t *testing.T) {
	empty := t.TempDir()
	noGitAbove(t, empty)
	if _, _, err := TightenModule(empty); err == nil {
		t.Error("a module with no test file tightened")
	}
	noBaseline := t.TempDir()
	writeFiles(t, noBaseline, map[string]string{"p/p_test.go": passTest})
	if _, _, err := TightenModule(noBaseline); err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Errorf("err = %v, want the missing baseline named", err)
	}
	broken := t.TempDir()
	writeFiles(t, broken, map[string]string{"p/p_test.go": "package p\nfunc {", filepath.ToSlash(BaselineRelPath): "{}"})
	if _, _, err := TightenModule(broken); err == nil {
		t.Error("an unparsable test file tightened")
	}
}
