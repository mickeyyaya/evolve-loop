package proctree

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSaveTree_RoundTripsThroughLoadTree(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "dispatch-trees")
	id := "01R/1835/code/review/p4242n1"
	ids := []Identity{{Pid: 10, Started: t0}, {Pid: 11, Started: t0.Add(time.Second)}}

	if err := SaveTree(dir, id, ids); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}
	got, err := LoadTree(dir, id)

	if err != nil || !reflect.DeepEqual(Recorded(got)(Process{Pid: 11, Started: t0.Add(time.Second)}), true) || len(got) != 2 {
		t.Errorf("LoadTree = %+v, %v, want the two saved identities", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || strings.ContainsAny(entries[0].Name(), "/") || !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Errorf("entries = %v, want one flat .json file", entries)
	}
}

func TestLoadTree_AMissingTreeIsEmptyAndNoError(t *testing.T) {
	t.Parallel()
	got, err := LoadTree(t.TempDir(), "01R/1/build/p5n1")

	if err != nil || len(got) != 0 {
		t.Errorf("LoadTree = %v, %v, want an empty tree", got, err)
	}
}

func TestTreeFile_StaysInsideItsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, id := range []string{"../../etc/x", "..", "a/../../b", "r/1/a\x00b/p5n1"} {
		p := TreeFile(dir, id)
		if filepath.Dir(p) != dir {
			t.Errorf("TreeFile(%q) = %q, want a file directly in %q", id, p, dir)
		}
	}
}

func TestTreeDir_IsUnderTheEvolveDirOfTheProject(t *testing.T) {
	t.Parallel()
	if got := TreeDir("/hub/runtime"); got != "/hub/runtime/.evolve/dispatch-trees" {
		t.Errorf("TreeDir = %q", got)
	}
}

func TestRemoveTree_DeletesTheFileAndToleratesItsAbsence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := "01R/1/build/p5n1"
	if err := SaveTree(dir, id, []Identity{{Pid: 10, Started: t0}}); err != nil {
		t.Fatal(err)
	}

	if err := RemoveTree(dir, id); err != nil {
		t.Fatalf("RemoveTree: %v", err)
	}
	if err := RemoveTree(dir, id); err != nil {
		t.Errorf("a second RemoveTree = %v, want nil", err)
	}
	if _, err := os.Stat(TreeFile(dir, id)); !os.IsNotExist(err) {
		t.Errorf("the tree file is still there: %v", err)
	}
}

func TestSaveTree_MakesItsDirectoryPrivate(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "dispatch-trees")

	if err := SaveTree(dir, "01R/1/build/p5n1", nil); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v, want 0700", fi.Mode().Perm(), err)
	}
}
