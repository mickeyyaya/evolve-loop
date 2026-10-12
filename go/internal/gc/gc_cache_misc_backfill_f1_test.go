package gc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireNonRootF1(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-denied paths cannot be produced as root")
	}
}

func lockedDirF1(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestApplyBackfillF1_ADeleteThatTheFilesystemRefusesIsReportedAndTheRestStillRuns(t *testing.T) {
	requireNonRootF1(t)
	evolve := t.TempDir()
	stuckParent := filepath.Join(evolve, "runs", "stuck")
	writeFile(t, filepath.Join(stuckParent, "cycle-1", "payload"), "x")
	free := filepath.Join(evolve, "runs", "cycle-2")
	writeFile(t, filepath.Join(free, "payload"), "x")
	lockedDirF1(t, stuckParent, 0o555)

	err := Apply(evolve, Manifest{Items: []Item{
		{Path: filepath.Join(stuckParent, "cycle-1"), Action: ActionDelete, Rule: "runs.delete_after_days"},
		{Path: free, Action: ActionDelete, Rule: "runs.delete_after_days"},
	}})

	if err == nil || !strings.Contains(err.Error(), "gc: delete "+filepath.Join(stuckParent, "cycle-1")) {
		t.Fatalf("err = %v, want the failed delete named", err)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want it to wrap the permission failure", err)
	}
	if _, statErr := os.Stat(free); !os.IsNotExist(statErr) {
		t.Fatalf("one refused delete must not stop the next item: %v", statErr)
	}
	if info, statErr := os.Stat(filepath.Join(stuckParent, "cycle-1")); statErr != nil || !info.IsDir() {
		t.Fatalf("the directory the filesystem refused to remove must remain: %v", statErr)
	}
}

func shardWithOneEntryF1(t *testing.T) (dir, shard string) {
	t.Helper()
	dir = t.TempDir()
	shard = filepath.Join(dir, "ab")
	writeFile(t, filepath.Join(shard, "ab01-d"), "0123456789")
	return dir, shard
}

func TestListShardBackfillF1_AnEntryThatVanishesBetweenListingAndStatIsSkippedSilently(t *testing.T) {
	_, shard := shardWithOneEntryF1(t)
	orig := goCacheEntryInfo
	t.Cleanup(func() { goCacheEntryInfo = orig })
	goCacheEntryInfo = func(fs.DirEntry) (fs.FileInfo, error) { return nil, fs.ErrNotExist }

	entries, errs := listShard(shard)

	if len(entries) != 0 || len(errs) != 0 {
		t.Fatalf("entries=%v errs=%v; a vanished entry is neither listed nor an error", entries, errs)
	}
}

func TestListShardBackfillF1_AnEntryWhoseStatFailsIsReportedByPathAndSkipped(t *testing.T) {
	_, shard := shardWithOneEntryF1(t)
	writeFile(t, filepath.Join(shard, "ab02-a"), "0123456789")
	orig := goCacheEntryInfo
	t.Cleanup(func() { goCacheEntryInfo = orig })
	boom := errors.New("input/output error")
	goCacheEntryInfo = func(e fs.DirEntry) (fs.FileInfo, error) {
		if e.Name() == "ab01-d" {
			return nil, boom
		}
		return e.Info()
	}

	entries, errs := listShard(shard)

	if len(entries) != 1 || entries[0].name != "ab02-a" {
		t.Fatalf("entries = %+v, want only the readable one", entries)
	}
	if len(errs) != 1 || errs[0] != "stat "+filepath.Join(shard, "ab01-d")+": input/output error" {
		t.Fatalf("errs = %q", errs)
	}
}

func TestListShardBackfillF1_ARealUnreadableShardReportsEveryEntryItCannotStat(t *testing.T) {
	requireNonRootF1(t)
	_, shard := shardWithOneEntryF1(t)
	lockedDirF1(t, shard, 0o444)

	entries, errs := listShard(shard)

	if len(entries) != 0 || len(errs) != 1 || !strings.HasPrefix(errs[0], "stat "+filepath.Join(shard, "ab01-d")+":") {
		t.Fatalf("entries=%v errs=%q; a listable-but-not-searchable shard must surface its stat failure", entries, errs)
	}
}

func TestReleaseBackfillF1_ALstatFailureOtherThanAbsenceIsRecordedAndNothingIsRemoved(t *testing.T) {
	requireNonRootF1(t)
	_, shard := shardWithOneEntryF1(t)
	victim := goCacheEntry{shard: shard, name: "ab01-d", size: 10, mtime: time.Unix(1, 0)}
	lockedDirF1(t, shard, 0o444)
	removed := 0
	rep := CacheTrimReport{}

	files, bytes := rep.release([]goCacheEntry{victim}, time.Unix(1<<31, 0), func(string) error { removed++; return nil })

	if files != 0 || bytes != 0 || removed != 0 {
		t.Fatalf("files=%d bytes=%d removed=%d, want nothing released", files, bytes, removed)
	}
	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "ab01-d") {
		t.Fatalf("errors = %q, want the lstat failure naming the entry", rep.Errors)
	}
}

