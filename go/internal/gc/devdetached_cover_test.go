package gc

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPlanDevWorktrees_ATreeIsNotJudgedWhenMainCannotBeRead(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDetached("no-main")
	e.store.Git("update-ref", "-d", "refs/remotes/origin/main")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if !it.Failed || !strings.Contains(it.Reason, "cannot judge") {
		t.Fatalf("item = %+v, want a tree that is not judged", it)
	}
}

func TestPlanDevWorktrees_ALookupFailureLeavesANoteForTheOperator(t *testing.T) {
	e := newDevEnv(t)
	e.addDetachedWithUniqueWork("note")
	o := e.options(idleOffset)
	o.PRHeads = func(context.Context, string) (map[string]bool, error) { return nil, ErrPRLookupUnavailable }
	m := e.plan(o)
	if len(m.Notes) != 1 || !strings.Contains(m.Notes[0], "counts as unique") {
		t.Fatalf("notes = %q, want one note that the heads are unavailable", m.Notes)
	}
}

func TestApplyDevWorktrees_ACheckpointThatCannotBeWrittenKeepsTheTree(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("pin-fails")
	o := e.options(idleOffset)
	m := e.plan(o)
	e.store.Git("update-ref", "refs/checkpoints/pin-fails/"+o.Now().UTC().Format("20060102T150405Z"), "main")
	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 || !strings.Contains(err.Error(), "could not be pinned") {
		t.Fatalf("apply = %+v, %v; want a refusal because the head could not be pinned", recs, err)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Fatalf("the tree was removed: %v", serr)
	}
}
