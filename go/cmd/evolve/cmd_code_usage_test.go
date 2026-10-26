package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/usagelog"
)

func hitFake(req codeindex.Request, hits int) codeindex.Backend {
	list := make([]codeindex.Hit, hits)
	for i := range list {
		list[i] = codeindex.Hit{File: "a.go", Line: i + 1, Symbol: "s"}
	}
	return codeindextest.NewFake("fake", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{req: list})
}

func codeindextestNotFound() codeindex.Backend {
	return codeindextest.NewFake("fake", codeindex.AllCapabilities(), nil)
}

func mapFake() codeindex.Backend {
	return hitFake(codeindex.Request{Query: codeindex.QueryMap}, 2)
}

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func runLogged(t *testing.T, sink codeUsageSink, build codeBackendsBuilder, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := codeVerbLogged(context.Background(), args, &stdout, &stderr, build, sink)
	return rc, stdout.String(), stderr.String()
}

func readLog(t *testing.T, path string) []usagelog.Record {
	t.Helper()
	records, skipped, err := usagelog.Read(path, time.Time{})
	if err != nil || skipped != 0 {
		t.Fatalf("read log: skipped=%d err=%v", skipped, err)
	}
	return records
}

func userSink(t *testing.T, env map[string]string) (codeUsageSink, string) {
	t.Helper()
	cache := t.TempDir()
	return codeUsageSink{env: envOf(env), cacheDir: func() (string, error) { return cache, nil }, rootOf: func() string { return "" }}, filepath.Join(cache, "evolve", usagelog.FileName)
}

func TestCodeVerbLogged_InAPhaseTheRecordGoesToTheCycleWorkspaceFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sink, userPath := userSink(t, map[string]string{"EVOLVE_DISPATCH_ID": "r/7/scout/p4242nab", "EVOLVE_CLI": "agy"})
	path := filepath.Join(root, ".evolve", "runs", "cycle-7", "code-calls.ndjson")

	rc, stdout, stderr := runLogged(t, sink, codeIndexOf(hitFake(codeindex.Request{Query: codeindex.QueryFind, Arg: "alpha beta"}, 3)), "find", "alpha", "beta", "--project-root", root)

	records := readLog(t, path)
	if rc != 0 || stderr != "" || len(records) != 1 {
		t.Fatalf("rc=%d stderr=%q records=%v", rc, stderr, records)
	}
	cwd, _ := os.Getwd()
	got := records[0]
	if got.Command != "find" || got.Phase != "scout" || got.Cycle != 7 || got.CLI != "agy" || got.Cwd != cwd ||
		got.ArgLen != len("alpha beta") || got.Exit != 0 || got.Hits != 3 || strings.Count(stdout, "\n") != 3 || got.DurationMS < 0 || got.TS.IsZero() {
		t.Errorf("record = %+v, stdout=%q", got, stdout)
	}
	if _, err := os.Stat(userPath); err == nil {
		t.Error("a phase call must not write the per-user file")
	}
}

func TestCodeVerbLogged_OutsideAPhaseTheRecordGoesToThePerUserFileAs0600(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)

	rc, _, _ := runLogged(t, sink, codeIndexOf(mapFake()), "map")

	records := readLog(t, path)
	info, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if rc != 0 || len(records) != 1 || records[0].Phase != "" || info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
		t.Errorf("rc=%d records=%v file=%v dir=%v", rc, records, info.Mode(), dir.Mode())
	}
}

func TestCodeVerbLogged_TheRecordNeverHoldsTheRawArgument(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)

	runLogged(t, sink, codeIndexOf(codeindextestNotFound()), "find", "secretToken")

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "secretToken") || len(raw) == 0 {
		t.Errorf("log = %q", raw)
	}
}

func TestCodeVerbLogged_ANotFoundExitIsLoggedWithItsExitCode(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)

	rc, _, _ := runLogged(t, sink, codeIndexOf(codeindextestNotFound()), "api", "nofile.go")

	records := readLog(t, path)
	if rc != 1 || len(records) != 1 || records[0].Exit != 1 || records[0].Hits != 0 || records[0].Command != "api" {
		t.Errorf("rc=%d records=%v", rc, records)
	}
}

func TestCodeVerbLogged_ABackendBuildFailureIsLogged(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)
	build := func(string) (codeIndexHandle, error) { return codeIndexHandle{}, errors.New("no policy") }

	rc, _, _ := runLogged(t, sink, build, "map")

	records := readLog(t, path)
	if rc != codeExitFailed || len(records) != 1 || records[0].Exit != codeExitFailed {
		t.Errorf("rc=%d records=%v", rc, records)
	}
}

func TestCodeVerbLogged_AUsageErrorWithAKnownVerbIsLoggedUnderTheDefaultRoot(t *testing.T) {
	t.Parallel()
	sink, _ := userSink(t, map[string]string{"EVOLVE_DISPATCH_ID": "r/2/scout/p4242nab"})
	root := t.TempDir()
	sink.rootOf = func() string { return root }
	path := filepath.Join(root, ".evolve", "runs", "cycle-2", "code-calls.ndjson")

	rc, _, _ := runLogged(t, sink, codeIndexOf(mapFake()), "callers")

	records := readLog(t, path)
	if rc != codeExitUsage || len(records) != 1 || records[0].Exit != codeExitUsage || records[0].Command != "callers" {
		t.Errorf("rc=%d records=%v", rc, records)
	}
}

