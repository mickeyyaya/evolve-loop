package inboxcurator

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	pr1220ItemID  = "reviewer-profiles-may-write-their-report"
	pr1220Message = "Merge PR #1220 (fix/reviewer-profiles-write-report) into wave-145 boundary batch"
)

func TestMatchLanding_AClosesInboxTrailerInAMergedBranchIsADeterministicConsume(t *testing.T) {
	change := Change{Commit: "c1220", Text: "Merge PR #1220 into the boundary batch", Closes: []string{"rolling-pool"}}
	other := item("pool", "other work", []string{"go/x.go"})
	other.Path = "2026-10-18T00-00-00Z-pool.json"

	got := MatchLanding(change, []inboxbatch.Item{poolOpenItem(), other}, testsIn(), 0.5)

	if len(got) != 1 {
		t.Fatalf("MatchLanding = %+v, want one proposal, for rolling-pool only", got)
	}
	p := got[0]
	if p.ID != "rolling-pool" || p.Kind != ProposeConsume || !p.Deterministic || p.Trigger != TriggerLanding || p.Path != poolOpenItem().Path {
		t.Errorf("proposal = %+v, want the deterministic landing consume of rolling-pool", p)
	}
	if !slices.Contains(p.Evidence, "commit c1220") {
		t.Errorf("evidence = %v, want it to name the commit", p.Evidence)
	}
}

func TestMatchLanding_ATrailerThatClosesNoOpenItemProposesNothing(t *testing.T) {
	open := []inboxbatch.Item{item("rolling-pool", "x", nil)}

	for name, closes := range map[string][]string{
		"a nil set":           nil,
		"an empty set":        {},
		"the id of a sibling": {"rolling-pool-again"},
		"the id in a prefix":  {"rolling"},
	} {
		if got := MatchLanding(Change{Commit: "c", Closes: closes}, open, testsIn(), 0.5); len(got) != 0 {
			t.Errorf("%s: MatchLanding = %+v, want no proposal", name, got)
		}
	}
}

func branchOfBodies(repo *gittest.Repo, name string, bodies ...string) {
	repo.Git("checkout", "-q", "-b", name)
	for i, body := range bodies {
		repo.Write(name+"-"+string(rune('a'+i))+".txt", body+"\n")
		repo.Git("add", ".")
		repo.Git("commit", "-q", "-m", "work on "+name, "-m", body)
	}
}

