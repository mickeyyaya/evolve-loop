package wtcheckpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func TestRefEntryWorktreeBackfillF1(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, want string }{
		{"refs/checkpoints/task/20261006T060000Z", "task"},
		{"refs/checkpoints/nested/name/20261006T060000Z", "nested/name"},
		{"refs/checkpoints/lonely", "lonely"},
		{"refs/checkpoints/", ""},
	}
	for _, tc := range cases {
		if got := (refEntry{name: tc.name}).worktree(); got != tc.want {
			t.Errorf("worktree(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestListAndDeleteRefsBackfillF1(t *testing.T) {
	boom := errors.New("for-each-ref failed")
	t.Run("a listing failure is returned with no refs", func(t *testing.T) {
		refs, err := listRefs(context.Background(), newScriptF1(stepF1{prefix: "for-each-ref", err: boom}).git("/s"), refPrefix)
		if !errors.Is(err, boom) || refs != nil {
			t.Fatalf("refs=%v err=%v", refs, err)
		}
	})
	t.Run("blank lines are skipped and well-formed ones parsed", func(t *testing.T) {
		out := "\nrefs/checkpoints/a/20261006T060000Z o1 t1\n\n"
		refs, err := listRefs(context.Background(), newScriptF1(stepF1{prefix: "for-each-ref", outs: []string{out}}).git("/s"), refPrefix)
		if err != nil || len(refs) != 1 || refs[0] != (refEntry{name: "refs/checkpoints/a/20261006T060000Z", oid: "o1", tree: "t1"}) {
			t.Fatalf("refs=%+v err=%v", refs, err)
		}
	})
	t.Run("deleteRefs stops at the first refusal and reports what it did delete", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "update-ref -d refs/checkpoints/a/1", times: 1},
			stepF1{prefix: "update-ref -d refs/checkpoints/a/2", err: boom},
		)
		refs := []refEntry{{name: "refs/checkpoints/a/1", oid: "o1"}, {name: "refs/checkpoints/a/2", oid: "o2"}, {name: "refs/checkpoints/a/3", oid: "o3"}}
		deleted, err := deleteRefs(context.Background(), s.git("/s"), refs)
		if !errors.Is(err, boom) || len(deleted) != 1 || deleted[0] != "refs/checkpoints/a/1" {
			t.Fatalf("deleted=%v err=%v", deleted, err)
		}
		if s.called("update-ref -d refs/checkpoints/a/3") != 0 {
			t.Fatal("a ref after the failure must not be attempted")
		}
	})
}

func TestDescribeBackfillF1_EachGitStepFailureIsReturnedAndNamed(t *testing.T) {
	boom := errors.New("describe step failed")
	ref := refEntry{name: "refs/checkpoints/task/20261006T060000Z", oid: "oid1", tree: "t"}
	cases := []struct {
		name     string
		fail     string
		wantWrap bool
		wantText string
	}{
		{"no base commit", "rev-parse --verify", true, "has no base commit"},
		{"body unreadable", "show -s", true, ""},
		{"numstat unreadable", "diff --numstat", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newScriptF1(
				stepF1{prefix: tc.fail, err: boom},
				stepF1{prefix: "rev-parse --verify", outs: []string{"base1"}},
				stepF1{prefix: "show -s", outs: []string{"label: x"}},
				stepF1{prefix: "diff --numstat", outs: []string{"1\t2\ta.go"}},
			)
			e, err := describe(context.Background(), s.git("/s"), ref)
			if !errors.Is(err, boom) || !strings.Contains(err.Error(), tc.wantText) || e != (Entry{}) {
				t.Fatalf("entry=%+v err=%v", e, err)
			}
		})
	}
	t.Run("a healthy describe reads label and totals", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "rev-parse --verify", outs: []string{"base1"}},
			stepF1{prefix: "show -s", outs: []string{"checkpoint body\nlabel: before refactor\n"}},
			stepF1{prefix: "diff --numstat", outs: []string{"1\t2\ta.go\n-\t-\tbin.dat\n3\t0\tdir/b.go"}},
		)
		e, err := describe(context.Background(), s.git("/s"), ref)
		if err != nil || e.Base != "base1" || e.Label != "before refactor" || e.Files != 3 || e.Added != 4 || e.Deleted != 2 || e.Worktree != "task" {
			t.Fatalf("entry=%+v err=%v", e, err)
		}
	})
}

