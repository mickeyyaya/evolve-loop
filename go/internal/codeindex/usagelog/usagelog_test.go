package usagelog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

func sample(phase, command string, hits, exit int, at time.Time) Record {
	return Record{TS: at, Command: command, ArgLen: 7, Exit: exit, Hits: hits, DurationMS: 12, Phase: phase, Cycle: 3, CLI: "claude", Cwd: "/w"}
}

func TestAppend_WritesOneJSONLineWithEveryField(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")

	if err := Append(path, sample("scout", "callers", 4, 0, t0), 0); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	if strings.Count(string(raw), "\n") != 1 || !strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("want exactly one terminated line, got %q", raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"ts": "2026-10-17T09:00:00Z", "command": "callers", "arg_len": 7.0, "exit": 0.0, "hits": 4.0, "duration_ms": 12.0, "phase": "scout", "cycle": 3.0, "cli": "claude", "cwd": "/w"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("fields = %v", got)
	}
}

func TestAppend_ASecondCallAddsASecondLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	for _, cmd := range []string{"api", "map"} {
		if err := Append(path, sample("builder", cmd, 1, 0, t0), 0); err != nil {
			t.Fatal(err)
		}
	}

	records, skipped, err := Read(path, time.Time{})

	if err != nil || skipped != 0 || len(records) != 2 || records[0].Command != "api" || records[1].Command != "map" {
		t.Errorf("records=%v skipped=%d err=%v", records, skipped, err)
	}
}

func TestAppend_AnUnwritablePathReturnsTheError(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "plain")
	_ = os.WriteFile(blocker, nil, 0o600)
	if err := Append(filepath.Join(blocker, "sub", "log.ndjson"), sample("scout", "find", 0, 1, t0), 0); err == nil {
		t.Error("want an error when a parent of the path is a file")
	}
	dir := filepath.Join(t.TempDir(), "isdir")
	_ = os.Mkdir(dir, 0o700)
	if err := Append(dir, sample("scout", "find", 0, 1, t0), 0); err == nil {
		t.Error("want an error when the path is a directory")
	}
}

func TestAppend_ConcurrentCallsGiveWholeLines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	const writers, each = 32, 25
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				rec := sample(fmt.Sprintf("p%d", w), "find", i, 0, t0)
				rec.Cwd = "/" + strings.Repeat("x", 300)
				if err := Append(path, rec, 0); err != nil {
					t.Error(err)
				}
			}
		}(w)
	}
	wg.Wait()

	f, _ := os.Open(path)
	defer f.Close()
	lines := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("torn line %q: %v", sc.Text(), err)
		}
		lines++
	}
	if lines != writers*each {
		t.Errorf("lines = %d, want %d", lines, writers*each)
	}
}

func TestIdentity_ReadsPhaseCycleAndCLIFromTheEnvironment(t *testing.T) {
	t.Parallel()
	env := map[string]string{"EVOLVE_DISPATCH_ID": "run1/12/scout/p4242nabc", "EVOLVE_CLI": "agy"}

	phase, cycle, cli := Identity(func(k string) string { return env[k] })

	if phase != "scout" || cycle != 12 || cli != "agy" {
		t.Errorf("got %q %d %q", phase, cycle, cli)
	}
}

func TestIdentity_NoDispatchTagGivesEmptyPhase(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"", "not-a-tag"} {
		phase, cycle, cli := Identity(func(k string) string {
			if k == "EVOLVE_DISPATCH_ID" {
				return tag
			}
			return ""
		})
		if phase != "" || cycle != 0 || cli != "" {
			t.Errorf("tag %q: got %q %d %q", tag, phase, cycle, cli)
		}
	}
}

func TestRead_MissingFileIsEmptyNotAnError(t *testing.T) {
	t.Parallel()
	records, skipped, err := Read(filepath.Join(t.TempDir(), "none"), time.Time{})
	if err != nil || len(records) != 0 || skipped != 0 {
		t.Errorf("records=%v skipped=%d err=%v", records, skipped, err)
	}
}

func TestRead_UnreadablePathReturnsTheError(t *testing.T) {
	t.Parallel()
	if _, _, err := Read(t.TempDir(), time.Time{}); err == nil {
		t.Error("want an error when the path is a directory")
	}
}