func TestReleaseBackfillF1_RemoveOutcomes(t *testing.T) {
	cases := []struct {
		name       string
		removeErr  error
		wantFiles  int
		wantErrors int
	}{
		{"removed", nil, 1, 0},
		{"already gone counts as released", fs.ErrNotExist, 1, 0},
		{"a refused removal is recorded and not counted", errors.New("busy"), 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, shard := shardWithOneEntryF1(t)
			victim := goCacheEntry{shard: shard, name: "ab01-d", size: 10, mtime: time.Unix(1, 0)}
			rep := CacheTrimReport{}
			files, bytes := rep.release([]goCacheEntry{victim}, time.Unix(1<<31, 0), func(string) error { return tc.removeErr })
			if files != tc.wantFiles || bytes != int64(10*tc.wantFiles) || len(rep.Errors) != tc.wantErrors {
				t.Fatalf("files=%d bytes=%d errors=%q", files, bytes, rep.Errors)
			}
		})
	}
}

func TestParseCwdListingBackfillF1_BlankLinesAndUnknownFieldsAreIgnored(t *testing.T) {
	t.Parallel()
	listing := "p100\nR1\nn/work/a\n\np200\n\nR100\nn/work/b\nfcwd\np300\nR1\n\n"
	got := parseCwdListing(listing)
	want := []Process{{Pid: 100, Ppid: 1, Cwd: "/work/a"}, {Pid: 200, Ppid: 100, Cwd: "/work/b"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v (pid 300 has no cwd and must be dropped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("process %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(parseCwdListing("")) != 0 || len(parseCwdListing("\n\n")) != 0 {
		t.Error("empty input has no processes")
	}
}

func TestReapPipelineTempBackfillF1_ADirectoryThatCannotBeReadIsReportedNotFatal(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "absent")
	rep := ReapPipelineTemp(missing, time.Now(), func(string) error { t.Error("nothing may be removed"); return nil })
	if rep.Entries != 0 || rep.Bytes != 0 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "read temp dir "+missing) {
		t.Fatalf("report = %+v", rep)
	}
}

func TestReapPipelineTempBackfillF1_ARefusedRemovalIsReportedAndDoesNotCountOrStopTheSweep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"go-build111", "go-build222"} {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "payload"), "0123456789")
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	rep := ReapPipelineTemp(dir, time.Now().Add(-time.Hour), func(p string) error {
		if strings.HasSuffix(p, "go-build111") {
			return errors.New("device busy")
		}
		return os.RemoveAll(p)
	})
	if rep.Entries != 1 || rep.Bytes != 10 {
		t.Fatalf("report = %+v, want only the removed tree counted", rep)
	}
	if len(rep.Errors) != 1 || rep.Errors[0] != "device busy" {
		t.Fatalf("errors = %q", rep.Errors)
	}
	if _, err := os.Stat(filepath.Join(dir, "go-build111")); err != nil {
		t.Fatalf("the refused tree must still exist: %v", err)
	}
}