func TestSumNumstatBackfillF1(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		in                   string
		files, added, deletd int
		wantErr              string
	}{
		{name: "empty", in: ""},
		{name: "binary rows count as files but add no lines", in: "-\t-\tlogo.png\n2\t1\ta.go", files: 2, added: 2, deletd: 1},
		{name: "a malformed count aborts", in: "x\t1\ta.go", wantErr: `unreadable numstat line "x\t1\ta.go"`},
		{name: "a malformed deletion count aborts", in: "1\ty\ta.go", wantErr: "unreadable numstat line"},
		{name: "lines without three fields are ignored", in: "garbage\n4\t5\n6\t7\tc.go", files: 1, added: 6, deletd: 7},
		{name: "tabs inside the path stay in the path", in: "1\t1\ta\tb.go", files: 1, added: 1, deletd: 1},
	}
	for _, tc := range cases {
		files, added, deleted, err := sumNumstat(tc.in)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || files != 0 || added != 0 || deleted != 0 {
				t.Errorf("%s: (%d,%d,%d,%v), want zero totals and %q", tc.name, files, added, deleted, err, tc.wantErr)
			}
			continue
		}
		if err != nil || files != tc.files || added != tc.added || deleted != tc.deletd {
			t.Errorf("%s: (%d,%d,%d,%v), want (%d,%d,%d)", tc.name, files, added, deleted, err, tc.files, tc.added, tc.deletd)
		}
	}
}

func hubOverScriptF1(t *testing.T, s *scriptF1) Hub {
	t.Helper()
	useScriptF1(t, s)
	return Hub{plane.Hub{Root: "/hub", Store: "/hub/.store"}}
}

