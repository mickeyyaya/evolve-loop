package gc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func (e *devEnv) apply(o DevOptions, m DevManifest) ([]DevArchiveRecord, error) {
	return ApplyDevWorktrees(context.Background(), o, m)
}

func (e *devEnv) branchExists(branch string) bool {
	return e.store.Git("branch", "--list", branch) != ""
}

func TestApplyDevWorktrees_AnIdleCleanTreeIsArchivedAndItsBranchKept(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("clean")
	e.commitIn(dir, "work.txt", "committed\n")
	o := e.options(idleOffset)
	m := e.plan(o)

	recs, err := e.apply(o, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("tree %s still exists (stat err %v)", dir, err)
	}
	if !e.branchExists("br-clean") {
		t.Fatal("the branch was deleted; archive must keep it")
	}
	if len(recs) != 1 || recs[0].Checkpoint != "" || recs[0].Branch != "br-clean" || recs[0].Head != e.store.Git("rev-parse", "br-clean") {
		t.Fatalf("records = %+v, want one record with the branch and head and no checkpoint", recs)
	}
	stored, err := ReadDevArchive(e.evolve)
	if err != nil || len(stored) != 1 || stored[0].Path != recs[0].Path || stored[0].IdleDays < 19.9 {
		t.Fatalf("manifest = %+v (err %v), want the one record that apply returned", stored, err)
	}
	if refs := e.store.Git("for-each-ref", "refs/checkpoints/"); refs != "" {
		t.Errorf("a clean tree left checkpoint refs: %q", refs)
	}
}

func TestApplyDevWorktrees_AnIdleDirtyTreeIsCheckpointedThenArchivedAndTheRestoreBringsTheWorkBack(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("dirty")
	e.commitIn(dir, "committed.txt", "committed\n")
	writeRepoFile(t, dir, "base.txt", "edited tracked\n")
	writeRepoFile(t, dir, "deep/untracked.txt", "untracked work\n")
	o := e.options(idleOffset)
	m := e.plan(o)

	recs, err := e.apply(o, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("tree %s still exists (stat err %v)", dir, err)
	}
	if len(recs) != 1 || !strings.HasPrefix(recs[0].Checkpoint, "refs/checkpoints/dirty/") {
		t.Fatalf("records = %+v, want one record with a checkpoint ref under refs/checkpoints/dirty/", recs)
	}

	cmds := recs[0].RestoreCommands()
	if len(cmds) != 2 {
		t.Fatalf("restore commands = %q, want two", cmds)
	}
	e.store.Git("worktree", "add", "-q", dir, "br-dirty")
	if _, err := wtcheckpoint.Restore(context.Background(), e.hub(), recs[0].Checkpoint, dir); err != nil {
		t.Fatalf("restore: %v", err)
	}
	for rel, want := range map[string]string{"base.txt": "edited tracked\n", "deep/untracked.txt": "untracked work\n", "committed.txt": "committed\n"} {
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil || string(got) != want {
			t.Errorf("after restore %s = %q (err %v), want %q", rel, got, err, want)
		}
	}
}

func TestApplyDevWorktrees_ACheckpointSaveFailureRefusesTheArchive(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("unsaved")
	writeRepoFile(t, dir, "precious.txt", "do not lose\n")
	o := e.options(idleOffset)
	m := e.plan(o)
	o.Save = failingDevSave

	recs, err := e.apply(o, m)
	if err == nil || !strings.Contains(err.Error(), "checkpoint save failed") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want the save failure named", err)
	}
	if len(recs) != 0 {
		t.Errorf("records = %+v, want none", recs)
	}
	if got, rerr := os.ReadFile(filepath.Join(dir, "precious.txt")); rerr != nil || string(got) != "do not lose\n" {
		t.Errorf("the work file is %q (err %v), want it untouched in place", got, rerr)
	}
	if stored, _ := ReadDevArchive(e.evolve); len(stored) != 0 {
		t.Errorf("manifest = %+v, want no record for a refused archive", stored)
	}
}

func TestApplyDevWorktrees_TheDefaultSaveLabelsTheCheckpoint(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("labelled")
	writeRepoFile(t, dir, "x.txt", "x\n")
	o := e.options(idleOffset)
	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 {
		t.Fatalf("apply = %+v, %v", recs, err)
	}
	entries, err := wtcheckpoint.List(context.Background(), e.hub(), "labelled")
	if err != nil || len(entries) != 1 || entries[0].Label != devArchiveLabel {
		t.Fatalf("checkpoints = %+v (err %v), want one labelled %q", entries, err, devArchiveLabel)
	}
}

func TestApplyDevWorktrees_ATreeThatBecameActiveSinceThePlanIsRefused(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("woke")
	m := e.plan(e.options(idleOffset))

	recs, err := e.apply(e.options(0), m)
	if err == nil || !strings.Contains(err.Error(), "active within the 14-day limit since the plan") {
		t.Fatalf("err = %v, want a refusal that the tree is active", err)
	}
	if _, serr := os.Stat(dir); serr != nil || len(recs) != 0 {
		t.Errorf("tree stat err %v records %+v, want the tree kept and no record", serr, recs)
	}
}

