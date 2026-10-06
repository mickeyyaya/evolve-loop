package gc

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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

func ageOnly(now time.Time) GoCacheBounds {
	return GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour}
}

func TestTrimGoCache_RemovesOnlyEntriesUnusedSinceTheCutoff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 23, 0, 0, 0, time.UTC)
	dir, want := goCacheFixture(t, now)

	rep := TrimGoCache(dir, ageOnly(now), os.Remove)

	if wantRep := (CacheTrimReport{Files: len(want), Bytes: int64(10 * len(want)), RemainingBytes: 10}); !reflect.DeepEqual(rep, wantRep) {
		t.Fatalf("report = %+v, want %+v", rep, wantRep)
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

	rep := TrimGoCache(dir, ageOnly(now), func(string) error { return nil })

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
	rep := TrimGoCache(filepath.Join(t.TempDir(), "absent"), GoCacheBounds{Now: time.Now(), UnusedFor: time.Hour, MaxBytes: 1}, os.Remove)

	if len(rep.Errors) != 1 {
		t.Errorf("Errors = %v, want the unreadable cache dir named", rep.Errors)
	}
}

type cacheFile struct {
	rel  string
	size int
	age  time.Duration
}

func capFixture(t *testing.T, now time.Time, files []cacheFile) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f.rel))
		writeFile(t, p, strings.Repeat("x", f.size))
		mtime := now.Add(-f.age)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func survivors(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func TestTrimGoCache_TheCapTakesTheLeastRecentlyUsedEntriesUntilTheCacheFits(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 400, 20 * time.Hour},
		{"0b/0b01-a", 100, 19 * time.Hour},
		{"0c/0c01-d", 300, 10 * time.Hour},
		{"0d/0d01-d", 200, 5 * time.Hour},
		{"0e/0e01-a", 100, 3 * time.Hour},
		{"0f/0f01-d", 250, 30 * time.Minute},
		{"trim.txt", 900, 40 * time.Hour},
		{"fuzz/0a01-d", 900, 40 * time.Hour},
	})

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 600}, os.Remove)

	want := CacheTrimReport{CapFiles: 3, CapBytes: 800, RemainingBytes: 550}
	if !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v (the three oldest entries go, oldest first, until 1350 bytes fit under 600)", rep, want)
	}
	if got, keep := survivors(t, dir), []string{"0d/0d01-d", "0e/0e01-a", "0f/0f01-d", "fuzz/0a01-d", "trim.txt"}; !reflect.DeepEqual(got, keep) {
		t.Errorf("survivors = %v, want %v", got, keep)
	}
}

func TestTrimGoCache_ACacheAlreadyUnderTheCapIsLeftUntouched(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	files := []cacheFile{
		{"0a/0a01-d", 400, 20 * time.Hour},
		{"0b/0b01-a", 100, 19 * time.Hour},
		{"0c/0c01-d", 300, 5 * time.Minute},
	}
	dir := capFixture(t, now, files)
	before := survivors(t, dir)

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 800}, os.Remove)

	if want := (CacheTrimReport{RemainingBytes: 800}); !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v: a cache exactly at its cap needs no trim", rep, want)
	}
	if got := survivors(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("an under-cap cache lost entries: %v -> %v", before, got)
	}
	for _, f := range files {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(now.Add(-f.age)) {
			t.Errorf("%s mtime changed to %v: an under-cap cache must not be touched", f.rel, info.ModTime())
		}
	}
}

func TestTrimGoCache_TheCapNeverTakesAnEntryARunningBuildMayStillRead(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 500, GoCacheInUseWindow + time.Minute},
		{"0b/0b01-d", 500, GoCacheInUseWindow - time.Minute},
		{"0c/0c01-a", 500, 10 * time.Minute},
	})

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 100}, os.Remove)

	if want := (CacheTrimReport{CapFiles: 1, CapBytes: 500, RemainingBytes: 1000}); !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v: only the entry unused for longer than %s may go, so the cache stays over its cap", rep, want, GoCacheInUseWindow)
	}
	if got, keep := survivors(t, dir), []string{"0b/0b01-d", "0c/0c01-a"}; !reflect.DeepEqual(got, keep) {
		t.Errorf("survivors = %v, want %v", got, keep)
	}
}

