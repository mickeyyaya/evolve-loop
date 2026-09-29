package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func goCacheFixture(t *testing.T, now time.Time) (string, map[string]bool) {
	t.Helper()
	dir := t.TempDir()
	old, fresh := now.Add(-30*time.Hour), now.Add(-2*time.Hour)
	files := map[string]time.Time{
		"00/0a1b-a":       old,
		"00/0a1b-d":       old,
		"ab/ab77-d":       fresh,
		"ff/ff01-a":       old,
		"trim.txt":        old,
		"README":          old,
		"testexpire.txt":  old,
		"tmp/scratch-d":   old,
		"0g/notahash-d":   old,
		"ab/ab77.partial": old,
		"fuzz/0a1b-a":     old,
	}
	want := map[string]bool{"00/0a1b-a": true, "00/0a1b-d": true, "ff/ff01-a": true}
	for name, mtime := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		writeFile(t, p, "0123456789")
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return dir, want
}

func TestTrimGoCache_RemovesOnlyEntriesUnusedSinceTheCutoff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 23, 0, 0, 0, time.UTC)
	dir, want := goCacheFixture(t, now)

	rep := TrimGoCache(dir, now.Add(-24*time.Hour), os.Remove)

	if rep.Files != len(want) || rep.Bytes != int64(10*len(want)) || len(rep.Errors) != 0 {
		t.Fatalf("report = %+v, want %d files / %d bytes / no errors", rep, len(want), 10*len(want))
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if want[filepath.ToSlash(rel)] {
			t.Errorf("%s survived the trim", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"ab/ab77-d", "trim.txt", "README", "testexpire.txt", "tmp/scratch-d", "0g/notahash-d", "ab/ab77.partial", "fuzz/0a1b-a"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(kept))); err != nil {
			t.Errorf("%s was removed: only a hash subdirectory's -a/-d entries past the cutoff may go", kept)
		}
	}
}

func TestTrimGoCache_APreviewCountsTheSameEntriesAndRemovesNothing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 23, 0, 0, 0, time.UTC)
	dir, want := goCacheFixture(t, now)

	rep := TrimGoCache(dir, now.Add(-24*time.Hour), func(string) error { return nil })

	if rep.Files != len(want) {
		t.Errorf("preview counted %d files, want %d", rep.Files, len(want))
	}
	for name := range want {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Errorf("preview removed %s", name)
		}
	}
}

func TestTrimGoCache_AMissingCacheIsReportedNotIgnored(t *testing.T) {
	t.Parallel()
	rep := TrimGoCache(filepath.Join(t.TempDir(), "absent"), time.Now(), os.Remove)

	if len(rep.Errors) != 1 {
		t.Errorf("Errors = %v, want the unreadable cache dir named", rep.Errors)
	}
}