func TestPruneBackfillF1(t *testing.T) {
	boom := errors.New("prune step failed")
	oldRef := "refs/checkpoints/task/20261006T060000Z"
	newRef := "refs/checkpoints/task/20261007T060000Z"
	listing := oldRef + " o1 t1\n" + newRef + " o2 t2\n"
	t.Run("a ref listing failure deletes nothing", func(t *testing.T) {
		h := hubOverScriptF1(t, newScriptF1(stepF1{prefix: "for-each-ref", err: boom}))
		got, err := Prune(context.Background(), h, PruneOptions{Keep: 1})
		if !errors.Is(err, boom) || got != nil {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("a deletion refused midway reports what was already pruned", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "for-each-ref", outs: []string{listing}},
			stepF1{prefix: "update-ref -d", err: boom},
		)
		got, err := Prune(context.Background(), hubOverScriptF1(t, s), PruneOptions{Keep: 1})
		if !errors.Is(err, boom) || len(got) != 0 {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if s.called("update-ref -d "+oldRef) != 1 || s.called("update-ref -d "+newRef) != 0 {
			t.Fatalf("only the over-retention ref may be attempted: %v", s.calls)
		}
	})
	t.Run("the second of two refused deletions keeps the first in the result", func(t *testing.T) {
		three := oldRef + " o1 t1\nrefs/checkpoints/task/20261008T060000Z o3 t3\nrefs/checkpoints/task/20261009T060000Z o4 t4\n"
		s := newScriptF1(
			stepF1{prefix: "for-each-ref", outs: []string{three}},
			stepF1{prefix: "update-ref -d " + oldRef, times: 1},
			stepF1{prefix: "update-ref -d", err: boom},
		)
		got, err := Prune(context.Background(), hubOverScriptF1(t, s), PruneOptions{Keep: 1})
		if !errors.Is(err, boom) || len(got) != 1 || got[0].Ref != oldRef || got[0].Reason != PruneRetention {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("a landed prune without origin/main stops before judging", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "for-each-ref", outs: []string{listing}},
			stepF1{prefix: "rev-parse --verify", err: boom},
		)
		got, err := Prune(context.Background(), hubOverScriptF1(t, s), PruneOptions{Keep: 5, Landed: true})
		if !errors.Is(err, boom) || got != nil || !strings.Contains(err.Error(), plane.OriginMainRef) {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
}

func TestHasLandedBackfillF1(t *testing.T) {
	boom := errors.New("diff failed")
	r := refEntry{name: "refs/checkpoints/task/20261006T060000Z", oid: "oid1"}
	t.Run("the checkpoint's own diff cannot be read", func(t *testing.T) {
		landed, err := hasLanded(context.Background(), newScriptF1(stepF1{prefix: "diff --name-only", err: boom}).git("/s"), r)
		if !errors.Is(err, boom) || landed {
			t.Fatalf("landed=%v err=%v", landed, err)
		}
	})
	t.Run("the comparison with origin/main cannot be read", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "diff --name-only -z --no-renames oid1~2 oid1", outs: []string{"a.go\x00"}},
			stepF1{prefix: "diff --name-only", err: boom},
		)
		landed, err := hasLanded(context.Background(), s.git("/s"), r)
		if !errors.Is(err, boom) || landed {
			t.Fatalf("landed=%v err=%v", landed, err)
		}
	})
	t.Run("a checkpoint that changed nothing never counts as landed", func(t *testing.T) {
		landed, err := hasLanded(context.Background(), newScriptF1(stepF1{prefix: "diff --name-only", outs: []string{""}}).git("/s"), r)
		if err != nil || landed {
			t.Fatalf("landed=%v err=%v", landed, err)
		}
	})
	t.Run("landed only when none of its changed paths still differ from main", func(t *testing.T) {
		mk := func(stillDiffers string) bool {
			s := newScriptF1(
				stepF1{prefix: "diff --name-only -z --no-renames oid1~2 oid1", outs: []string{"a.go\x00b.go\x00"}},
				stepF1{prefix: "diff --name-only", outs: []string{stillDiffers}},
			)
			landed, err := hasLanded(context.Background(), s.git("/s"), r)
			if err != nil {
				t.Fatal(err)
			}
			return landed
		}
		if !mk("") || !mk("unrelated.go\x00") {
			t.Error("paths identical to main (or differing only elsewhere) mean the work landed")
		}
		if mk("b.go\x00") {
			t.Error("one changed path still differing from main means it has not landed")
		}
	})
}

func TestResolveCheckpointBackfillF1(t *testing.T) {
	t.Run("not a checkpoint", func(t *testing.T) {
		boom := errors.New("bad revision")
		_, err := resolveCheckpoint(context.Background(), newScriptF1(stepF1{prefix: "rev-parse", err: boom}).git("/s"), "task/stamp")
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "refs/checkpoints/task/stamp is not a checkpoint") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("an answer that does not carry three ids", func(t *testing.T) {
		_, err := resolveCheckpoint(context.Background(), newScriptF1(stepF1{prefix: "rev-parse", outs: []string{"one two"}}).git("/s"), "refs/checkpoints/task/stamp")
		if err == nil || !strings.Contains(err.Error(), `resolved to "one two", want base, staged and full`) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the prefix is optional and the three ids map to base, staged, full", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "rev-parse", outs: []string{"b\ns\nf"}})
		cp, err := resolveCheckpoint(context.Background(), s.git("/s"), "task/stamp")
		if err != nil || cp != (checkpoint{ref: "refs/checkpoints/task/stamp", base: "b", staged: "s", full: "f"}) {
			t.Fatalf("cp=%+v err=%v", cp, err)
		}
	})
}

func TestPrepareTargetBackfillF1(t *testing.T) {
	cp := checkpoint{ref: "refs/checkpoints/task/stamp", base: "base1", staged: "s", full: "f"}
	t.Run("a path that cannot be made absolute", func(t *testing.T) {
		orig := checkpointAbs
		t.Cleanup(func() { checkpointAbs = orig })
		boom := errors.New("getwd failed")
		checkpointAbs = func(string) (string, error) { return "", boom }
		res, err := prepareTarget(context.Background(), hubOverScriptF1(t, newScriptF1()), "rel/dir", cp)
		if !errors.Is(err, boom) || res.Dir != "" || res.Ref != cp.ref || res.Base != "base1" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("a target that cannot be inspected is not treated as absent", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "a-file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		s := newScriptF1()
		res, err := prepareTarget(context.Background(), hubOverScriptF1(t, s), filepath.Join(file, "inside"), cp)
		if err == nil || res.Dir != "" || len(s.calls) != 0 {
			t.Fatalf("res=%+v err=%v git calls=%v; a path under a regular file must neither create nor probe a worktree", res, err, s.calls)
		}
	})
	t.Run("a worktree add git refuses is reported with its code and stderr", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "fresh")
		s := newScriptF1(stepF1{prefix: "worktree add", code: 128, stderr: "fatal: '" + target + "' already exists\n"})
		res, err := prepareTarget(context.Background(), hubOverScriptF1(t, s), target, cp)
		if err == nil || !strings.Contains(err.Error(), "git worktree add "+target+" rc=128") || !strings.Contains(err.Error(), "already exists") || res.Created {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if s.called("worktree add") != 1 {
			t.Fatalf("a permanent refusal must not be retried: %v", s.calls)
		}
	})
	t.Run("a new target is created detached at the base", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "fresh")
		s := newScriptF1(stepF1{prefix: "worktree add --detach " + target + " base1"})
		res, err := prepareTarget(context.Background(), hubOverScriptF1(t, s), target, cp)
		if err != nil || !res.Created || !res.Detached || res.Dir != target {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestReadyExistingBackfillF1(t *testing.T) {
	boom := errors.New("tree step failed")
	cp := checkpoint{ref: "r", base: "base1", staged: "s", full: "f"}
	t.Run("the dirty check fails", func(t *testing.T) {
		ok, err := readyExisting(context.Background(), newScriptF1(stepF1{prefix: "status", err: boom}).git("/wt"), cp)
		if !errors.Is(err, boom) || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("a dirty tree is refused with the paths listed", func(t *testing.T) {
		ok, err := readyExisting(context.Background(), newScriptF1(stepF1{prefix: "status", outs: []string{" M a.go\n?? b.go\n"}}).git("/wt"), cp)
		if err == nil || ok || !strings.Contains(err.Error(), "is dirty (2 path(s): a.go, b.go)") {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("HEAD cannot be read", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "status"}, stepF1{prefix: "rev-parse HEAD", err: boom})
		ok, err := readyExisting(context.Background(), s.git("/wt"), cp)
		if !errors.Is(err, boom) || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("the collision check fails", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "status"}, stepF1{prefix: "rev-parse HEAD", outs: []string{"head1"}}, stepF1{prefix: "diff --name-only", err: boom})
		ok, err := readyExisting(context.Background(), s.git("/wt"), cp)
		if !errors.Is(err, boom) || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("a tree already at the base is used as it is", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "status"}, stepF1{prefix: "rev-parse HEAD", outs: []string{"base1"}},
			stepF1{prefix: "diff --name-only"}, stepF1{prefix: "ls-files"},
		)
		ok, err := readyExisting(context.Background(), s.git("/wt"), cp)
		if err != nil || ok || s.called("checkout") != 0 {
			t.Fatalf("detached=%v err=%v calls=%v", ok, err, s.calls)
		}
	})
	t.Run("a tree elsewhere is detached at the base", func(t *testing.T) {
		s := newScriptF1(
			stepF1{prefix: "status"}, stepF1{prefix: "rev-parse HEAD", outs: []string{"head1"}},
			stepF1{prefix: "diff --name-only"}, stepF1{prefix: "ls-files"}, stepF1{prefix: "checkout -q --detach base1"},
		)
		ok, err := readyExisting(context.Background(), s.git("/wt"), cp)
		if err != nil || !ok {
			t.Fatalf("detached=%v err=%v", ok, err)
		}
	})
}

func TestIgnoredCollisionsBackfillF1(t *testing.T) {
	boom := errors.New("collision step failed")
	cp := checkpoint{base: "base1", full: "full1"}
	run := func(steps ...stepF1) ([]string, error) {
		return ignoredCollisions(context.Background(), newScriptF1(steps...).git("/wt"), "head1", cp)
	}
	if _, err := run(stepF1{prefix: "diff --name-only", err: boom}); !errors.Is(err, boom) {
		t.Errorf("moved-paths failure = %v", err)
	}
	if _, err := run(stepF1{prefix: "diff --name-only -z --no-renames head1 base1", times: 1}, stepF1{prefix: "diff --name-only", err: boom}); !errors.Is(err, boom) {
		t.Errorf("restored-paths failure = %v", err)
	}
	if _, err := run(stepF1{prefix: "diff --name-only"}, stepF1{prefix: "ls-files", err: boom}); !errors.Is(err, boom) {
		t.Errorf("ignored-listing failure = %v", err)
	}
	hits, err := run(
		stepF1{prefix: "diff --name-only -z --no-renames head1 base1", outs: []string{"keep.go\x00out/x.txt\x00"}},
		stepF1{prefix: "diff --name-only -z --no-renames base1 full1", outs: []string{"build.log\x00deep/er/y.txt\x00"}},
		stepF1{prefix: "ls-files", outs: []string{"build.log\x00out/\x00deep/\x00"}},
	)
	if err != nil || strings.Join(hits, ",") != "out/x.txt,build.log,deep/er/y.txt" {
		t.Errorf("hits = %v, err = %v; want files in ignored dirs and ignored files, not tracked ones", hits, err)
	}
}

func TestApplyCheckpointBackfillF1(t *testing.T) {
	boom := errors.New("apply step failed")
	cp := checkpoint{staged: "staged1", full: "full1"}
	t.Run("the merge into the working tree fails", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "read-tree -m -u HEAD full1", err: boom})
		if err := applyCheckpoint(context.Background(), s.git("/wt"), cp); !errors.Is(err, boom) || s.called("read-tree staged1") != 0 {
			t.Fatalf("err=%v calls=%v", err, s.calls)
		}
	})
	t.Run("resetting the index to the staged tree fails", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "read-tree -m -u HEAD full1"}, stepF1{prefix: "read-tree staged1", err: boom})
		if err := applyCheckpoint(context.Background(), s.git("/wt"), cp); !errors.Is(err, boom) || s.called("update-index") != 0 {
			t.Fatalf("err=%v calls=%v", err, s.calls)
		}
	})
	t.Run("the refresh tolerates exit 1 and rejects anything worse", func(t *testing.T) {
		for code, wantErr := range map[int]bool{0: false, 1: false, 2: true, 128: true} {
			s := newScriptF1(stepF1{prefix: "read-tree"}, stepF1{prefix: "update-index -q --refresh", code: code, stderr: " broken index \n"})
			err := applyCheckpoint(context.Background(), s.git("/wt"), cp)
			if (err != nil) != wantErr {
				t.Errorf("exit %d: err = %v, want error=%v", code, err, wantErr)
			}
			if wantErr && !strings.Contains(err.Error(), "update-index --refresh rc="+strconv.Itoa(code)+" err=<nil>: broken index") {
				t.Errorf("exit %d: err = %v, want the code and trimmed stderr", code, err)
			}
		}
	})
	t.Run("a refresh that cannot run is an error even with exit 0", func(t *testing.T) {
		s := newScriptF1(stepF1{prefix: "read-tree"}, stepF1{prefix: "update-index", err: boom})
		if err := applyCheckpoint(context.Background(), s.git("/wt"), cp); err == nil || !strings.Contains(err.Error(), "update-index --refresh") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRestoreBackfillF1_AnApplyFailureNamesTheCheckpointAndTheTarget(t *testing.T) {
	boom := errors.New("read-tree refused")
	target := filepath.Join(t.TempDir(), "fresh")
	s := newScriptF1(
		stepF1{prefix: "rev-parse", outs: []string{"base1\nstaged1\nfull1"}},
		stepF1{prefix: "worktree add"},
		stepF1{prefix: "read-tree", err: boom},
	)
	h := hubOverScriptF1(t, s)
	res, err := Restore(context.Background(), h, "task/stamp", target)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "restore refs/checkpoints/task/stamp into "+target) || res != (RestoreResult{}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRestoreBackfillF1_FailuresBeforeThePreparationReturnAnEmptyResult(t *testing.T) {
	boom := errors.New("rev-parse failed")
	h := hubOverScriptF1(t, newScriptF1(stepF1{prefix: "rev-parse", err: boom}))
	res, err := Restore(context.Background(), h, "task/stamp", filepath.Join(t.TempDir(), "x"))
	if !errors.Is(err, boom) || res != (RestoreResult{}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	h = hubOverScriptF1(t, newScriptF1(stepF1{prefix: "rev-parse", outs: []string{"b\ns\nf"}}, stepF1{prefix: "worktree add", code: 128, stderr: "already exists"}))
	res, err = Restore(context.Background(), h, "task/stamp", filepath.Join(t.TempDir(), "x"))
	if err == nil || res != (RestoreResult{}) {
		t.Fatalf("res=%+v err=%v; a refused preparation yields no partial result", res, err)
	}
}
