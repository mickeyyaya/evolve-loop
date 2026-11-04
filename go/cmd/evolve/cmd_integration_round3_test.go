package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type prStep struct {
	message string
	file    string
	body    string
}

func (j *trainJourney) prSteps(t *testing.T, n int, steps ...prStep) {
	t.Helper()
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", "-b", name, "main")
	for _, s := range steps {
		j.work.Write(s.file, s.body)
		j.work.Git("add", ".")
		j.work.Git("commit", "-q", "-m", s.message)
	}
	j.work.Git("push", "-q", "origin", name+":refs/pull/"+strconv.Itoa(n)+"/head")
	j.work.Git("checkout", "-q", "main")
}

func (j *trainJourney) advancePR(n int, file, body string) string {
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", name)
	j.work.Write(file, body)
	j.work.Git("add", file)
	j.work.Git("commit", "-q", "-m", "fix: late")
	j.work.Git("push", "-q", "origin", name+":refs/pull/"+strconv.Itoa(n)+"/head")
	head := strings.TrimSpace(j.work.Git("rev-parse", "HEAD"))
	j.work.Git("checkout", "-q", "main")
	return head
}

func (j *trainJourney) recordDocs(t *testing.T, prs ...int) {
	t.Helper()
	for _, n := range prs {
		if rc, _, errb := j.run("record-pr", "--pr", strconv.Itoa(n), "--class", "docs-review"); rc != 0 {
			t.Fatalf("record-pr %d: rc=%d %s", n, rc, errb)
		}
	}
}

func (j *trainJourney) afterGit(match func(args []string) bool, hook func()) {
	real := j.env.Git
	j.env.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		out, err := real(ctx, dir, stdin, args...)
		if err == nil && hook != nil && match(args) {
			run := hook
			hook = nil
			run()
		}
		return out, err
	}
}

func (j *trainJourney) failGit(match func(args []string) bool) {
	real := j.env.Git
	j.env.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if match(args) {
			return "", &integration.GitError{Args: args, Code: 128}
		}
		return real(ctx, dir, stdin, args...)
	}
}

func pushesMain(args []string) bool {
	return len(args) >= 3 && args[0] == "push" && strings.HasSuffix(args[len(args)-1], ":refs/heads/main")
}

func readsTheLog(args []string) bool {
	return len(args) > 0 && args[0] == "log" && slices.Contains(args, "-z")
}

func TestJourneyBatchKeepsAPRWhoseHeadMovesDuringTheCIAndNamesTheCommits(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	rec, _ := integration.FindPRRecord(j.work.Dir, 1)
	before := j.originMain()
	var late string
	j.gh.onCreate = func() { late = j.advancePR(1, "late.md", "late\n") }

	rc, _, errb := j.run("batch", "1")

	if rc != exitRefused || !strings.Contains(errb, "PR #1") {
		t.Errorf("rc=%d err=%q, want a refusal that names PR #1", rc, errb)
	}
	if j.originMain() != before {
		t.Error("a PR whose head moved during the CI must not reach main")
	}
	if _, closed := j.gh.closed["1"]; closed {
		t.Error("a PR whose head moved must stay open")
	}
	brief := strings.Join(j.gh.comments["1"], "\n")
	if !strings.Contains(brief, rec.Commits[0].SHA) || !strings.Contains(brief, late) {
		t.Errorf("brief = %q, want the recorded commit %s and the new commit %s", brief, rec.Commits[0].SHA, late)
	}
}

func TestJourneyBatchKeepsAPRWhoseHeadMovesAfterTheFastForward(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	before := j.originMain()
	var late string
	j.afterGit(pushesMain, func() { late = j.advancePR(1, "late.md", "late\n") })

	rc, out, errb := j.run("batch", "1")

	if j.originMain() == before {
		t.Fatal("the recorded commits must land")
	}
	if rc != exitRefused || !strings.Contains(errb, "PR #1") {
		t.Errorf("rc=%d out=%q err=%q, want a refusal that names PR #1", rc, out, errb)
	}
	if _, closed := j.gh.closed["1"]; closed {
		t.Error("a PR whose head moved after the landing must stay open")
	}
	if brief := strings.Join(j.gh.comments["1"], "\n"); !strings.Contains(brief, late) {
		t.Errorf("brief = %q, want the commit %s that did not land", brief, late)
	}
}

