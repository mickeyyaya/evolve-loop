package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLeafStats_MissingDirReturnsTheStatError(t *testing.T) {
	t.Parallel()
	_, _, _, err := leafStats(filepath.Join(t.TempDir(), "absent"))
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want a not-exist error", err)
	}
}

func TestLeafStats_UnreadableSubdirReturnsTheWalkError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, _, _, err := leafStats(dir); !os.IsPermission(err) {
		t.Fatalf("err = %v, want a permission error from the walk", err)
	}
}

func TestLeafStats_SumsRegularFilesAndTakesTheNewestModTime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.patch"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	newest := filepath.Join(dir, "sub", "b.bundle")
	if err := os.WriteFile(newest, []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	fresh := old.Add(24 * time.Hour)
	for _, p := range []string{filepath.Join(dir, "a.patch"), filepath.Join(dir, "sub")} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(newest, fresh, fresh); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}

	size, dirMod, newestMod, err := leafStats(dir)
	if err != nil {
		t.Fatalf("leafStats: %v", err)
	}
	if size != 8 {
		t.Errorf("size = %d, want 8 (directories add nothing)", size)
	}
	if !dirMod.Equal(old) {
		t.Errorf("dirMod = %v, want %v", dirMod, old)
	}
	if !newestMod.Equal(fresh) {
		t.Errorf("newestMod = %v, want the newer file time %v", newestMod, fresh)
	}
}
