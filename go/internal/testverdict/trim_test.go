package testverdict

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

const seededFiles = 4

func seededStore(t *testing.T) (root string, files []string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "testcache")
	store := NewStore(root, func() time.Time { return epoch })
	for _, code := range []string{"a", "b", "c"} {
		if err := store.Put(recordFor(t, code, nil)); err != nil {
			t.Fatal(err)
		}
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return root, files
}

func ageAll(t *testing.T, files []string, ages ...time.Duration) {
	t.Helper()
	for i, f := range files {
		mtime := epoch.Add(-ages[i%len(ages)])
		if err := os.Chtimes(f, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestTrim_RemovesEveryFilePastTheAgeAndTheEmptyKeyDirectories(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	if len(files) != seededFiles {
		t.Fatalf("seed: %d files", len(files))
	}
	ageAll(t, files, 40*24*time.Hour)

	var rep TrimReport = Trim(root, epoch, 14*24*time.Hour, 0, os.Remove)

	if rep.Files != len(files) || rep.Bytes <= 0 || len(rep.Errors) != 0 {
		t.Fatalf("every aged file goes; got %+v", rep)
	}
	left, _ := os.ReadDir(root)
	if len(left) != 0 {
		t.Fatalf("empty key directories must go too; left %v", left)
	}
}

func TestTrim_KeepsFreshFilesWhenNoCapIsSet(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, time.Hour)
	rep := Trim(root, epoch, 14*24*time.Hour, 0, os.Remove)
	if rep.Files != 0 {
		t.Fatalf("fresh files stay; got %+v", rep)
	}
	for _, f := range files {
		if !exists(f) {
			t.Fatalf("%s was removed", f)
		}
	}
}

func TestTrim_TheSizeCapRemovesTheOldestFirstUntilTheRestFits(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour)
	var total int64
	for _, f := range files {
		info, _ := os.Stat(f)
		total += info.Size()
	}

	rep := Trim(root, epoch, 14*24*time.Hour, total-1, os.Remove)

	if rep.Files != 1 || exists(files[3]) {
		t.Fatalf("one byte over the cap removes only the oldest file; got %+v", rep)
	}
	for _, f := range files[:3] {
		if !exists(f) {
			t.Fatalf("%s is newer and must stay", f)
		}
	}
}

func TestTrim_ADryRunRemoverCountsTheSamePlanAndLeavesEveryFile(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, 40*24*time.Hour)
	real := Trim(root, epoch, 14*24*time.Hour, 0, func(string) error { return nil })
	if real.Files != len(files) {
		t.Fatalf("the preview counts every aged file; got %+v", real)
	}
	for _, f := range files {
		if !exists(f) {
			t.Fatalf("a no-op remover must leave %s", f)
		}
	}
}

func TestTrim_ARemovalThatFailsIsReportedAndNotCounted(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, 40*24*time.Hour)
	boom := errors.New("read-only filesystem")
	rep := Trim(root, epoch, 14*24*time.Hour, 0, func(string) error { return boom })
	if rep.Files != 0 || len(rep.Errors) != len(files) {
		t.Fatalf("each failed removal is one error and no count; got %+v", rep)
	}
}

func TestTrim_AFileThatIsAlreadyGoneIsNotAnError(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, 40*24*time.Hour)
	rep := Trim(root, epoch, 14*24*time.Hour, 0, func(string) error { return fs.ErrNotExist })
	if len(rep.Errors) != 0 || rep.Files != len(files) {
		t.Fatalf("a concurrent removal is fine; got %+v", rep)
	}
}

func TestTrim_ADirectoryThatCannotBeRemovedIsReported(t *testing.T) {
	t.Parallel()
	root, files := seededStore(t)
	ageAll(t, files, 40*24*time.Hour)
	boom := errors.New("busy")
	rep := Trim(root, epoch, 14*24*time.Hour, 0, func(p string) error {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return boom
		}
		return os.Remove(p)
	})
	if len(rep.Errors) == 0 {
		t.Fatalf("a directory that stays is reported; got %+v", rep)
	}
}

func TestTrim_AMissingOrUnreadableRootIsHandledWithoutAPanic(t *testing.T) {
	t.Parallel()
	if rep := Trim(filepath.Join(t.TempDir(), "absent"), epoch, time.Hour, 1, os.Remove); rep.Files != 0 || len(rep.Errors) != 0 {
		t.Fatalf("a store that was never written has nothing to trim; got %+v", rep)
	}
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rep := Trim(notDir, epoch, time.Hour, 0, os.Remove); rep.Files != 0 || len(rep.Errors) != 0 {
		t.Fatalf("a fresh file as the root is listed and kept; got %+v", rep)
	}
}

func TestTrim_AnUnreadableStoreDirectoryIsReportedAndNothingIsRemoved(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	root, files := seededStore(t)
	ageAll(t, files, 40*24*time.Hour)
	locked := filepath.Dir(files[0])
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	rep := Trim(root, epoch, 14*24*time.Hour, 0, os.Remove)

	if rep.Files != 0 || len(rep.Errors) != 1 {
		t.Fatalf("a listing that fails removes nothing and says why; got %+v", rep)
	}
}
