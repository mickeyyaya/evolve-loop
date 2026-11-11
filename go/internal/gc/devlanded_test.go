package gc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPlanDevWorktrees_ALandedTreeSkipsTheIdleLimitOnlyWhenEveryRuleHolds(t *testing.T) {
	cases := []struct {
		name    string
		prs     OpenPRLookup
		arrange func(e *devEnv, dir string)
		want    DevAction
		reason  string
	}{
		{"landed, clean, no pull request", noOpenPRs, nil, DevActionArchive, "landed early: branch br-tree is in main"},
		{"no pull request lookup", nil, nil, DevActionKeep, "active within the 14-day limit"},
		{"gh unavailable", func(context.Context, string) (map[string][]time.Time, error) {
			return nil, ErrPRLookupUnavailable
		}, nil, DevActionKeep, "active within the 14-day limit"},
		{"a failing lookup", func(context.Context, string) (map[string][]time.Time, error) {
			return nil, context.DeadlineExceeded
		}, nil, DevActionKeep, "landed check: " + context.DeadlineExceeded.Error()},
		{"untracked file", noOpenPRs, func(e *devEnv, dir string) { writeRepoFile(e.t, dir, "new.txt", "x\n") }, DevActionKeep, "active within the 14-day limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newDevEnv(t)
			dir := e.addDev("tree")
			e.commitIn(dir, "work.txt", "work\n")
			e.store.Git("update-ref", "refs/remotes/origin/main", e.store.Git("-C", dir, "rev-parse", "HEAD"))
			if tc.arrange != nil {
				tc.arrange(e, dir)
			}
			o := e.options(0)
			o.OpenPRs = tc.prs
			m, err := PlanDevWorktrees(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			it := m.Items[0]
			if it.Action != tc.want || !strings.Contains(it.Reason, tc.reason) {
				t.Errorf("item = %+v, want action %s with a reason that holds %q", it, tc.want, tc.reason)
			}
		})
	}
}

func TestHasOwnWork_ReadsTheOldestReflogEntryOfTheTree(t *testing.T) {
	e := newDevEnv(t)
	fresh := e.addDev("fresh")
	worked := e.addDev("worked")
	e.commitIn(worked, "w.txt", "w\n")
	for dir, want := range map[string]bool{fresh: false, worked: true} {
		head := e.store.Git("-C", dir, "rev-parse", "HEAD")
		got, err := hasOwnWork(context.Background(), gitexec.Git{Dir: dir, Exec: sysexec.DefaultRunner}, head)
		if err != nil || got != want {
			t.Errorf("hasOwnWork(%s) = %v, %v; want %v", dir, got, err, want)
		}
	}
}

func mergedTipsOf(branch string, oids ...string) MergedPRLookup {
	return func(context.Context, string) (map[string][]string, error) {
		return map[string][]string{branch: oids}, nil
	}
}

func devTipOf(e *devEnv, dir string) string {
	return e.store.Git("-C", dir, "rev-parse", "HEAD")
}

func TestPlanDevWorktrees_ABranchWithAMergedPullRequestAtItsTipIsLandedAndALaterCommitIsNot(t *testing.T) {
	cases := []struct {
		name    string
		offset  time.Duration
		arrange func(e *devEnv, dir string) MergedPRLookup
		want    DevAction
		reason  string
	}{
		{"merged pull request at the tip, inside the idle limit", 0, func(e *devEnv, dir string) MergedPRLookup {
			return mergedTipsOf("br-tree", devTipOf(e, dir))
		}, DevActionArchive, "landed early: branch br-tree is in main"},
		{"a later commit on the merged branch, inside the idle limit", 0, func(e *devEnv, dir string) MergedPRLookup {
			merged := devTipOf(e, dir)
			e.commitIn(dir, "later.txt", "later\n")
			return mergedTipsOf("br-tree", merged)
		}, DevActionKeep, "active within the 14-day limit"},
		{"merged pull request at the tip, past the idle limit", idleOffset, func(e *devEnv, dir string) MergedPRLookup {
			return mergedTipsOf("br-tree", devTipOf(e, dir))
		}, DevActionArchive, "branch br-tree is in main and stays"},
		{"a later commit on the merged branch, past the idle limit", idleOffset, func(e *devEnv, dir string) MergedPRLookup {
			merged := devTipOf(e, dir)
			e.commitIn(dir, "later.txt", "later\n")
			return mergedTipsOf("br-tree", merged)
		}, DevActionArchive, "branch br-tree is not in main and stays"},
		{"a merged pull request of another branch at the same tip", 0, func(e *devEnv, dir string) MergedPRLookup {
			return mergedTipsOf("br-other", devTipOf(e, dir))
		}, DevActionKeep, "active within the 14-day limit"},
		{"two merged pull requests on a reused branch name, the second at the tip", 0, func(e *devEnv, dir string) MergedPRLookup {
			return mergedTipsOf("br-tree", "0000000000000000000000000000000000000000", devTipOf(e, dir))
		}, DevActionArchive, "landed early: branch br-tree is in main"},
		{"a merged pull request listing an empty oid", 0, func(e *devEnv, dir string) MergedPRLookup {
			return mergedTipsOf("br-tree", "")
		}, DevActionKeep, "active within the 14-day limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newDevEnv(t)
			dir := e.addDev("tree")
			e.commitIn(dir, "work.txt", "work\n")
			o := e.options(tc.offset)
			o.MergedPRs = tc.arrange(e, dir)
			it := e.plan(o).Items[0]
			if it.Action != tc.want || !strings.Contains(it.Reason, tc.reason) {
				t.Errorf("item = %+v, want action %s with a reason that holds %q", it, tc.want, tc.reason)
			}
		})
	}
}

