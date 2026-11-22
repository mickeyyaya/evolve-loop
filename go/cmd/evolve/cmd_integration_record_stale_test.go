package main

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type pushedBehindPR struct {
	stale string
	tip   string
}

func (j *trainJourney) pushedBehindPR(t *testing.T, n int) pushedBehindPR {
	t.Helper()
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", "-b", name, "main")
	j.work.Write("a.md", "a\n")
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "feat: a")
	stale := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Write("b.md", "b\n")
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "feat: b")
	tip := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("push", "-q", "origin", name+":refs/heads/"+name)
	j.work.Git("push", "-q", "origin", stale+":refs/pull/"+strconv.Itoa(n)+"/head")
	j.work.Git("checkout", "-q", "main")
	return pushedBehindPR{stale: stale, tip: tip}
}

func (j *trainJourney) reportHeadBranch(n int) {
	inner := j.env.Gh
	prefix := "pr view " + strconv.Itoa(n) + " "
	name := "pr-" + strconv.Itoa(n)
	j.env.Gh = func(ctx context.Context, dir string, args ...string) (string, int, error) {
		if !strings.HasPrefix(strings.Join(args, " "), prefix) {
			return inner(ctx, dir, args...)
		}
		oid := strings.TrimSpace(j.origin.Git("rev-parse", "refs/heads/"+name))
		return `{"state":"OPEN","number":` + strconv.Itoa(n) + `,"headRefName":"` + name + `","headRefOid":"` + oid + `"}` + "\n", 0, nil
	}
}

func TestRecordPRStalePullRefExitsRefusedNamingBothSHAs(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	pr := j.pushedBehindPR(t, 4)
	j.reportHeadBranch(4)
	rc, _, errb := j.run("record-pr", "--pr", "4", "--class", "docs-review")
	if rc != exitRefused {
		t.Fatalf("rc=%d err=%q, want %d for a pull ref behind its head branch", rc, errb, exitRefused)
	}
	for name, sha := range map[string]string{"the pull ref": pr.stale, "the head branch": pr.tip} {
		if !strings.Contains(errb, integration.ShortSHA(sha)) {
			t.Errorf("stderr does not name the SHA of %s (%s): %q", name, integration.ShortSHA(sha), errb)
		}
	}
	if _, ok := integration.FindPRRecord(j.env.Root, 4); ok {
		t.Error("a stale pull ref left a record")
	}
}

func TestRecordPRStalePullRefLeavesACurrentPullRefAlone(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	pr := j.pushedBehindPR(t, 4)
	j.origin.Git("update-ref", "refs/pull/4/head", pr.tip)
	j.reportHeadBranch(4)
	if rc, _, errb := j.run("record-pr", "--pr", "4", "--class", "docs-review"); rc != 0 {
		t.Fatalf("rc=%d err=%q, want 0 for a pull ref equal to its head branch", rc, errb)
	}
	rec, ok := integration.FindPRRecord(j.env.Root, 4)
	if !ok || len(rec.Commits) != 2 {
		t.Errorf("record = %+v, %v, want two commits", rec.Commits, ok)
	}
}
