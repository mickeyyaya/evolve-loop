package main

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

func redHeadJourney(t *testing.T) *trainJourney {
	t.Helper()
	j := recordedJourney(t, 1, 2)
	head := strings.TrimSpace(j.origin.Git("rev-parse", "refs/pull/1/head"))
	j.gh.headRuns[head] = "failure"
	return j
}

func TestRunBatch_EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns(t *testing.T) {
	t.Parallel()
	j := redHeadJourney(t)
	rc, out, errb := j.run("batch", "1", "2")
	if rc != exitRefused || !strings.Contains(errb, "PR #1") {
		t.Fatalf("batch rc=%d out=%s err=%s, want a refusal that names PR #1", rc, out, errb)
	}
	if len(j.gh.comments["1"]) != 1 || !strings.Contains(j.gh.comments["1"][0], integration.CauseHeadCI) {
		t.Errorf("PR 1 comments = %v, want one brief that names %q", j.gh.comments["1"], integration.CauseHeadCI)
	}
	if _, closed := j.gh.closed["1"]; closed {
		t.Error("PR 1 was closed, want it left open")
	}
	if !strings.Contains(j.gh.closed["2"], "#99") {
		t.Errorf("PR 2 close comment = %q, want it landed through batch PR #99", j.gh.closed["2"])
	}
	if tree := j.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"); strings.Contains(tree, "b.md") || !strings.Contains(tree, "c.md") {
		t.Errorf("main holds %q, want the file of PR 2 and not the file of the red PR 1", tree)
	}
}

func TestRunBatch_DryRunNamesThePRWhoseHeadCIIsRed(t *testing.T) {
	t.Parallel()
	j := redHeadJourney(t)
	before := j.originMain()
	rc, out, errb := j.run("batch", "--dry-run", "1", "2")
	if rc != 0 || j.originMain() != before || j.gh.creates != 0 {
		t.Fatalf("dry run rc=%d err=%s creates=%d", rc, errb, j.gh.creates)
	}
	if want := "eject PR #1 " + integration.CauseHeadCI; !strings.Contains(out, want) {
		t.Errorf("dry run output lacks %q:\n%s", want, out)
	}
}
