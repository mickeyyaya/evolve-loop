package inboxstamps

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const quarantinedCurated = ".evolve/inbox/quarantine/curated.json"

func quarantineCurated(t *testing.T, w world, body string) {
	t.Helper()
	if err := os.Remove(filepath.Join(w.plane.Dir, curatedItem)); err != nil {
		t.Fatal(err)
	}
	write(t, w.plane, quarantinedCurated, body)
}

func TestClassify_AQuarantineMoveWithOnlyTheLoopsStampsIsAMove(t *testing.T) {
	w := newWorld(t)
	quarantineCurated(t, w, routeStamp("curated"))

	partition, err := Classify(context.Background(), git(w.plane))

	want := []Move{{Path: curatedItem, QuarantinePath: quarantinedCurated}}
	if err != nil || !reflect.DeepEqual(partition.Moved, want) || len(partition.Other) != 0 {
		t.Errorf("Classify = (%+v, %v), want Moved %+v and no other dirt", partition, err, want)
	}
}

func TestClassify_ARewrittenOrMissingQuarantineCopyLeavesTheDeletionAsDirt(t *testing.T) {
	rewritten := newWorld(t)
	quarantineCurated(t, rewritten, `{"id":"curated","weight":0.9,"failure_count":1}`)
	missing := newWorld(t)
	if err := os.Remove(filepath.Join(missing.plane.Dir, curatedItem)); err != nil {
		t.Fatal(err)
	}

	for name, w := range map[string]world{"rewritten": rewritten, "missing": missing} {
		partition, err := Classify(context.Background(), git(w.plane))

		if err != nil || len(partition.Moved) != 0 || !reflect.DeepEqual(partition.Other, []string{curatedItem}) {
			t.Errorf("%s copy: Classify = (%+v, %v), want the deletion as other dirt", name, partition, err)
		}
	}
}

func TestQuarantineMove_NamesAHeadItemItCannotReadAndIgnoresAMissingOne(t *testing.T) {
	w := newWorld(t)
	write(t, w.plane, quarantinedCurated, routeStamp("curated"))
	tree := worktree{root: w.plane.Dir}
	ctx := context.Background()

	if _, ok, err := tree.quarantineMove(ctx, scriptedGit{failsOn: "show"}, " D", curatedItem); ok || err == nil || !strings.Contains(err.Error(), "git show") {
		t.Errorf("unreadable HEAD item: ok=%v err=%v, want the show failure", ok, err)
	}
	if _, ok, err := tree.quarantineMove(ctx, scriptedGit{failsOn: "cat-file"}, " D", curatedItem); ok || err != nil {
		t.Errorf("HEAD item absent: ok=%v err=%v, want no move and no error", ok, err)
	}
	if _, ok, err := tree.quarantineMove(ctx, git(w.plane), " M", curatedItem); ok || err != nil {
		t.Errorf("modified item: ok=%v err=%v, want no move", ok, err)
	}
}

func TestClassify_AHeadItemItCannotReadFailsTheClassification(t *testing.T) {
	w := newWorld(t)
	quarantineCurated(t, w, routeStamp("curated"))

	scripted := moveFailingGit{real: git(w.plane)}
	if _, err := Classify(context.Background(), scripted); err == nil || !strings.Contains(err.Error(), "git show") {
		t.Errorf("Classify err = %v, want the show failure", err)
	}
}

type moveFailingGit struct{ real Git }

func (g moveFailingGit) Capture(ctx context.Context, args ...string) (string, string, int, error) {
	if args[0] == "show" {
		return "", "boom", 1, nil
	}
	return g.real.Capture(ctx, args...)
}

func TestCommitMoves_RecordsBothHalvesOfEachMoveInOneCommit(t *testing.T) {
	w := newWorld(t)
	quarantineCurated(t, w, routeStamp("curated"))
	ctx := context.Background()

	if err := CommitMoves(ctx, git(w.plane), nil); err != nil {
		t.Fatalf("no moves: %v", err)
	}
	err := CommitMoves(ctx, git(w.plane), []Move{{Path: curatedItem, QuarantinePath: quarantinedCurated}})

	if err != nil {
		t.Fatalf("CommitMoves: %v", err)
	}
	if status := w.plane.Git("status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Errorf("tracked tree dirty after the commit:\n%s", status)
	}
	if got := w.plane.Git("ls-tree", "-r", "--name-only", "HEAD", "--", quarantinedCurated, curatedItem); got != quarantinedCurated {
		t.Errorf("HEAD tracks %q, want only the quarantine copy", got)
	}
}

func TestCommitMoves_NamesTheGitCommandThatFailed(t *testing.T) {
	moves := []Move{{Path: curatedItem, QuarantinePath: quarantinedCurated}}
	for _, failing := range []string{"add", "commit"} {
		err := CommitMoves(context.Background(), scriptedGit{failsOn: failing}, moves)

		if err == nil || !strings.Contains(err.Error(), "git "+failing) {
			t.Errorf("git %s failing: err = %v", failing, err)
		}
	}
}
