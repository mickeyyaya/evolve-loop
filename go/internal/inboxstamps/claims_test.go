package inboxstamps

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const claimOfCurated = ".evolve/inbox/processing/cycle-1836/curated.json"

func claim(t *testing.T, w world, rel, claimRel string) {
	t.Helper()
	dest := filepath.Join(w.plane.Dir, claimRel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(w.plane.Dir, rel), dest); err != nil {
		t.Fatal(err)
	}
}

func TestClassify_AClaimedRootItemIsAClaimNotDirt(t *testing.T) {
	w := newWorld(t)
	claim(t, w, curatedItem, claimOfCurated)
	if err := os.Remove(filepath.Join(w.plane.Dir, untouchedItem)); err != nil {
		t.Fatal(err)
	}

	partition, err := Classify(context.Background(), git(w.plane))

	want := []Claimed{{Path: curatedItem, ClaimPath: claimOfCurated}}
	if err != nil || !reflect.DeepEqual(partition.Claimed, want) {
		t.Errorf("Claimed = %+v (err %v), want %+v", partition.Claimed, err, want)
	}
	if !reflect.DeepEqual(partition.Other, []string{untouchedItem}) {
		t.Errorf("Other = %v, want only the deletion that no claim holds", partition.Other)
	}
}

func TestClassify_OnlyAnUnstagedRootDeletionCountsAsAClaim(t *testing.T) {
	w := newWorld(t)
	nested := ".evolve/inbox/retry/curated.json"
	write(t, w.plane, nested, `{"id":"curated-retry"}`)
	w.plane.Git("add", nested)
	w.plane.Git("commit", "-q", "-m", "a nested namesake")
	claim(t, w, curatedItem, claimOfCurated)
	if err := os.Remove(filepath.Join(w.plane.Dir, nested)); err != nil {
		t.Fatal(err)
	}
	staged := ".evolve/inbox/processing/cycle-1836/untouched.json"
	claim(t, w, untouchedItem, staged)
	w.plane.Git("rm", "-q", "--cached", untouchedItem)

	partition, err := Classify(context.Background(), git(w.plane))

	if err != nil || !reflect.DeepEqual(partition.Claimed, []Claimed{{Path: curatedItem, ClaimPath: claimOfCurated}}) {
		t.Errorf("Claimed = %+v (err %v), want only the unstaged root deletion", partition.Claimed, err)
	}
	if !slices.Contains(partition.Other, nested) || !slices.Contains(partition.Other, untouchedItem) {
		t.Errorf("Other = %v, want the nested deletion and the staged deletion as dirt", partition.Other)
	}
}

func TestClassify_NamesTheRevParseAndTheShowFailure(t *testing.T) {
	ctx := context.Background()
	if _, err := Classify(ctx, scriptedGit{status: " M " + untouchedItem + "\x00", failsOn: "rev-parse"}); err == nil || !strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("rev-parse failing: err = %v", err)
	}
	if _, err := Classify(ctx, scriptedGit{status: " M " + untouchedItem + "\x00", failsOn: "show"}); err == nil || !strings.Contains(err.Error(), "git show") {
		t.Errorf("show failing: err = %v", err)
	}
}

func TestClassify_AClaimDirItCannotReadHoldsNoClaim(t *testing.T) {
	w := newWorld(t)
	claim(t, w, curatedItem, claimOfCurated)
	lockDir(t, filepath.Join(w.plane.Dir, ".evolve", "inbox", "processing", "cycle-1836"), 0o000)

	partition, err := Classify(context.Background(), git(w.plane))

	if err != nil || len(partition.Claimed) != 0 || !slices.Contains(partition.Other, curatedItem) {
		t.Errorf("Classify = %+v, %v; want the deletion as dirt when no readable claim holds it", partition, err)
	}
}

func lockDir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}