func TestCodeVerbLogged_AnUnknownVerbOrNoVerbWritesNoRecord(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)

	rcBad, _, _ := runLogged(t, sink, codeIndexOf(mapFake()), "frobnicate", "x")
	rcNone, _, _ := runLogged(t, sink, codeIndexOf(mapFake()))

	if _, err := os.Stat(path); rcBad != codeExitUsage || rcNone != codeExitUsage || err == nil {
		t.Errorf("rcBad=%d rcNone=%d statErr=%v", rcBad, rcNone, err)
	}
}

func brokenSinks(t *testing.T) map[string]codeUsageSink {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "plain")
	_ = os.WriteFile(blocker, nil, 0o600)
	return map[string]codeUsageSink{
		"no cache dir":    {env: envOf(nil), cacheDir: func() (string, error) { return "", errors.New("no home") }, rootOf: func() string { return "" }},
		"unwritable path": {env: envOf(nil), cacheDir: func() (string, error) { return blocker, nil }, rootOf: func() string { return "" }},
	}
}

func TestCodeVerbLogged_AWriteFailureKeepsTheExitCodeAndTheOutput(t *testing.T) {
	t.Parallel()
	wantRC, wantOut, _ := runCode(t, codeIndexOf(mapFake()), "map")
	for name, sink := range brokenSinks(t) {
		rc, stdout, stderr := runLogged(t, sink, codeIndexOf(mapFake()), "map")

		lines := strings.Split(strings.TrimSpace(stderr), "\n")
		if rc != wantRC || stdout != wantOut || len(lines) != 1 || !strings.HasPrefix(lines[0], "evolve code: usage log: ") {
			t.Errorf("%s: rc=%d (want %d) stdout=%q stderr=%q", name, rc, wantRC, stdout, stderr)
		}
	}
}

func TestCodeVerbLogged_AWriteFailureKeepsANonZeroExitCode(t *testing.T) {
	t.Parallel()
	sink := brokenSinks(t)["unwritable path"]

	rc, _, stderr := runLogged(t, sink, codeIndexOf(codeindextestNotFound()), "api", "x.go")

	if rc != 1 || !strings.Contains(stderr, "usage log") || !strings.Contains(stderr, "no result") {
		t.Errorf("rc=%d stderr=%q", rc, stderr)
	}
}

func TestCodeVerbLogged_ConcurrentCallsLeaveWholeLines(t *testing.T) {
	t.Parallel()
	sink, path := userSink(t, nil)
	const calls = 40
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runLogged(t, sink, codeIndexOf(mapFake()), "map")
		}()
	}
	wg.Wait()

	if got := len(readLog(t, path)); got != calls {
		t.Errorf("records = %d, want %d", got, calls)
	}
}

type codeUsageFixture struct {
	cacheDir func() (string, error)
	root     string
}

