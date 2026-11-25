package gc

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func x12Policy(t *testing.T, globs ...string) DevWorktreesPolicy {
	t.Helper()
	raw, err := json.Marshal(map[string][]string{"protected": globs})
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p DevWorktreesPolicy
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("gc.dev_worktrees.protected is not a field of DevWorktreesPolicy: %v", err)
	}
	return p
}

func x12Landed(e *devEnv, name string) string {
	e.t.Helper()
	dir := filepath.Join(e.root, "dev", name)
	e.store.Git("worktree", "add", "-q", "-b", "br-"+name, dir, plane.OriginMainRef)
	e.commitIn(dir, name+".txt", name+"\n")
	e.store.Git("update-ref", plane.OriginMainRef, e.store.Git("-C", dir, "rev-parse", "HEAD"))
	return dir
}

func TestPlanDevWorktrees_X12_AProtectedGlobKeepsALandedTreeAndNamesTheGlob(t *testing.T) {
	e := newDevEnv(t)
	index := x12Landed(e, "code-index")
	lane := x12Landed(e, "lane-7")
	plain := x12Landed(e, "plain")
	partial := x12Landed(e, "my-code-index-copy")
	o := e.options(0)
	o.Policy = x12Policy(t, "code-index", "lane-*")

	m := e.plan(o)

	for dir, glob := range map[string]string{index: "code-index", lane: "lane-*"} {
		it := devItemFor(t, m, dir)
		if it.Action != DevActionKeep || !strings.Contains(it.Reason, "protected by policy: "+glob) {
			t.Errorf("%s = %+v, want kept with the reason %q", dir, it, "protected by policy: "+glob)
		}
	}
	for _, dir := range []string{plain, partial} {
		if it := devItemFor(t, m, dir); it.Action != DevActionArchive {
			t.Errorf("%s = %+v, want archived: a glob matches the whole name under the dev dir, not a part of it", dir, it)
		}
	}
}

func TestPlanDevWorktrees_X12_AProtectedTreeStaysEvenWhenItIsIdle(t *testing.T) {
	e := newDevEnv(t)
	index := e.addDev("code-index")
	other := e.addDev("other")
	o := e.options(idleOffset)
	o.Policy = x12Policy(t, "code-index")

	m := e.plan(o)

	if it := devItemFor(t, m, index); it.Action != DevActionKeep || !strings.Contains(it.Reason, "protected by policy: code-index") {
		t.Errorf("idle protected tree = %+v, want kept by policy", it)
	}
	if it := devItemFor(t, m, other); it.Action != DevActionArchive {
		t.Errorf("idle unprotected tree = %+v, want archived", it)
	}
}

func TestPlanDevWorktrees_X12_WithNoPolicyNoNameIsProtected(t *testing.T) {
	e := newDevEnv(t)
	index := x12Landed(e, "code-index")

	it := devItemFor(t, e.plan(e.options(0)), index)

	if it.Action != DevActionArchive || strings.Contains(it.Reason, "protected") {
		t.Errorf("code-index with an empty policy = %+v, want archived: no code list may protect a name", it)
	}
}

func TestPlanDevWorktrees_X12_ADirtyLandedTreeIsKeptAndItsChangedPathsAreListed(t *testing.T) {
	e := newDevEnv(t)
	dirty := x12Landed(e, "dirty")
	clean := x12Landed(e, "clean")
	writeRepoFile(t, dirty, "base.txt", "changed\n")
	writeRepoFile(t, dirty, "scratch.txt", "x\n")
	writeRepoFile(t, dirty, "staged.txt", "y\n")
	e.store.Git("-C", dirty, "add", "staged.txt")

	m := e.plan(e.options(0))

	it := devItemFor(t, m, dirty)
	if it.Action != DevActionKeep {
		t.Fatalf("dirty tree = %+v, want kept", it)
	}
	for _, rel := range []string{"base.txt", "scratch.txt", "staged.txt"} {
		if !strings.Contains(it.Reason, rel) {
			t.Errorf("the keep reason %q does not list the changed path %s", it.Reason, rel)
		}
	}
	if got := devItemFor(t, m, clean); got.Action != DevActionArchive {
		t.Errorf("clean landed tree = %+v, want archived", got)
	}
}

func TestApplyDevWorktrees_X12_AStalePlanCannotArchiveATreeThatPolicyNowProtects(t *testing.T) {
	e := newDevEnv(t)
	dir := x12Landed(e, "code-index")
	planned := e.plan(e.options(0))
	if it := devItemFor(t, planned, dir); it.Action != DevActionArchive {
		t.Fatalf("plan without policy = %+v, want archive", it)
	}
	o := e.options(0)
	o.Policy = x12Policy(t, "code-index")

	recs, err := ApplyDevWorktrees(context.Background(), o, planned)

	if err == nil || len(recs) != 0 {
		t.Errorf("ApplyDevWorktrees = %v, %v, want a refusal and no record", recs, err)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Errorf("the protected tree was removed by a stale plan: %v", statErr)
	}
}