func TestRead_FiltersBySinceAndCountsMalformedLines(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	_ = Append(path, sample("scout", "find", 1, 0, t0.Add(-2*time.Hour)), 0)
	_ = Append(path, sample("scout", "map", 1, 0, t0), 0)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("{torn\n\n")
	_ = f.Close()

	records, skipped, err := Read(path, t0.Add(-time.Hour))

	if err != nil || skipped != 1 || len(records) != 1 || records[0].Command != "map" {
		t.Errorf("records=%v skipped=%d err=%v", records, skipped, err)
	}
}

func TestSummarize_GroupsByPhaseAndCommandInStableOrder(t *testing.T) {
	t.Parallel()
	records := []Record{
		sample("scout", "find", 3, 0, t0),
		sample("scout", "find", 0, 1, t0),
		sample("builder", "callers", 5, 0, t0),
		sample("scout", "api", 2, 0, t0),
		sample("", "map", 1, 0, t0),
	}

	got := Summarize(records)

	want := []Row{
		{Phase: "", Command: "map", Calls: 1, Hits: 1},
		{Phase: "builder", Command: "callers", Calls: 1, Hits: 5},
		{Phase: "scout", Command: "api", Calls: 1, Hits: 2},
		{Phase: "scout", Command: "find", Calls: 2, Hits: 3, Failures: 1},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestAppend_AnUnencodableTimeReturnsTheErrorAndWritesNothing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")

	err := Append(path, sample("scout", "find", 0, 0, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)), 0)

	if _, statErr := os.Stat(path); err == nil || statErr == nil {
		t.Errorf("err=%v statErr=%v, want an error and no file", err, statErr)
	}
}

func TestRead_APathThroughAFileReturnsTheOpenError(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain")
	_ = os.WriteFile(file, nil, 0o644)

	if _, _, err := Read(filepath.Join(file, "log"), time.Time{}); err == nil {
		t.Error("want an error when a path part is a file")
	}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestAppend_CreatesTheDirectoryAs0700AndTheFileAs0600(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "cache", "evolve")
	path := filepath.Join(dir, FileName)

	if err := Append(path, sample("scout", "find", 1, 0, t0), 0); err != nil {
		t.Fatal(err)
	}

	if mode(t, dir) != 0o700 || mode(t, path) != 0o600 {
		t.Errorf("dir mode %v, file mode %v", mode(t, dir), mode(t, path))
	}
}

func TestAppend_RotatesAFileOverTheLimitToOneOldFileAndStartsANewOne(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	const limit = 200
	for i := 0; i < 4; i++ {
		if err := Append(path, sample("scout", fmt.Sprintf("c%d", i), 1, 0, t0), limit); err != nil {
			t.Fatal(err)
		}
	}
	old, _, _ := Read(path+".1", time.Time{})
	cur, _, _ := Read(path, time.Time{})
	if len(old) == 0 || len(cur) == 0 || len(old)+len(cur) > 4 {
		t.Fatalf("old=%d current=%d", len(old), len(cur))
	}
	info, _ := os.Stat(path)
	if info.Size() > limit+300 {
		t.Errorf("current file is %d bytes", info.Size())
	}
	if _, err := os.Stat(path + ".1.1"); err == nil {
		t.Error("only one old file may exist")
	}
}

func TestAppend_ALimitOfZeroNeverRotatesAndAnOldFileIsReplaced(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	for i := 0; i < 3; i++ {
		_ = Append(path, sample("scout", "find", 1, 0, t0), 0)
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Error("a limit of 0 must not rotate")
	}
	_ = os.WriteFile(path+".1", []byte("stale\n"), 0o600)
	_ = Append(path, sample("scout", "find", 1, 0, t0), 10)
	_ = Append(path, sample("scout", "map", 1, 0, t0), 10)
	raw, _ := os.ReadFile(path + ".1")
	if strings.Contains(string(raw), "stale") {
		t.Errorf("old file = %q", raw)
	}
}

func TestAppend_ARotationFailureReturnsTheError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "log.ndjson")
	_ = Append(path, sample("scout", "find", 1, 0, t0), 0)
	_ = os.Mkdir(path+".1", 0o700)
	_ = os.WriteFile(filepath.Join(path+".1", "keep"), nil, 0o600)

	if err := Append(path, sample("scout", "map", 1, 0, t0), 1); err == nil {
		t.Error("want the rename error")
	}
}