func seedUsageLog(t *testing.T) codeUsageFixture {
	t.Helper()
	cache, root := t.TempDir(), t.TempDir()
	fixture := codeUsageFixture{cacheDir: func() (string, error) { return cache, nil }, root: root}
	userPath, _ := usagelog.UserPath(fixture.cacheDir)
	workspacePath := filepath.Join(root, ".evolve", "runs", "cycle-3", usagelog.FileName)
	now := time.Now()
	for path, rec := range map[string][]usagelog.Record{
		userPath: {
			{TS: now.Add(-48 * time.Hour), Command: "find", Phase: "scout", Hits: 9},
			{TS: now, Command: "find", Phase: "scout", Hits: 2},
		},
		workspacePath: {
			{TS: now, Command: "find", Phase: "scout", Hits: 0, Exit: 1},
			{TS: now, Command: "callers", Phase: "builder", Hits: 4},
		},
	} {
		for _, r := range rec {
			if err := usagelog.Append(path, r, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	return fixture
}

func runUsage(f codeUsageFixture, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	rc := codeUsageVerb(args, &stdout, &stderr, f.cacheDir, f.root)
	return rc, stdout.String(), stderr.String()
}

func TestCodeUsage_MergesThePerUserFileAndEveryCycleWorkspaceByPhaseAndCommand(t *testing.T) {
	t.Parallel()
	rc, stdout, stderr := runUsage(seedUsageLog(t))

	want := "PHASE\tCOMMAND\tCALLS\tHITS\tFAILURES\nbuilder\tcallers\t1\t4\t0\nscout\tfind\t3\t11\t1\n"
	if rc != 0 || stdout != want || stderr != "" {
		t.Errorf("rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
	}
}

func TestCodeUsage_SinceDropsOlderRecords(t *testing.T) {
	t.Parallel()
	rc, stdout, _ := runUsage(seedUsageLog(t), "--since", "24h")

	want := "PHASE\tCOMMAND\tCALLS\tHITS\tFAILURES\nbuilder\tcallers\t1\t4\t0\nscout\tfind\t2\t2\t1\n"
	if rc != 0 || stdout != want {
		t.Errorf("rc=%d stdout=%q", rc, stdout)
	}
}

func TestCodeUsage_JSONListsTheRows(t *testing.T) {
	t.Parallel()
	rc, stdout, _ := runUsage(seedUsageLog(t), "--json", "--since", "24h")

	var rows []usagelog.Row
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil || rc != 0 || len(rows) != 2 || rows[1] != (usagelog.Row{Phase: "scout", Command: "find", Calls: 2, Hits: 2, Failures: 1}) {
		t.Errorf("rc=%d rows=%v err=%v stdout=%q", rc, rows, err, stdout)
	}
}

func emptyFixture(t *testing.T) codeUsageFixture {
	t.Helper()
	cache := t.TempDir()
	return codeUsageFixture{cacheDir: func() (string, error) { return cache, nil }, root: t.TempDir()}
}

func TestCodeUsage_AMissingLogGivesAnEmptyJSONListAndAHeaderOnlyTable(t *testing.T) {
	t.Parallel()
	f := emptyFixture(t)

	_, jsonOut, _ := runUsage(f, "--json")
	_, tableOut, _ := runUsage(f)

	if strings.TrimSpace(jsonOut) != "[]" || tableOut != "PHASE\tCOMMAND\tCALLS\tHITS\tFAILURES\n" {
		t.Errorf("json=%q table=%q", jsonOut, tableOut)
	}
}

func TestCodeUsage_AnEmptyPhaseShowsAsNone(t *testing.T) {
	t.Parallel()
	f := emptyFixture(t)
	path, _ := usagelog.UserPath(f.cacheDir)
	_ = usagelog.Append(path, usagelog.Record{TS: time.Now(), Command: "map"}, 0)

	_, stdout, _ := runUsage(f)

	if !strings.Contains(stdout, "\n(none)\tmap\t1\t0\t0\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestCodeUsage_MalformedLinesAreReportedOnStderr(t *testing.T) {
	t.Parallel()
	f := seedUsageLog(t)
	path, _ := usagelog.UserPath(f.cacheDir)
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = file.WriteString("{torn\n")
	_ = file.Close()

	rc, _, stderr := runUsage(f)

	if rc != 0 || !strings.Contains(stderr, "skipped 1 malformed line") {
		t.Errorf("rc=%d stderr=%q", rc, stderr)
	}
}

func TestCodeUsage_BadInputExitsTenAndAnUnreadableLogExitsFour(t *testing.T) {
	t.Parallel()
	f := seedUsageLog(t)
	for _, args := range [][]string{{"--since", "soon"}, {"--since", "-1h"}, {"extra"}, {"--nope"}} {
		if rc, _, stderr := runUsage(f, args...); rc != codeExitUsage || stderr == "" {
			t.Errorf("args %v: rc=%d stderr=%q", args, rc, stderr)
		}
	}
	noHome := codeUsageFixture{cacheDir: func() (string, error) { return "", errors.New("no home") }, root: f.root}
	if rc, _, stderr := runUsage(noHome); rc != codeExitFailed || !strings.Contains(stderr, "no home") {
		t.Errorf("no cache dir: rc=%d stderr=%q", rc, stderr)
	}
	userPath, _ := usagelog.UserPath(f.cacheDir)
	_ = os.Remove(userPath)
	_ = os.Mkdir(userPath, 0o700)
	if rc, _, stderr := runUsage(f); rc != codeExitFailed || !strings.Contains(stderr, "evolve code usage:") {
		t.Errorf("unreadable: rc=%d stderr=%q", rc, stderr)
	}
}

func TestRunCodeCommand_UsageRoutesToTheUsageVerb(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	rc := runCodeCommand([]string{"usage", "--json"}, nil, &stdout, &stderr)

	if rc != 0 || !strings.HasPrefix(strings.TrimSpace(stdout.String()), "[") {
		t.Errorf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

func countMapCalls(t *testing.T, root string) int {
	t.Helper()
	userPath, err := usagelog.UserPath(codeUsageCacheDir)
	if err != nil {
		t.Fatal(err)
	}
	records, _, err := usagelog.ReadAll(userPath, root, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, rec := range records {
		if rec.Command == "map" {
			n++
		}
	}
	return n
}

func TestRunCodeCommand_ARealCallAppendsOneRecordForTheProject(t *testing.T) {
	t.Parallel()
	root := codePolicyRoot(t, map[string]string{"trail_bin": strconvQuote(filepath.Join(t.TempDir(), "no-trail"))})
	before := countMapCalls(t, root)

	rc := runCodeCommand([]string{"map", "--project-root", root}, nil, &bytes.Buffer{}, &bytes.Buffer{})

	if rc != 2 || countMapCalls(t, root) != before+1 {
		t.Errorf("rc=%d map calls before=%d after=%d", rc, before, countMapCalls(t, root))
	}
}
