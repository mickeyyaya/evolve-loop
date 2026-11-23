package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	closureIgnore = ".evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n"
	closureItemA  = `{"id":"item-a","title":"A","weight":0.5}`
	closureKeep   = `{"id":"item-keep","title":"Keep","weight":0.5}`
)

func newInboxClosureJourney(t *testing.T) *trainJourney {
	t.Helper()
	j := newTrainJourney(t)
	j.work.Write(".gitignore", closureIgnore)
	j.work.Write(".evolve/inbox/item-a.json", closureItemA)
	j.work.Write(".evolve/inbox/item-keep.json", closureKeep)
	j.work.Git("add", "-f", ".gitignore", ".evolve/inbox")
	j.work.Git("commit", "-q", "-m", "track the inbox")
	j.work.Git("push", "-q", "origin", "main")
	j.gh.main = mainRun{tip: j.originMain(), status: "completed", conclusion: "success"}
	return j
}

func (j *trainJourney) landBatch(t *testing.T, prs ...string) {
	t.Helper()
	for _, n := range prs {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatalf("record-pr %s: rc=%d %s", n, rc, errb)
		}
	}
	if rc, _, errb := j.run(append([]string{"batch"}, prs...)...); rc != 0 {
		t.Fatalf("batch rc=%d %s", rc, errb)
	}
}

func (j *trainJourney) syncMain() (int, string) {
	var out, errb bytes.Buffer
	rc := runSyncMain([]string{"--project-root", j.work.Dir}, nil, &out, &errb)
	return rc, out.String() + errb.String()
}

func (j *trainJourney) tracked(rel string) bool {
	return j.work.Git("ls-files", "--", rel) != ""
}

func TestJourneyInboxClosureBatchThenSyncMainPasses(t *testing.T) {
	t.Parallel()
	j := newInboxClosureJourney(t)
	j.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", "a.md", "a\n")
	j.landBatch(t, "1")
	inbox := filepath.Join(j.work.Dir, ".evolve", "inbox")
	if !inboxProcessed(t, inbox, "item-a") {
		t.Fatal("the batch must move item-a under processed/")
	}

	rc, output := j.syncMain()

	if rc != 0 {
		t.Fatalf("sync-main rc=%d after a batch that closed an item, want 0:\n%s", rc, output)
	}
	if j.tracked(".evolve/inbox/item-a.json") {
		t.Error("item-a is still tracked at the inbox root after the sync")
	}
	if !j.tracked(".evolve/inbox/item-keep.json") {
		t.Error("item-keep, which no PR closed, must stay tracked")
	}
	if dirty := j.work.Git("status", "--porcelain", "--untracked-files=no"); dirty != "" {
		t.Errorf("tracked tree dirty after sync-main:\n%s", dirty)
	}
	if again, output := j.syncMain(); again != 0 {
		t.Errorf("a second sync-main rc=%d, want 0:\n%s", again, output)
	}
	if _, err := os.Stat(filepath.Join(inbox, "item-keep.json")); err != nil {
		t.Errorf("item-keep must stay at the inbox root: %v", err)
	}
}

func TestJourneyInboxClosureStillRefusesOtherTrackedDeletions(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"an inbox item no PR closed": ".evolve/inbox/item-keep.json",
		"a file outside the inbox":   "shared.md",
	}
	for name, victim := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			j := newInboxClosureJourney(t)
			j.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", "a.md", "a\n")
			j.landBatch(t, "1")
			if err := os.Remove(filepath.Join(j.work.Dir, filepath.FromSlash(victim))); err != nil {
				t.Fatal(err)
			}

			rc, output := j.syncMain()

			if rc != cliExitRefused {
				t.Errorf("sync-main rc=%d, want %d (refused) for a tracked deletion of %s:\n%s", rc, cliExitRefused, victim, output)
			}
			if !strings.Contains(output, victim) {
				t.Errorf("the refusal must name %s:\n%s", victim, output)
			}
		})
	}
}

func TestJourneyInboxClosureWithNoClosedItemLeavesSyncMainUnchanged(t *testing.T) {
	t.Parallel()
	j := newInboxClosureJourney(t)
	j.pr(t, 1, "feat: a", "a.md", "a\n")
	j.landBatch(t, "1")

	rc, output := j.syncMain()

	if rc != 0 {
		t.Fatalf("sync-main rc=%d, want 0:\n%s", rc, output)
	}
	if head, origin := j.work.Git("rev-parse", "HEAD"), j.originMain(); head != origin {
		t.Errorf("a batch that closed nothing must leave local main at origin main: HEAD=%s origin=%s", head, origin)
	}
	if !j.tracked(".evolve/inbox/item-a.json") || !j.tracked(".evolve/inbox/item-keep.json") {
		t.Error("no item was closed, so both stay tracked")
	}
}
