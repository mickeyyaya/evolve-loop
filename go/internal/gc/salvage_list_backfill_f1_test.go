package gc

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func leafWithHeadF1(t *testing.T) string {
	t.Helper()
	leaf := filepath.Join(t.TempDir(), "cycle-aaa-7")
	writeFile(t, filepath.Join(leaf, "HEAD"), salvageTestSHA+" cycle-aaa-7\n")
	return leaf
}

func TestListSalvageBackfillF1_LeavesOfOneCycleAreOrderedByName(t *testing.T) {
	evolve := t.TempDir()
	dir := OperatorSalvageDir(evolve)
	for _, leaf := range []string{"cycle-zzz-7", "cycle-aaa-7", "cycle-mmm-7", "cycle-bbb-3"} {
		writeFile(t, filepath.Join(dir, leaf, "HEAD"), salvageTestSHA+"\n")
	}
	leaves, err := ListSalvage(evolve)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, l := range leaves {
		order = append(order, l.Leaf)
	}
	if got := strings.Join(order, ","); got != "cycle-bbb-3,cycle-aaa-7,cycle-mmm-7,cycle-zzz-7" {
		t.Fatalf("order = %s, want cycle number first and the leaf name as the tie-break", got)
	}
}

func TestReadSalvageLeafBackfillF1_EachUnreadablePartFailsTheLeafWithItsOwnReason(t *testing.T) {
	requireNonRootF1(t)
	t.Run("the leaf directory is gone", func(t *testing.T) {
		if _, err := readSalvageLeaf(filepath.Join(t.TempDir(), "absent")); !os.IsNotExist(err) {
			t.Fatalf("err = %v, want not-exist", err)
		}
	})
	t.Run("no HEAD record", func(t *testing.T) {
		leaf := filepath.Join(t.TempDir(), "cycle-aaa-7")
		if err := os.MkdirAll(leaf, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := readSalvageLeaf(leaf); err == nil || !strings.Contains(err.Error(), "read HEAD") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the patch is a directory so it cannot be read", func(t *testing.T) {
		leaf := leafWithHeadF1(t)
		if err := os.Mkdir(filepath.Join(leaf, salvagePatchFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := readSalvageLeaf(leaf); err == nil || !strings.Contains(err.Error(), "read "+salvagePatchFile) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the patch exists but cannot be opened", func(t *testing.T) {
		leaf := leafWithHeadF1(t)
		patch := filepath.Join(leaf, salvagePatchFile)
		writeFile(t, patch, "diff --git a/x b/x\n")
		if err := os.Chmod(patch, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(patch, 0o644) })
		if _, err := readSalvageLeaf(leaf); err == nil || !strings.Contains(err.Error(), "open "+salvagePatchFile) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the archive exists but cannot be opened", func(t *testing.T) {
		leaf := leafWithHeadF1(t)
		archive := filepath.Join(leaf, salvageUntrackedFile)
		writeSalvageArchive(t, archive, "a")
		if err := os.Chmod(archive, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(archive, 0o644) })
		if _, err := readSalvageLeaf(leaf); err == nil || !strings.Contains(err.Error(), "open "+salvageUntrackedFile) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the archive is a gzip stream holding no valid tar", func(t *testing.T) {
		leaf := leafWithHeadF1(t)
		f, err := os.Create(filepath.Join(leaf, salvageUntrackedFile))
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(f)
		_, _ = gz.Write([]byte(strings.Repeat("this is not a tar header ", 40)))
		_ = gz.Close()
		_ = f.Close()
		if _, err := readSalvageLeaf(leaf); err == nil || !strings.Contains(err.Error(), "read "+salvageUntrackedFile) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCountUntrackedArchiveBackfillF1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if n, err := countUntrackedArchive(filepath.Join(dir, "absent.tgz")); n != 0 || err != nil {
		t.Errorf("a missing archive counts zero without error, got %d, %v", n, err)
	}
	empty := filepath.Join(dir, "empty.tgz")
	writeSalvageArchive(t, empty)
	if n, err := countUntrackedArchive(empty); n != 0 || err != nil {
		t.Errorf("an archive with no entries counts zero, got %d, %v", n, err)
	}
	one := filepath.Join(dir, "one.tgz")
	writeSalvageArchive(t, one, "only")
	if n, err := countUntrackedArchive(one); n != 1 || err != nil {
		t.Errorf("one entry counts one, got %d, %v", n, err)
	}
}

func TestSalvagePatchStatsBackfillF1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name        string
		body        string
		wantChanged int
		wantBytes   int64
	}{
		{"empty patch", "", 0, 0},
		{"no trailing newline still counts the last header", "diff --git a/a b/a", 1, 18},
		{"a context line mentioning diff is not a header", " diff --git a/a b/a\n+diff --git x\n", 0, 34},
		{"two files", "diff --git a/a b/a\n+x\ndiff --git a/b b/b\n+y\n", 2, 44},
	}
	for _, tc := range cases {
		p := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "_"))
		writeFile(t, p, tc.body)
		changed, size, err := salvagePatchStats(p)
		if err != nil || changed != tc.wantChanged || size != tc.wantBytes {
			t.Errorf("%s: (%d, %d, %v), want (%d, %d, nil)", tc.name, changed, size, err, tc.wantChanged, tc.wantBytes)
		}
	}
}