func TestMaxUserBytes_IsFiveMegabytes(t *testing.T) {
	t.Parallel()
	if MaxUserBytes != 5<<20 {
		t.Errorf("MaxUserBytes = %d", MaxUserBytes)
	}
}

func cacheIn(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func TestUserPath_IsUnderTheUserCacheDirNotSharedTmp(t *testing.T) {
	t.Parallel()
	got, err := UserPath(cacheIn("/home/u/.cache"))
	if err != nil || got != "/home/u/.cache/evolve/code-calls.ndjson" {
		t.Errorf("got %q err=%v", got, err)
	}
	if _, err := UserPath(func() (string, error) { return "", errors.New("no home") }); err == nil {
		t.Error("want the cache dir error")
	}
}

func TestTarget_InAPhaseIsTheCycleWorkspaceFileWithNoRotation(t *testing.T) {
	t.Parallel()
	env := func(k string) string {
		if k == "EVOLVE_DISPATCH_ID" {
			return "run1/12/scout/p4242nabc"
		}
		return ""
	}

	path, limit, err := Target(env, "/proj", cacheIn("/cache"))

	if err != nil || path != "/proj/.evolve/runs/cycle-12/code-calls.ndjson" || limit != 0 {
		t.Errorf("path=%q limit=%d err=%v", path, limit, err)
	}
}

func TestTarget_OutsideAPhaseIsThePerUserFileWithTheRotationLimit(t *testing.T) {
	t.Parallel()
	path, limit, err := Target(func(string) string { return "" }, "/proj", cacheIn("/cache"))

	if err != nil || path != "/cache/evolve/code-calls.ndjson" || limit != MaxUserBytes {
		t.Errorf("path=%q limit=%d err=%v", path, limit, err)
	}
	if _, _, err := Target(func(string) string { return "" }, "/proj", func() (string, error) { return "", errors.New("no home") }); err == nil {
		t.Error("want the cache dir error")
	}
}

func TestReadAll_MergesTheUserFileItsOldFileAndEveryCycleWorkspace(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	user, _ := UserPath(cacheIn(cache))
	_ = Append(user, sample("", "map", 1, 0, t0), 0)
	_ = os.Rename(user, user+".1")
	_ = Append(user, sample("", "find", 1, 0, t0), 0)
	_ = Append(filepath.Join(root, ".evolve", "runs", "cycle-3", FileName), sample("scout", "api", 1, 0, t0), 0)
	_ = Append(filepath.Join(root, ".evolve", "runs", "cycle-4", FileName), sample("builder", "callers", 1, 0, t0.Add(-5*time.Hour)), 0)
	_ = os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-5"), 0o700)
	_ = os.MkdirAll(filepath.Join(root, ".evolve", "runs", "other"), 0o700)
	_ = os.WriteFile(filepath.Join(root, ".evolve", "runs", "cycle-9.txt"), nil, 0o600)

	all, skipped, err := ReadAll(user, root, time.Time{})
	recent, _, _ := ReadAll(user, root, t0.Add(-time.Hour))

	cmds := []string{}
	for _, r := range all {
		cmds = append(cmds, r.Command)
	}
	if err != nil || skipped != 0 || strings.Join(cmds, ",") != "map,find,api,callers" || len(recent) != 3 {
		t.Errorf("cmds=%v recent=%d skipped=%d err=%v", cmds, len(recent), skipped, err)
	}
}

func TestReadAll_CountsMalformedLinesOfEveryFileAndReturnsReadErrors(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	user, _ := UserPath(cacheIn(cache))
	_ = Append(user, sample("", "map", 1, 0, t0), 0)
	f, _ := os.OpenFile(user, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("{torn\n")
	_ = f.Close()
	if _, skipped, err := ReadAll(user, root, time.Time{}); err != nil || skipped != 1 {
		t.Errorf("skipped=%d err=%v", skipped, err)
	}
	if _, _, err := ReadAll(cache, root, time.Time{}); err == nil {
		t.Error("want an error when the user path is a directory")
	}
	_ = os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o600)
	if _, _, err := ReadAll(user, root, time.Time{}); err == nil {
		t.Error("want an error when the runs dir cannot be listed")
	}
}