func (j *trainJourney) multiCommitPRs(t *testing.T) {
	t.Helper()
	j.work.Write("shared.md", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	j.prSteps(t, 2,
		prStep{"feat: one\n\nCloses-Inbox: item-1", "shared.md", "one\nTWO\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"},
		prStep{"feat: two", "shared.md", "one\nTwo!\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"},
		prStep{"feat: three\n\nCloses-Inbox: item-2", "c.md", "c\n"})
	j.prSteps(t, 3, prStep{"feat: peer", "shared.md", "one\ntwo\nthree\nFOUR\nfive\nsix\nseven\neight\nnine\nten\n"})
}

func (j *trainJourney) seedInbox(t *testing.T, ids ...string) string {
	t.Helper()
	inbox := filepath.Join(j.work.Dir, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return inbox
}

func TestJourneyMultiCommitPRBindsEveryCommitOfThePR(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.prSteps(t, 1,
		prStep{"feat: code first", "main.go", "package main\n"},
		prStep{"docs: text", "n.md", "n\n"})
	if rc, _, _ := j.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != exitRefused {
		t.Errorf("a PR whose first commit holds code took the docs class: rc=%d, want %d", rc, exitRefused)
	}
	if _, ok := integration.FindPRRecord(j.work.Dir, 1); ok {
		t.Error("a refused record left a file")
	}
}

func TestJourneyMultiCommitPRsLandInOrderCloseTheirInboxAndNeverLandTwice(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.multiCommitPRs(t)
	inbox := j.seedInbox(t, "item-1", "item-2")
	j.recordDocs(t, 2, 3)
	rec, _ := integration.FindPRRecord(j.work.Dir, 2)
	if len(rec.Commits) != 3 || !strings.Contains(j.origin.Git("log", "-1", "--format=%B", "refs/pull/2/head"), "item-2") {
		t.Fatalf("record = %+v, want three commits", rec.Commits)
	}
	before := j.originMain()
	j.gh.failClose = true

	if rc, _, _ := j.run("batch", "2", "3"); rc == 0 {
		t.Fatal("a failed close must fail the batch")
	}

	landed := j.originMain()
	if got := strings.TrimSpace(j.origin.Git("rev-list", "--count", before+".."+landed)); got != "4" {
		t.Fatalf("main moved by %s commits, want 4 (one for each commit of PR 2 and one for PR 3)", got)
	}
	log := j.origin.Git("log", "--reverse", "--format=%s%n%B", before+".."+landed)
	if strings.Count(log, "Source-PR: #2") != 3 || !strings.Contains(log, "Closes-Inbox: item-1") || !strings.Contains(log, "Closes-Inbox: item-2") {
		t.Errorf("composed commits lost the Source-PR line or an inbox id:\n%s", log)
	}
	if tree := j.origin.Git("show", landed+":shared.md"); !strings.Contains(tree, "Two!") || !strings.Contains(tree, "FOUR") {
		t.Errorf("shared.md on main = %q, want the edits of both PRs", tree)
	}
	for _, id := range []string{"item-1", "item-2"} {
		if !inboxProcessed(t, inbox, id) {
			t.Errorf("%s is not under processed/", id)
		}
	}

	j.gh.failClose = false
	j.env.Wave = "j2"
	rc, out, errb := j.run("batch", "2", "3")
	if rc != 0 || j.originMain() != landed || j.gh.creates != 1 {
		t.Fatalf("rerun rc=%d out=%s err=%s main moved=%v creates=%d, want a close with no second landing", rc, out, errb, j.originMain() != landed, j.gh.creates)
	}
	if _, ok := j.gh.closed["2"]; !ok {
		t.Error("PR 2 must close on the rerun")
	}
	if _, ok := j.gh.closed["3"]; !ok {
		t.Error("PR 3 must close on the rerun")
	}
}

func TestJourneyBatchStopsWhenTheLandedCheckFails(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	j.gh.failClose = true
	if rc, _, _ := j.run("batch", "1"); rc == 0 {
		t.Fatal("a failed close must fail the batch")
	}
	landed := j.originMain()
	j.gh.failClose = false
	j.env.Wave = "j2"
	j.failGit(readsTheLog)

	rc, _, errb := j.run("batch", "1")

	if rc != exitIO || errb == "" {
		t.Errorf("rc=%d err=%q, want exit %d and the cause", rc, errb, exitIO)
	}
	if j.originMain() != landed || j.gh.creates != 1 {
		t.Errorf("a failed landed check must not land the PR again: creates=%d", j.gh.creates)
	}
}

func TestJourneyBatchPrintsTheCommandThatPutsTheInboxMoveOnMain(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", "a.md", "a\n")
	j.seedInbox(t, "item-a")
	j.recordDocs(t, 1)

	rc, out, errb := j.run("batch", "1")

	want := "evolve ship --class manual --paths .evolve/inbox/item-a.json"
	if rc != 0 || !strings.Contains(out, want) {
		t.Errorf("rc=%d err=%q out=%q, want the command %q", rc, errb, out, want)
	}
}

func TestJourneyBatchExitsNonzeroAndNamesAPRThatRedCIEjects(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	j.gh.conclusion = "failure"

	rc, _, errb := j.run("batch", "1")

	if rc != exitRefused || !strings.Contains(errb, "PR #1") {
		t.Errorf("rc=%d err=%q, want exit %d and the name of PR #1", rc, errb, exitRefused)
	}
}

func TestJourneyBatchReportsAFlakedRun(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	j.gh.conclusions = []string{"failure", "success"}

	rc, out, errb := j.run("batch", "1")

	if rc != 0 || !strings.Contains(out, "flake") {
		t.Errorf("rc=%d out=%q err=%q, want a landing and a note about the flake", rc, out, errb)
	}
}

func TestJourneyBatchFetchesAMainThatMovedInAnotherClone(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.recordDocs(t, 1)
	j.gh.onCreate = func() {
		other := gittest.Clone(t, j.origin.Dir)
		other.Write("elsewhere.md", "e\n")
		other.Git("add", ".")
		other.Git("commit", "-q", "-m", "other: moves main")
		other.Git("push", "-q", "origin", "main")
	}

	rc, _, errb := j.run("batch", "1")

	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errb)
	}
	if got := j.origin.Git("show", j.originMain()+":a.md"); !strings.Contains(got, "a") {
		t.Errorf("PR 1 must land on the moved main: %q", got)
	}
	if !strings.Contains(j.origin.Git("ls-tree", "-r", "--name-only", j.originMain()), "elsewhere.md") {
		t.Error("the work of the other clone is lost")
	}
}
