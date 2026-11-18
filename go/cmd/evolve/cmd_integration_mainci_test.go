package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

func recordedJourney(t *testing.T, prs ...int) *trainJourney {
	t.Helper()
	j := newTrainJourney(t)
	for _, n := range prs {
		name := string(rune('a' + n))
		j.pr(t, n, "feat: "+name, name+".md", name+"\n")
	}
	for _, n := range prs {
		if rc, _, errb := j.run("record-pr", "--pr", strconv.Itoa(n), "--class", "docs-review"); rc != 0 {
			t.Fatalf("record-pr %d: rc=%d %s", n, rc, errb)
		}
	}
	return j
}

func TestRunBatch_RefusesARedMain(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		set    func(m *mainRun)
		reason string
	}{
		{"red", func(m *mainRun) { m.conclusion = "failure" }, "(red)"},
		{"cancelled", func(m *mainRun) { m.conclusion = "cancelled" }, "(cancelled)"},
		{"no run", func(m *mainRun) { m.noRun = true }, "(no CI run)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			j := recordedJourney(t, 1, 2)
			c.set(&j.gh.main)
			before := j.originMain()
			for _, args := range [][]string{{"batch", "1", "2"}, {"batch", "--dry-run", "1", "2"}} {
				rc, out, errb := j.run(args...)
				if rc != exitRefused || !strings.Contains(errb, integration.ShortSHA(before)) || !strings.Contains(errb, c.reason) || !strings.Contains(errb, "--main-red-fix") {
					t.Errorf("%v: rc=%d err=%q, want a refusal that names main %s, %s and the fix flag", args, rc, errb, integration.ShortSHA(before), c.reason)
				}
				if strings.Contains(out, "order:") {
					t.Errorf("%v: a refused batch printed a plan:\n%s", args, out)
				}
			}
			if j.gh.creates != 0 || j.originMain() != before {
				t.Errorf("a red main opened %d batch PRs and moved main=%v", j.gh.creates, j.originMain() != before)
			}
		})
	}
}

func TestRunBatch_AGhFailureOnTheMainCheckIsAnIOErrorNotARefusal(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1)
	j.gh.main.listRC = 4
	rc, _, errb := j.run("batch", "1")
	if rc != exitIO || strings.Contains(errb, "--main-red-fix") || j.gh.creates != 0 {
		t.Errorf("rc=%d err=%q creates=%d, want exit %d with no main refusal and no batch", rc, errb, j.gh.creates, exitIO)
	}
}

func TestRunBatch_ChecksAGreenMainAndPrintsEachStep(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1, 2)
	main := j.originMain()
	rc, out, errb := j.run("batch", "1", "2")
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errb)
	}
	want := []string{
		"main: " + integration.ShortSHA(main) + " CI green",
		"step: test PRs 1 2 in batch PR #99",
		"step: batch PR #99 of PRs 1 2 is green",
		"step: landed PRs 1 2 through batch PR #99",
		"landed: 1 2",
	}
	at := 0
	for _, w := range want {
		i := strings.Index(out[at:], w)
		if i < 0 {
			t.Fatalf("output lacks %q after offset %d:\n%s", w, at, out)
		}
		at += i + len(w)
	}
	if j.gh.main.watched != 1 {
		t.Errorf("the real run watched the main run %d times, want 1", j.gh.main.watched)
	}
}

func TestRunBatch_PrintsTheRerunAndTheSplitOfARedBatch(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1, 2)
	j.gh.conclusions = []string{"failure", "failure"}
	rc, out, errb := j.run("batch", "1", "2")
	if rc != 0 {
		t.Fatalf("rc=%d err=%s out=%s", rc, errb, out)
	}
	for _, w := range []string{"step: batch PR #99 of PRs 1 2 is red", "step: rerun the failed jobs of batch PR #99", "step: split PRs 1 2 in halves", "step: test PRs 1 in batch PR #99", "step: landed PRs 2 through batch PR #99"} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func TestRunBatch_TheDryRunReadsAPendingMainOnceAndGoesOn(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1)
	j.gh.main.status, j.gh.main.conclusion = "in_progress", ""
	rc, out, errb := j.run("batch", "--dry-run", "1")
	if rc != 0 || !strings.Contains(out, "CI pending") || !strings.Contains(out, "order: 1") {
		t.Errorf("rc=%d out=%q err=%q, want a pending main and the plan", rc, out, errb)
	}
	if j.gh.main.watched != 0 {
		t.Errorf("the dry run watched the main run %d times, want a single read", j.gh.main.watched)
	}
}

func TestRunBatch_MainRedFixLandsOneNamedPROnARedMain(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1)
	j.gh.main.conclusion = "failure"
	before := j.originMain()
	rc, out, errb := j.run("batch", "--main-red-fix", "1")
	if rc != 0 || !strings.Contains(out, "CI red, --main-red-fix") || !strings.Contains(out, "landed: 1") || j.originMain() == before {
		t.Errorf("rc=%d out=%q err=%q, want the fix to land on the red main", rc, out, errb)
	}
}

func TestRunBatch_MainRedFixNeedsExactlyOneNamedPR(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1, 2)
	for _, args := range [][]string{{"batch", "--main-red-fix"}, {"batch", "--main-red-fix", "1", "2"}} {
		rc, _, errb := j.run(args...)
		if rc != exitUsage || !strings.Contains(errb, "exactly one named PR") || j.gh.creates != 0 {
			t.Errorf("%v: rc=%d err=%q, want a usage error", args, rc, errb)
		}
	}
}

func TestRunBatch_ARedMainNeverTouchesTheMainRedStop(t *testing.T) {
	t.Parallel()
	s := newStopJourney(t, "enforce")
	s.pr(t, 1, "feat: a", "a.md", "a\n")
	if rc, _, errb := s.run("record-pr", "--pr", "1", "--class", "docs-review"); rc != 0 {
		t.Fatalf("record-pr: %s", errb)
	}
	s.holdStop()
	s.gh.main.conclusion = "failure"
	rc, _, errb := s.run("batch", "--project-root", s.root, "1")
	if rc != exitRefused || !strings.Contains(errb, "--main-red-fix") {
		t.Fatalf("rc=%d err=%q, want the main refusal", rc, errb)
	}
	if strings.Contains(errb, "batch pull request") || strings.Contains(errb, selfheal.ErrMainRedHold.Error()) {
		t.Errorf("the main check went through the stop gate: %q", errb)
	}
}

func TestStepLine_NamesAnUnknownKindAndNeverCallsItALanding(t *testing.T) {
	t.Parallel()
	got := stepLine(integration.Step{Kind: "probe", PRs: []int{4}, Batch: 9}, nil)
	if got != "step: probe PRs 4 in batch PR #9" {
		t.Errorf("stepLine = %q", got)
	}
}