func mergeFixture(t *testing.T, branches map[string][]string, order ...string) (repo *gittest.Repo, trunk, base string) {
	t.Helper()
	repo = gittest.Fixture(t)
	repo.Write("README.md", "x\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	trunk = repo.Git("rev-parse", "--abbrev-ref", "HEAD")
	base = repo.Git("rev-parse", "HEAD")
	for _, name := range order {
		repo.Git("checkout", "-q", trunk)
		branchOfBodies(repo, name, branches[name]...)
	}
	repo.Git("checkout", "-q", trunk)
	return repo, trunk, base
}

func commitOf(t *testing.T, repo *gittest.Repo, sha string) Change {
	t.Helper()
	got, err := Commit(context.Background(), gitexec.Default(repo.Dir), sha)
	if err != nil || len(got) != 1 {
		t.Fatalf("Commit(%s) = %+v, %v; want one change", sha, got, err)
	}
	return got[0]
}

func TestCommit_PR1220ReplayAMergeCommitReadsTheClosesIdOfItsMergedBranchBodies(t *testing.T) {
	repo, _, _ := mergeFixture(t, map[string][]string{
		"fix/reviewer-profiles-write-report": {
			"The five diff review seats put Write in disallowed_tools.",
			"Closes-Inbox: " + pr1220ItemID,
			"Closes-Inbox: " + pr1220ItemID,
		},
	}, "fix/reviewer-profiles-write-report")
	repo.Git("merge", "--no-ff", "-q", "-m", pr1220Message, "fix/reviewer-profiles-write-report")

	change := commitOf(t, repo, repo.Git("rev-parse", "HEAD"))

	if !slices.Equal(change.Closes, []string{pr1220ItemID}) {
		t.Fatalf("Closes = %v, want the id from the merged branch once", change.Closes)
	}
	if change.Text != pr1220Message || strings.Contains(change.Text, pr1220ItemID) {
		t.Errorf("Text = %q, want the merge message alone, which names no id", change.Text)
	}
	if !slices.Contains(change.Paths, "fix/reviewer-profiles-write-report-b.txt") {
		t.Errorf("Paths = %v, want the files that the merge brought in", change.Paths)
	}
	open := item(pr1220ItemID, "Each profile may write its report", []string{"go/never/touched.go"})
	got := MatchLanding(change, []inboxbatch.Item{open}, testsIn(), 0.5)
	if len(got) != 1 || !got[0].Deterministic || got[0].Kind != ProposeConsume || got[0].ID != pr1220ItemID {
		t.Errorf("MatchLanding = %+v, want the deterministic consume of the item", got)
	}
}

func TestCurate_APR1220ReplayThroughTheLandingEntryConsumesTheItem(t *testing.T) {
	repo, _, base := mergeFixture(t, map[string][]string{
		"fix/reviewer-profiles-write-report": {"Closes-Inbox: " + pr1220ItemID},
	}, "fix/reviewer-profiles-write-report")
	repo.Git("merge", "--no-ff", "-q", "-m", pr1220Message, "fix/reviewer-profiles-write-report")
	changes, err := Landed(context.Background(), gitexec.Default(repo.Dir), base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	got := Curate(Inputs{Changes: changes, Open: []inboxbatch.Item{item(pr1220ItemID, "x", nil)}, MinShare: 0.5, HasTest: testsIn()})

	if len(got) != 1 || got[0].Kind != ProposeConsume || got[0].ID != pr1220ItemID || !got[0].Deterministic {
		t.Fatalf("Curate = %+v, want the deterministic consume of %s", got, pr1220ItemID)
	}
}

func TestLanded_AMergeReadsOnlyItsMergedBranchAndNeverTheFirstParentHistory(t *testing.T) {
	repo, trunk, base := mergeFixture(t, map[string][]string{
		"feature": {"Closes-Inbox: branch-item"},
	}, "feature")
	repo.Write("trunk.txt", "x\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "trunk work", "-m", "Closes-Inbox: trunk-item")
	repo.Git("merge", "--no-ff", "-q", "-m", "Merge feature", "feature")

	got, err := Landed(context.Background(), gitexec.Default(repo.Dir), base, trunk)

	if err != nil || len(got) != 2 {
		t.Fatalf("Landed = %+v, %v; want the merge and the trunk commit", got, err)
	}
	if !slices.Equal(got[0].Closes, []string{"branch-item"}) {
		t.Errorf("merge Closes = %v, want only branch-item", got[0].Closes)
	}
	if !slices.Equal(got[1].Closes, []string{"trunk-item"}) {
		t.Errorf("trunk commit Closes = %v, want only trunk-item", got[1].Closes)
	}
}

func TestCommit_AnOctopusMergeReadsEveryMergedBranch(t *testing.T) {
	repo, _, _ := mergeFixture(t, map[string][]string{
		"left":  {"Closes-Inbox: left-item"},
		"right": {"Closes-Inbox: right-item, left-item"},
	}, "left", "right")
	repo.Git("merge", "--no-ff", "-q", "-m", "Merge both", "left", "right")

	change := commitOf(t, repo, repo.Git("rev-parse", "HEAD"))

	got := slices.Clone(change.Closes)
	slices.Sort(got)
	if !slices.Equal(got, []string{"left-item", "right-item"}) {
		t.Errorf("Closes = %v, want each id once across both branches", change.Closes)
	}
}

func TestCommit_AnOrdinaryCommitCarriesTheClosesIdsOfItsOwnBody(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"two ids":              {"Closes-Inbox: a-item, b-item", []string{"a-item", "b-item"}},
		"none is no id":        {"Closes-Inbox: none", nil},
		"a refs line is no id": {"Refs-Inbox: a-item", nil},
		"no trailer":           {"just prose", nil},
	} {
		repo := gittest.Fixture(t)
		repo.Write("a.txt", "x\n")
		repo.Git("add", ".")
		repo.Git("commit", "-q", "-m", "work", "-m", tc.body)

		change := commitOf(t, repo, repo.Git("rev-parse", "HEAD"))

		if !slices.Equal(change.Closes, tc.want) {
			t.Errorf("%s: Closes = %v, want %v", name, change.Closes, tc.want)
		}
	}
}

func TestCurate_AMergedBranchThatDeclaresNoneOrOnlyRefsConsumesNothing(t *testing.T) {
	repo, _, base := mergeFixture(t, map[string][]string{
		"quiet": {"Closes-Inbox: none", "Refs-Inbox: rolling-pool"},
	}, "quiet")
	repo.Git("merge", "--no-ff", "-q", "-m", "Merge quiet", "quiet")
	changes, err := Landed(context.Background(), gitexec.Default(repo.Dir), base, "HEAD")
	if err != nil || len(changes) != 1 {
		t.Fatalf("Landed = %+v, %v", changes, err)
	}

	got := Curate(Inputs{Changes: changes, Open: []inboxbatch.Item{item("rolling-pool", "x", nil)}, MinShare: 0.5, HasTest: testsIn()})

	if len(changes[0].Closes) != 0 || len(got) != 0 {
		t.Errorf("Closes = %v, Curate = %+v; want an empty set and no proposal", changes[0].Closes, got)
	}
}