func TestPlanDevWorktrees_TheMergedPullRequestSourceIsReadOnceForTheWholeRun(t *testing.T) {
	e := newDevEnv(t)
	first := e.addDev("first")
	second := e.addDev("second")
	e.commitIn(first, "a.txt", "a\n")
	e.commitIn(second, "b.txt", "b\n")
	reads := 0
	o := e.options(0)
	o.MergedPRs = func(context.Context, string) (map[string][]string, error) {
		reads++
		return map[string][]string{"br-first": {devTipOf(e, first)}, "br-second": {devTipOf(e, second)}}, nil
	}
	m := e.plan(o)
	if reads != 1 {
		t.Errorf("the merged pull request source was read %d times, want exactly once for the run", reads)
	}
	for _, dir := range []string{first, second} {
		if it := devItemFor(t, m, dir); it.Action != DevActionArchive || !strings.Contains(it.Reason, "landed early") {
			t.Errorf("item for %s = %+v, want landed early from the one read", dir, it)
		}
	}
}

func TestPlanDevWorktrees_AFailedMergedPullRequestReadFallsBackToGitEvidenceAndSaysSo(t *testing.T) {
	cases := []struct {
		name string
		err  error
		note string
	}{
		{"gh is unavailable", ErrPRLookupUnavailable, "gh is unavailable: the merged pull request source was skipped, so landed rests on git evidence"},
		{"gh fails", context.DeadlineExceeded, "the merged pull request source failed, so landed rests on git evidence: " + context.DeadlineExceeded.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newDevEnv(t)
			byGit := e.addDev("by-git")
			e.commitIn(byGit, "g.txt", "g\n")
			e.store.Git("update-ref", "refs/remotes/origin/main", devTipOf(e, byGit))
			byPR := e.addDev("by-pr")
			e.commitIn(byPR, "p.txt", "p\n")
			o := e.options(0)
			o.MergedPRs = func(context.Context, string) (map[string][]string, error) { return nil, tc.err }
			m := e.plan(o)
			if it := devItemFor(t, m, byGit); it.Action != DevActionArchive || !strings.Contains(it.Reason, "landed early: branch br-by-git is in main") {
				t.Errorf("git evidence no longer lands a branch when the source fails: %+v", it)
			}
			if it := devItemFor(t, m, byPR); it.Action != DevActionKeep || it.Failed {
				t.Errorf("a failed source must neither land nor fail a branch git cannot place: %+v", it)
			}
			count := 0
			for _, n := range m.Notes {
				if n == tc.note {
					count++
				}
			}
			if count != 1 {
				t.Errorf("notes = %q, want %q exactly once", m.Notes, tc.note)
			}
		})
	}
}

func TestPlanDevWorktrees_NoMergedPullRequestSourceLeavesLandedToGitEvidenceWithoutANote(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("tree")
	e.commitIn(dir, "work.txt", "work\n")
	m := e.plan(e.options(0))
	if it := m.Items[0]; it.Action != DevActionKeep {
		t.Errorf("item = %+v, want keep", it)
	}
	for _, n := range m.Notes {
		if strings.Contains(n, "merged pull request") {
			t.Errorf("unexpected note %q with no source configured", n)
		}
	}
}
