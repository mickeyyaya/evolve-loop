package gc

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const salvageTestSHA = "0123456789abcdef0123456789abcdef01234567"

func writeSalvageArchive(t *testing.T, path string, names ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, n := range names {
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []interface{ Close() error }{tw, gz, f} {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListSalvage_ReadsEveryLeafAndNamesEachUnreadableOne(t *testing.T) {
	evolveDir := t.TempDir()
	dir := OperatorSalvageDir(evolveDir)
	longLine := strings.Repeat("y", 200_000)
	patch := "diff --git a/a b/a\n+" + longLine + "\ndiff --git a/b b/b\n+z\n"
	writeFile(t, filepath.Join(dir, "cycle-aaa-20", "HEAD"), salvageTestSHA+" cycle-aaa-20\n")
	writeFile(t, filepath.Join(dir, "cycle-aaa-20", "uncommitted.patch"), patch)
	writeSalvageArchive(t, filepath.Join(dir, "cycle-aaa-20", "untracked.tgz"), "a", "b", "c")
	writeFile(t, filepath.Join(dir, "cycle-282-main-tree", "HEAD"), salvageTestSHA+strings.Repeat("0", 24)+"\n")
	writeFile(t, filepath.Join(dir, "cycle-aaa-3", "HEAD"), salvageTestSHA+"\n")
	writeFile(t, filepath.Join(dir, ".DS_Store"), "stray")
	writeFile(t, filepath.Join(dir, "cycle-bad-4", "HEAD"), "not-a-sha branch\n")
	writeFile(t, filepath.Join(dir, "cycle-bad-5", "HEAD"), salvageTestSHA+" a b\n")
	writeFile(t, filepath.Join(dir, "cycle-bad-6", "uncommitted.patch"), patch)
	writeFile(t, filepath.Join(dir, "cycle-bad-7", "HEAD"), salvageTestSHA+"\n")
	writeFile(t, filepath.Join(dir, "cycle-bad-7", "untracked.tgz"), "not gzip")

	leaves, err := ListSalvage(evolveDir)

	var order []string
	for _, l := range leaves {
		order = append(order, l.Leaf)
	}
	if want := []string{"cycle-282-main-tree", "cycle-aaa-3", "cycle-aaa-20"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("leaves %v, want the readable ones sorted by (cycle, leaf) %v", order, want)
	}
	if l := leaves[2]; l.Cycle != 20 || l.Branch != "cycle-aaa-20" || l.ChangedFiles != 2 || l.PatchBytes != int64(len(patch)) || l.UntrackedFiles != 3 {
		t.Errorf("cycle-aaa-20 = %+v, want cycle 20, its branch, 2 changed files over a %d-byte patch with a long line, 3 untracked", l, len(patch))
	}
	if l := leaves[0]; l.Cycle != 0 || len(l.Head) != 64 || l.Branch != "" || l.ChangedFiles != 0 || l.PatchBytes != 0 || l.UntrackedFiles != 0 {
		t.Errorf("cycle-282-main-tree = %+v, want cycle 0, a 64-hex head, no branch and zero counts", l)
	}
	if err == nil {
		t.Fatal("ListSalvage err = nil, want one error per unreadable leaf")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 4 {
		t.Errorf("err has %d lines, want 4 (one per unreadable leaf): %v", len(lines), err)
	}
	for _, bad := range []string{"cycle-bad-4", "cycle-bad-5", "cycle-bad-6", "cycle-bad-7"} {
		if !strings.Contains(err.Error(), "gc: salvage leaf "+bad+": ") {
			t.Errorf("err does not name %s: %v", bad, err)
		}
	}
}

func TestListSalvage_LeaflessDirectories(t *testing.T) {
	missing := t.TempDir()
	empty := t.TempDir()
	if err := os.MkdirAll(OperatorSalvageDir(empty), 0o755); err != nil {
		t.Fatal(err)
	}
	stray := t.TempDir()
	writeFile(t, filepath.Join(OperatorSalvageDir(stray), ".DS_Store"), "stray")
	for _, evolveDir := range []string{missing, empty, stray} {
		leaves, err := ListSalvage(evolveDir)
		if err != nil || leaves == nil || len(leaves) != 0 {
			t.Errorf("ListSalvage(%s) = %#v, %v; want an empty non-nil slice and nil", evolveDir, leaves, err)
		}
	}
	if _, err := os.Stat(OperatorSalvageDir(missing)); !os.IsNotExist(err) {
		t.Errorf("ListSalvage created the missing salvage dir (stat err=%v)", err)
	}

	notADir := t.TempDir()
	writeFile(t, OperatorSalvageDir(notADir), "a file where the salvage dir must be")
	if leaves, err := ListSalvage(notADir); err == nil || leaves != nil || !strings.Contains(err.Error(), "gc: salvage list: read ") {
		t.Errorf("ListSalvage on an unreadable salvage dir = %v, %v; want no leaves and a salvage-list read error", leaves, err)
	}
}
