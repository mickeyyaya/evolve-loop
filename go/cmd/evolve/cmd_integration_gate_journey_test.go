package main

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type gatedCommit struct {
	file, body, id string
	gated          bool
}

func (j *trainJourney) commitThroughTheGate(t *testing.T, c gatedCommit) {
	t.Helper()
	j.work.Write(c.file, c.body)
	j.work.Git("add", c.file)
	if c.gated {
		tokens := []string{j.seat(t, j.work.Dir, "simp-"+c.id, "code-simplifier"), j.seat(t, j.work.Dir, "rev-"+c.id, "code-reviewer")}
		if rc, logs := j.gate(t, j.work.Dir, tokens...); rc != 0 {
			t.Fatalf("gate over %s rc=%d\n%s", c.id, rc, logs)
		}
	}
	j.work.Git("commit", "-q", "-m", "feat: "+c.id)
}

func (j *trainJourney) gatedPR(t *testing.T, n int, commits ...gatedCommit) {
	t.Helper()
	j.onPR(n, func() {
		for _, c := range commits {
			j.commitThroughTheGate(t, c)
		}
	})
}

func TestJourneyAPRCommittedThroughTheGateIsRecordedAndLandedByTheBatch(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.gatedPR(t, 6,
		gatedCommit{"a.txt", "a\n", "c1", true},
		gatedCommit{"b.txt", "b\n", "c2", true})
	before := j.originMain()

	rc, out, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	if rc != 0 || !strings.HasPrefix(out, "recorded PR #6: ") {
		t.Fatalf("record-pr rc=%d out=%q err=%q", rc, out, errb)
	}
	rec, ok := integration.FindPRRecord(j.work.Dir, 6)
	if !ok || rec.Audit.Kind != integration.KindCommitChain || len(rec.Commits) != 2 {
		t.Fatalf("record = %+v, %v, want a commit chain over two commits", rec, ok)
	}

	rc, out, errb = j.run("batch", "6")
	if rc != 0 || !strings.Contains(out, "landed: 6") {
		t.Fatalf("batch rc=%d out=%q err=%q", rc, out, errb)
	}
	after := j.originMain()
	if after == before || strings.TrimSpace(j.origin.Git("rev-list", "--count", before+".."+after)) != "2" || strings.TrimSpace(j.origin.Git("merge-base", before, after)) != before {
		t.Errorf("main must move by a fast-forward of two commits: %s -> %s", before, after)
	}
	if got := strings.TrimSpace(j.origin.Git("show", "main:b.txt")); got != "b" {
		t.Errorf("main lacks the second commit, b.txt = %q", got)
	}
	if !strings.Contains(j.gh.closed["6"], "#99") {
		t.Errorf("PR 6 close comment = %q, want a link to #99", j.gh.closed["6"])
	}
}

func TestJourneyAPRWhoseSecondCommitSkippedTheGateIsRefusedAndNamesThatCommit(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.gatedPR(t, 6,
		gatedCommit{"a.txt", "a\n", "c1", true},
		gatedCommit{"b.txt", "b\n", "c2", false})
	base, _, _, store := j.prFacts(t, "6")
	first := strings.TrimSpace(j.work.Git("rev-parse", "pr-6~1"))
	second := strings.TrimSpace(j.work.Git("rev-parse", "pr-6"))
	d2 := j.changeOf(t, first, second)
	before := j.originMain()
	want := j.refusal("6", "commit "+second[:7]+" (change "+d2[:7]+") has no passing review record: no review record at "+store+"/"+d2+".json. Next: in a worktree at "+first[:7]+", run git restore --source="+second[:7]+" --staged --worktree -- . , run the review seats and evolve commit-gate run, then run record-pr again.")

	rc, out, errb := j.run("record-pr", "--pr", "6", "--reviewed")
	if rc != exitRefused || out != "" || errb != want {
		t.Fatalf("base %s rc=%d out=%q\n got %q\nwant %q", base[:7], rc, out, errb, want)
	}
	if _, ok := integration.FindPRRecord(j.work.Dir, 6); ok {
		t.Error("a refusal left a record")
	}
	if rc, _, errb := j.run("batch", "6"); rc != exitRefused || j.originMain() != before {
		t.Errorf("batch of an unrecorded PR rc=%d err=%q moved=%v, want a refusal and no push", rc, errb, j.originMain() != before)
	}
}