func TestTrimGoCache_TheCapCountsOnlyWhatTheAgeTrimLeavesSoAPreviewNeverCountsAnEntryTwice(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 400, 30 * time.Hour},
		{"0b/0b01-a", 100, 26 * time.Hour},
		{"0c/0c01-d", 300, 10 * time.Hour},
		{"0d/0d01-d", 200, 5 * time.Hour},
		{"0e/0e01-a", 100, 1 * time.Hour},
	})
	bounds := GoCacheBounds{Now: now, UnusedFor: 24 * time.Hour, MaxBytes: 350}
	want := CacheTrimReport{Files: 2, Bytes: 500, CapFiles: 1, CapBytes: 300, RemainingBytes: 300}

	preview := TrimGoCache(dir, bounds, func(string) error { return nil })
	if !reflect.DeepEqual(preview, want) {
		t.Fatalf("preview = %+v, want %+v", preview, want)
	}
	if got := len(survivors(t, dir)); got != 5 {
		t.Fatalf("the preview removed entries: %d left, want 5", got)
	}

	if rep := TrimGoCache(dir, bounds, os.Remove); !reflect.DeepEqual(rep, want) {
		t.Fatalf("real run = %+v, want the preview's %+v", rep, want)
	}
	if got, keep := survivors(t, dir), []string{"0d/0d01-d", "0e/0e01-a"}; !reflect.DeepEqual(got, keep) {
		t.Errorf("survivors = %v, want %v", got, keep)
	}
}

func TestTrimGoCache_AnEntryThatCannotBeRemovedStaysCountedInWhatRemains(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 400, 20 * time.Hour},
		{"0b/0b01-d", 400, 19 * time.Hour},
	})
	stuck := filepath.Join(dir, "0a", "0a01-d")
	remove := func(p string) error {
		if p == stuck {
			return os.ErrPermission
		}
		return os.Remove(p)
	}

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 100}, remove)

	if rep.CapFiles != 1 || rep.CapBytes != 400 || rep.RemainingBytes != 400 || len(rep.Errors) != 1 {
		t.Errorf("report = %+v, want one trimmed entry, the stuck one's 400 bytes still remaining, and its error named", rep)
	}
}

func TestTrimGoCache_AnEntryAnotherTrimAlreadyRemovedCountsAsReleased(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{{"0a/0a01-d", 400, 20 * time.Hour}})
	gone := func(p string) error {
		if err := os.Remove(p); err != nil {
			return err
		}
		return os.Remove(p)
	}

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 100}, gone)

	if want := (CacheTrimReport{CapFiles: 1, CapBytes: 400}); !reflect.DeepEqual(rep, want) {
		t.Errorf("report = %+v, want %+v", rep, want)
	}
}

func TestTrimGoCache_AnUnreadableShardIsNamedAndTheOthersAreStillTrimmed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{{"0a/0a01-d", 400, 30 * time.Hour}})
	shard := filepath.Join(dir, "0b")
	if err := os.Mkdir(shard, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(shard, 0o755) })
	if _, err := os.ReadDir(shard); err == nil {
		t.Skip("a 0o000 directory is readable here (running as root)")
	}

	rep := TrimGoCache(dir, ageOnly(now), os.Remove)

	if rep.Files != 1 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "0b") {
		t.Errorf("report = %+v, want the readable shard trimmed and the unreadable one named", rep)
	}
}

func touchingNextVictim(t *testing.T, now time.Time, victim, next string) func(string) error {
	t.Helper()
	return func(p string) error {
		if p == victim {
			if err := os.Chtimes(next, now, now); err != nil {
				t.Fatal(err)
			}
		}
		return os.Remove(p)
	}
}

