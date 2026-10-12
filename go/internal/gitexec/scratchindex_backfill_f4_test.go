package gitexec

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyIndex_CopiesBytesModeAndModTime(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("DIRC-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(src, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	if err := copyIndex(src, dst); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(dst)
	info, _ := os.Stat(dst)
	if string(got) != "DIRC-bytes" || info.Mode().Perm() != 0o600 || !info.ModTime().Equal(stamp) {
		t.Fatalf("copy = %q mode=%v mtime=%v", got, info.Mode().Perm(), info.ModTime())
	}
}

func TestCopyIndex_EachFailingStageIsReturned(t *testing.T) {
	t.Run("missing source", func(t *testing.T) {
		if err := copyIndex(filepath.Join(t.TempDir(), "absent"), filepath.Join(t.TempDir(), "dst")); err == nil {
			t.Fatal("want an open error")
		}
	})
	t.Run("stat fails", func(t *testing.T) {
		orig := copyIndexStat
		t.Cleanup(func() { copyIndexStat = orig })
		boom := errors.New("stat failed")
		copyIndexStat = func(*os.File) (os.FileInfo, error) { return nil, boom }
		src := filepath.Join(t.TempDir(), "src")
		if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(t.TempDir(), "dst")
		if err := copyIndex(src, dst); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Errorf("nothing may be written after a failed stat: %v", err)
		}
	})
	t.Run("source is a directory so reading fails", func(t *testing.T) {
		if err := copyIndex(t.TempDir(), filepath.Join(t.TempDir(), "dst")); err == nil {
			t.Fatal("want a read error")
		}
	})
	t.Run("destination directory missing", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "src")
		if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := copyIndex(src, filepath.Join(t.TempDir(), "no", "such", "dst")); err == nil {
			t.Fatal("want a write error")
		}
	})
}
