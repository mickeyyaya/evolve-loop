package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

func TestJourneyRecordPRTakesOnlyAClassWithItsRule(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: code", "main.go", "package main\n")
	j.pr(t, 2, "docs: text", "notes.md", "notes\n")
	for _, class := range []string{"no-review-happened", "docs-review"} {
		if rc, _, _ := j.run("record-pr", "--pr", "1", "--class", class); rc != exitRefused {
			t.Errorf("class %q on a code PR: rc=%d, want %d", class, rc, exitRefused)
		}
	}
	if rc, _, _ := j.run("record-pr", "--pr", "2", "--class", "no-review-happened"); rc != exitRefused {
		t.Errorf("an invented class: rc=%d, want %d", rc, exitRefused)
	}
	if rc, _, errb := j.run("record-pr", "--pr", "2", "--class", "docs-review"); rc != 0 {
		t.Errorf("docs-review on a Markdown PR: rc=%d %s", rc, errb)
	}
}

func TestJourneyBatchRefusesAPRWhoseHeadMovedAfterTheRecord(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatal(errb)
	}
	j.work.Git("checkout", "-q", "pr-1")
	j.work.Write("late.md", "late fix\n")
	j.work.Git("add", "late.md")
	j.work.Git("commit", "-q", "-m", "fix: late")
	j.work.Git("push", "-q", "origin", "pr-1:refs/pull/1/head")
	j.work.Git("checkout", "-q", "main")
	before := j.originMain()
	for _, args := range [][]string{{"batch", "--dry-run", "1"}, {"batch", "1"}} {
		rc, _, errb := j.run(args...)
		if rc != exitRefused || !strings.Contains(errb, "PR #1") || !strings.Contains(errb, "record the review again") {
			t.Errorf("%v: rc=%d err=%q, want a refusal that says to record the review again", args, rc, errb)
		}
	}
	if j.originMain() != before || len(j.gh.closed) != 0 || j.gh.creates != 0 {
		t.Error("a PR with a moved head must not land or close")
	}
}

func TestJourneyBatchClosesTheInboxItemsOfTheLandedPRs(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", "a.md", "a\n")
	j.pr(t, 2, "feat: b\n\n- closes-inbox: item-b, item-c", "b.md", "b\n")
	inbox := filepath.Join(j.work.Dir, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"item-a", "item-b", "item-c", "item-keep"} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"1", "2"} {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatal(errb)
		}
	}
	if rc, _, errb := j.run("batch", "1", "2"); rc != 0 {
		t.Fatalf("batch rc=%d %s", rc, errb)
	}
	for _, id := range []string{"item-a", "item-b", "item-c"} {
		if _, err := os.Stat(filepath.Join(inbox, id+".json")); !os.IsNotExist(err) {
			t.Errorf("%s must leave the inbox root once its PR landed: %v", id, err)
		}
		if !inboxProcessed(t, inbox, id) {
			t.Errorf("%s is not under processed/", id)
		}
	}
	if _, err := os.Stat(filepath.Join(inbox, "item-keep.json")); err != nil {
		t.Errorf("an item no PR names must stay: %v", err)
	}
}

func inboxProcessed(t *testing.T, inbox, id string) bool {
	t.Helper()
	found := false
	_ = filepath.Walk(filepath.Join(inbox, "processed"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, id+".json") {
			found = true
		}
		return nil
	})
	return found
}

func TestJourneyBatchBriefsAPRThatConflictsOnceMainMovedDuringTheLand(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.pr(t, 2, "feat: x", "shared.md", "X\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	for _, n := range []string{"1", "2"} {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatal(errb)
		}
	}
	j.gh.onCreate = func() {
		j.work.Git("checkout", "-q", "-b", "mover", "main")
		j.work.Write("shared.md", "Z\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
		j.work.Git("add", "shared.md")
		j.work.Git("commit", "-q", "-m", "other: moves main")
		j.work.Git("push", "-q", "origin", "mover:main")
		j.work.Git("checkout", "-q", "main")
	}
	rc, out, errb := j.run("batch", "1", "2")
	if rc != exitRefused || !strings.Contains(errb, "PR #2") {
		t.Fatalf("a clean PR must land and the ejected PR must be named: rc=%d out=%s err=%s", rc, out, errb)
	}
	if _, ok := j.gh.closed["1"]; !ok {
		t.Error("PR 1 is clean and must close")
	}
	if got := strings.Join(j.gh.comments["2"], "\n"); !strings.Contains(got, "shared.md") {
		t.Errorf("PR 2 brief = %q, want the conflict path", got)
	}
	if _, ok := j.gh.closed["2"]; ok {
		t.Error("the ejected PR must stay open")
	}
}

func TestJourneyBatchDoesNotLandAPRTwiceAfterAFailedClose(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatal(errb)
	}
	before := j.originMain()
	j.gh.failClose = true
	if rc, _, _ := j.run("batch", "1"); rc == 0 {
		t.Fatal("a failed close must fail the batch")
	}
	landed := j.originMain()
	if landed == before {
		t.Fatal("the first run must move main")
	}
	j.gh.failClose = false
	j.env.Wave = "j2"
	rc, _, errb := j.run("batch", "1")
	if rc != 0 {
		t.Fatalf("rerun rc=%d %s", rc, errb)
	}
	if j.originMain() != landed {
		t.Error("the rerun landed the PR a second time")
	}
	if _, ok := j.gh.closed["1"]; !ok || j.gh.creates != 1 {
		t.Errorf("the rerun must close PR 1 without a new batch PR: closed=%v creates=%d", j.gh.closed, j.gh.creates)
	}
	if _, ok := integration.FindPRRecord(j.work.Dir, 1); ok {
		t.Error("the record of a landed PR must be spent")
	}
	if rc, _, _ := j.run("batch", "1"); rc != exitRefused {
		t.Errorf("a spent record must refuse: rc=%d, want %d", rc, exitRefused)
	}
}

func TestJourneyBatchTreatsAGhFailureAsAnErrorAndNotAsRedCI(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatal(errb)
	}
	before := j.originMain()
	j.gh.watchRC, j.gh.viewRC = 1, 1
	rc, _, _ := j.run("batch", "1")
	if rc != exitIO || len(j.gh.comments["1"]) != 0 || j.originMain() != before {
		t.Fatalf("rc=%d comments=%v: a failed gh must stop the batch and brief no PR", rc, j.gh.comments)
	}
	j.gh.viewRC, j.gh.conclusion = 0, "failure"
	j.env.Wave = "j2"
	if rc, _, _ := j.run("batch", "1"); rc != exitRefused || len(j.gh.comments["1"]) != 1 || !strings.Contains(j.gh.comments["1"][0], "ci") {
		t.Errorf("a failed run is red and briefs the PR: rc=%d comments=%v", rc, j.gh.comments)
	}
}

func TestJourneyBatchWaitsForTheRunToRegister(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatal(errb)
	}
	j.gh.noRunYet = 2
	settles := 0
	j.env.Settle = func(context.Context) error { settles++; return nil }
	var out, errb bytes.Buffer
	if rc := runIntegrationEnv(j.env, []string{"batch", "1"}, &out, &errb); rc != 0 || settles != 2 {
		t.Fatalf("rc=%d settles=%d err=%s, want a landing after two waits", rc, settles, errb.String())
	}
}