func TestApplyDevWorktrees_ATreeWhoseHeadMovedSinceThePlanIsRefused(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("moved")
	o := e.options(idleOffset)
	m := e.plan(o)
	e.commitIn(dir, "late.txt", "late\n")

	_, err := e.apply(o, m)
	if err == nil || !strings.Contains(err.Error(), "HEAD moved") {
		t.Fatalf("err = %v, want a HEAD-moved refusal", err)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Errorf("tree removed although its HEAD moved: %v", serr)
	}
}

func TestApplyDevWorktrees_APathThatIsNotADevTreeIsRefusedAndKeepItemsAreIgnored(t *testing.T) {
	e := newDevEnv(t)
	keep := e.addDev("kept")
	o := e.options(0)
	forged := DevManifest{Items: []DevItem{
		{Path: e.runtime, Action: DevActionArchive, Head: "x"},
		{Path: keep, Action: DevActionKeep},
	}}
	recs, err := e.apply(o, forged)
	if err == nil || !strings.Contains(err.Error(), "not a dev worktree of the hub") {
		t.Fatalf("err = %v, want the runtime plane refused", err)
	}
	for _, p := range []string{e.runtime, keep} {
		if _, serr := os.Stat(p); serr != nil || len(recs) != 0 {
			t.Errorf("%s: stat err %v, records %+v; want untouched", p, serr, recs)
		}
	}
}

func TestApplyDevWorktrees_ALockedTreeFailsTheRemovalAndWritesNoRecord(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("locked")
	o := e.options(idleOffset)
	m := e.plan(o)
	e.store.Git("worktree", "lock", dir)

	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 {
		t.Fatalf("records %+v err %v, want a removal error and no record", recs, err)
	}
	if stored, _ := ReadDevArchive(e.evolve); len(stored) != 0 {
		t.Errorf("manifest = %+v, want no record for a failed removal", stored)
	}
}

func TestApplyDevWorktrees_AManifestWriteFailureStillReportsTheRecord(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("unrecorded")
	o := e.options(idleOffset)
	m := e.plan(o)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	o.EvolveDir = blocker

	recs, err := e.apply(o, m)
	if err == nil || !strings.Contains(err.Error(), "was archived but its record was not written") {
		t.Fatalf("err = %v, want the record-write failure with the record inside", err)
	}
	if len(recs) != 1 || recs[0].Branch != "br-unrecorded" {
		t.Errorf("records = %+v, want the record returned so the restore data is not lost", recs)
	}
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Errorf("tree should be removed (stat err %v)", serr)
	}
}

func TestApplyDevWorktrees_SetupFailuresAndSkips(t *testing.T) {
	e := newDevEnv(t)
	if _, err := ApplyDevWorktrees(context.Background(), DevOptions{ProjectRoot: e.runtime}, DevManifest{}); err == nil {
		t.Error("no Exec: nil error, want a refusal")
	}
	plain := newSquashRepo(t)
	o := e.options(0)
	o.ProjectRoot = plain.Dir
	if recs, err := e.apply(o, DevManifest{}); err != nil || recs != nil {
		t.Errorf("plain repo: %v, %v; want a quiet skip", recs, err)
	}
	if err := os.WriteFile(filepath.Join(e.store.Dir, "HEAD"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.apply(e.options(0), DevManifest{}); err == nil {
		t.Error("a corrupt store: nil error, want the listing failure")
	}
}

func TestResolveDeep_FollowsSymlinksOfTheDeepestExistingAncestor(t *testing.T) {
	t.Cleanup(func() { devEvalSymlinks = filepath.EvalSymlinks })
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolvedReal, _ := filepath.EvalSymlinks(real)
	if got, want := resolveDeep(filepath.Join(link, "gone", "deeper")), filepath.Join(resolvedReal, "gone", "deeper"); got != want {
		t.Errorf("resolveDeep = %s, want %s", got, want)
	}
	if got := resolveDeep("/definitely-not-here-xyz/a"); got != "/definitely-not-here-xyz/a" {
		t.Errorf("resolveDeep of an unreachable path = %s, want it unchanged", got)
	}
	devEvalSymlinks = func(string) (string, error) { return "", errors.New("no symlink support") }
	if got := resolveDeep("/a/b"); got != "/a/b" {
		t.Errorf("resolveDeep with no resolvable ancestor = %s, want the cleaned path", got)
	}
	devEvalSymlinks = filepath.EvalSymlinks
	if !pathWithin(link, filepath.Join(real, "x")) || pathWithin(filepath.Join(real, "x"), real) {
		t.Error("pathWithin must see through a symlink and must not count a parent as inside a child")
	}
	if pathWithin(real, "") {
		t.Error("pathWithin(root, \"\") = true, want false")
	}
}