func TestTrimGoCache_TheCapSkipsAVictimABuildUsedAfterThePlan(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 400, 20 * time.Hour},
		{"0b/0b01-d", 400, 19 * time.Hour},
		{"0c/0c01-d", 400, 10 * time.Hour},
		{"0d/0d01-d", 100, 30 * time.Minute},
	})
	remove := touchingNextVictim(t, now, filepath.Join(dir, "0a", "0a01-d"), filepath.Join(dir, "0b", "0b01-d"))

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, MaxBytes: 100}, remove)

	if want := (CacheTrimReport{CapFiles: 2, CapBytes: 800, RemainingBytes: 500}); !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v: the entry a build looked up after the plan must be skipped and still count as remaining", rep, want)
	}
	if got, keep := survivors(t, dir), []string{"0b/0b01-d", "0d/0d01-d"}; !reflect.DeepEqual(got, keep) {
		t.Errorf("survivors = %v, want %v", got, keep)
	}
}

func TestTrimGoCache_TheAgeTrimSkipsAVictimABuildUsedAfterThePlan(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 400, 40 * time.Hour},
		{"0b/0b01-d", 400, 30 * time.Hour},
	})
	remove := touchingNextVictim(t, now, filepath.Join(dir, "0a", "0a01-d"), filepath.Join(dir, "0b", "0b01-d"))

	rep := TrimGoCache(dir, ageOnly(now), remove)

	if want := (CacheTrimReport{Files: 1, Bytes: 400, RemainingBytes: 400}); !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v", rep, want)
	}
	if got := survivors(t, dir); !reflect.DeepEqual(got, []string{"0b/0b01-d"}) {
		t.Errorf("survivors = %v, want the re-used entry kept", got)
	}
}

func TestGoCacheInUseWindow_IsTwiceTheGoCommandsMtimeRefreshInterval(t *testing.T) {
	t.Parallel()
	if GoCacheInUseWindow != 2*goMtimeRefresh || goMtimeRefresh != time.Hour {
		t.Errorf("GoCacheInUseWindow = %s, goMtimeRefresh = %s: the window is the go command's one-hour mtime refresh plus a one-hour hold", GoCacheInUseWindow, goMtimeRefresh)
	}
}

func TestLeastRecentlyUsedOverCap_NeverPlansAnEntryInsideTheInUseWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	entries := []goCacheEntry{
		{shard: "0a", name: "0a01-d", size: 500, mtime: now.Add(-GoCacheInUseWindow - time.Minute)},
		{shard: "0b", name: "0b01-d", size: 500, mtime: now.Add(-GoCacheInUseWindow + time.Minute)},
		{shard: "0c", name: "0c01-d", size: 500, mtime: now.Add(-time.Minute)},
	}

	victims := leastRecentlyUsedOverCap(entries, GoCacheBounds{Now: now, MaxBytes: 100})

	if len(victims) != 1 || victims[0].name != "0a01-d" {
		t.Errorf("planned %v, want only 0a01-d: the plan itself never takes an entry inside the %s window, so the removal-time re-check is a second guard, not the only one", victims, GoCacheInUseWindow)
	}
}

func TestTrimGoCache_AnAgeHorizonShorterThanTheInUseWindowNeverTakesAnEntryInsideIt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	dir := capFixture(t, now, []cacheFile{
		{"0a/0a01-d", 100, 3 * time.Hour},
		{"0b/0b01-d", 100, 90 * time.Minute},
	})

	rep := TrimGoCache(dir, GoCacheBounds{Now: now, UnusedFor: time.Hour}, os.Remove)

	if want := (CacheTrimReport{Files: 1, Bytes: 100, RemainingBytes: 100}); !reflect.DeepEqual(rep, want) {
		t.Fatalf("report = %+v, want %+v: a 1h age horizon must still keep an entry whose mtime is inside the %s in-use window", rep, want, GoCacheInUseWindow)
	}
	if got := survivors(t, dir); !reflect.DeepEqual(got, []string{"0b/0b01-d"}) {
		t.Errorf("survivors = %v, want the 90-minute-old entry kept", got)
	}
}
