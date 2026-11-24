package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const ratchetJourneyList = ratchetListPath

func (j *trainJourney) recordRatchetPR(t *testing.T, n int) {
	t.Helper()
	train, _ := j.env.train(j.work.Dir, io.Discard)
	if _, err := train.Record(context.Background(), n, ratchetListsBinder); err != nil {
		t.Fatalf("record the ratchet PR #%d: %v", n, err)
	}
}

func (j *trainJourney) seedList(t *testing.T) {
	t.Helper()
	j.work.Write(ratchetJourneyList, "{\n  \"internal/fixture.Big\": 90\n}\n")
	j.work.Git("add", "-A")
	j.work.Git("commit", "-q", "-m", "seed the list")
	j.work.Git("push", "-q", "origin", "main")
	j.gh.main = mainRun{tip: j.originMain(), status: "completed", conclusion: "success"}
}

func TestJourneyRatchetListsPRComposesAfterTheLanePRAndPassesTheTrailerCheck(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.seedList(t)
	j.pr(t, 1, ratchetTightenMessage, ratchetJourneyList, "{\n  \"internal/fixture.Big\": 64\n}\n")
	j.pr(t, 2, "docs: a", "a.md", "a\n")
	j.recordRatchetPR(t, 1)
	if rc, _, errb := j.run("record-pr", "--pr", "2", "--class", "docs-review"); rc != 0 {
		t.Fatalf("record-pr 2: rc=%d %s", rc, errb)
	}

	rc, out, errb := j.run("batch", "--dry-run", "1", "2")

	if rc != 0 || !strings.Contains(out, "order: 2 1") {
		t.Fatalf("dry run rc=%d out=%q err=%q, want the lane PR first and the ratchet PR last", rc, out, errb)
	}
	if rc, _, errb := j.run("batch", "1", "2"); rc != 0 {
		t.Fatalf("batch rc=%d %s", rc, errb)
	}
	if got := j.work.Git("show", "origin/main:"+ratchetJourneyList); !strings.Contains(got, "64") {
		t.Errorf("origin main holds %q, want the tightened list", got)
	}
	j.work.Git("fetch", "-q", "origin")
	msg := j.work.Git("log", "-1", "--format=%B", "--grep=evolve-pr 1", "origin/main")
	if !strings.Contains(msg, "Closes-Inbox: none") {
		t.Fatalf("the composed commit lacks Closes-Inbox: none:\n%s", msg)
	}
	if _, _, violations := inboxcmd.CheckInboxTrailerBytes(j.work.Dir, []byte(msg), "PR #1"); len(violations) != 0 {
		t.Errorf("trailer check: %v", violations)
	}
	if _, ok := reviewClasses[integration.RatchetListsClass]; ok {
		t.Error("the class must stay out of reviewClasses")
	}
}

func TestJourneyRatchetListsPRConflictEjectsTheRatchetPRAndKeepsTheLanePR(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.seedList(t)
	j.pr(t, 1, ratchetTightenMessage, ratchetJourneyList, "{\n  \"internal/fixture.Big\": 64\n}\n")
	j.pr(t, 2, "docs: edits the same list line", ratchetJourneyList, "{\n  \"internal/fixture.Big\": 80\n}\n")
	j.recordRatchetPR(t, 1)
	train, _ := j.env.train(j.work.Dir, io.Discard)
	anyPaths := func(ev integration.Evidence) (integration.AuditBinding, error) {
		return integration.ClassBinding(ev, "docs-review", func([]string) bool { return true })
	}
	if _, err := train.Record(context.Background(), 2, anyPaths); err != nil {
		t.Fatalf("record PR 2: %v", err)
	}

	rc, _, errb := j.run("batch", "1", "2")

	if rc != exitRefused || !strings.Contains(errb, "PR #1") {
		t.Fatalf("batch rc=%d err=%q, want the ratchet PR #1 refused", rc, errb)
	}
	if got := j.work.Git("show", "origin/main:"+ratchetJourneyList); !strings.Contains(got, "80") {
		t.Errorf("origin main holds %q, want the lane PR's list", got)
	}
}

func TestRatchetRecordPR_RecordsThroughTheIntegrationTrainOfTheProject(t *testing.T) {
	t.Parallel()
	j := newTrainJourney(t)
	j.seedList(t)
	j.pr(t, 1, ratchetTightenMessage, ratchetJourneyList, "{\n  \"internal/fixture.Big\": 64\n}\n")

	path, err := recordRatchetPR(ratchetRecordRequest{root: j.work.Dir, pr: 1, bind: ratchetListsBinder, env: j.env, warn: io.Discard})

	rec, ok := integration.FindPRRecord(j.work.Dir, 1)
	if err != nil || path == "" || !ok || rec.Audit.Class != integration.RatchetListsClass {
		t.Fatalf("path=%q err=%v record=%+v ok=%v", path, err, rec.Audit, ok)
	}
}
