package sizeratchet

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

func commitAll(t *testing.T, repo *gittest.Repo, message string) string {
	t.Helper()
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", message)
	return repo.Git("rev-parse", "HEAD")
}

func TestTighten_TwoLanesThatShrinkListedFunctionsInDifferentPackagesRebaseIdentically(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("p/p.go", fileWith("F", 60))
	repo.Write("q/q.go", fileWith("G", 60))
	repo.Write(OffendersRelPath, "{\n  \"p.F\": 60,\n  \"q.G\": 60\n}\n")
	base := commitAll(t, repo, "base")
	repo.Write("p/p.go", fileWith("F", 55))
	laneA := commitAll(t, repo, "lane a shrinks p.F")
	if err := Scan(repo.Dir); err != nil {
		t.Fatalf("lane a passes without an edit to the list: %v", err)
	}
	repo.Git("checkout", "-q", "-b", "lane-b", base)
	repo.Write("q/q.go", fileWith("G", 55))
	laneB := commitAll(t, repo, "lane b shrinks q.G")
	if err := Scan(repo.Dir); err != nil {
		t.Fatalf("lane b passes without an edit to the list: %v", err)
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
	if want := []string{"p.F: 60 -> 55", "q.G: 60 -> 55"}; !reflect.DeepEqual(changes, want) {
		t.Errorf("the boundary tighten = %v, want %v", changes, want)
	}
	if want := "{\n  \"p.F\": 55,\n  \"q.G\": 55\n}\n"; string(encoded) != want {
		t.Errorf("the boundary tighten encodes %q, want %q", encoded, want)
	}
}
