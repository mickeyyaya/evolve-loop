package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/usagelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func gotypesAndTrail(index map[codeindex.Request][]codeindex.Hit) []codeindex.Backend {
	return []codeindex.Backend{
		codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true, API: true}, index),
		codeindextest.NewFake("trail", codeindex.Capabilities{Find: true, Map: true}, index),
	}
}

func oneHitFor(reqs ...codeindex.Request) map[codeindex.Request][]codeindex.Hit {
	index := map[codeindex.Request][]codeindex.Hit{}
	for _, req := range reqs {
		index[req] = []codeindex.Hit{{File: "a.go", Line: 1, Symbol: "s"}}
	}
	return index
}

func loggedBackendOf(t *testing.T, backends []codeindex.Backend, args ...string) (int, usagelog.Record) {
	t.Helper()
	sink, path := userSink(t, nil)
	rc, _, _ := runLogged(t, sink, codeIndexOf(backends...), args...)
	records := readLog(t, path)
	if len(records) != 1 {
		t.Fatalf("records = %v, want exactly one", records)
	}
	return rc, records[0]
}

func TestCodeVerbLoggedBackend_ACallersCallRecordsGotypes(t *testing.T) {
	t.Parallel()
	index := oneHitFor(codeindex.Request{Query: codeindex.QueryCallers, Arg: "run", Direction: codeindex.DirectionIn})

	rc, rec := loggedBackendOf(t, gotypesAndTrail(index), "callers", "run")

	if rc != 0 || rec.Backend != "gotypes" || rec.Command != "callers" || rec.Hits != 1 {
		t.Errorf("rc=%d record=%+v; want backend gotypes", rc, rec)
	}
}

func TestCodeVerbLoggedBackend_AFindCallRecordsTrail(t *testing.T) {
	t.Parallel()
	index := oneHitFor(codeindex.Request{Query: codeindex.QueryFind, Arg: "run"})

	rc, rec := loggedBackendOf(t, gotypesAndTrail(index), "find", "run")

	if rc != 0 || rec.Backend != "trail" || rec.Command != "find" || rec.Hits != 1 {
		t.Errorf("rc=%d record=%+v; want backend trail", rc, rec)
	}
}

func TestCodeVerbLoggedBackend_TheRecordedNameIsTheBackendThatAnsweredNotAConstant(t *testing.T) {
	t.Parallel()
	index := oneHitFor(codeindex.Request{Query: codeindex.QueryFind, Arg: "run"})
	backends := []codeindex.Backend{codeindextest.NewFake("scip", codeindex.AllCapabilities(), index)}

	_, rec := loggedBackendOf(t, backends, "find", "run")

	if rec.Backend != "scip" {
		t.Errorf("backend = %q, want scip", rec.Backend)
	}
}

func TestCodeVerbLoggedBackend_ANotFoundAnswerStillNamesItsBackend(t *testing.T) {
	t.Parallel()

	rc, rec := loggedBackendOf(t, gotypesAndTrail(nil), "find", "nothing")

	if rc != codeExitNotFound || rec.Backend != "trail" || rec.Exit != codeExitNotFound {
		t.Errorf("rc=%d record=%+v; want exit 1 from trail", rc, rec)
	}
}

func TestCodeVerbLoggedBackend_AnAbsentIndexStillNamesItsBackend(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{codeindextest.NewAbsentFake("trail", codeindex.AllCapabilities())}

	rc, rec := loggedBackendOf(t, backends, "find", "x")

	if rc != codeExitAbsent || rec.Backend != "trail" {
		t.Errorf("rc=%d record=%+v; want exit 2 from trail", rc, rec)
	}
}

func TestCodeVerbLoggedBackend_ACallNoBackendServesRecordsNoBackend(t *testing.T) {
	t.Parallel()
	backends := []codeindex.Backend{codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil)}

	rc, rec := loggedBackendOf(t, backends, "find", "x")

	if rc != codeExitUnsupported || rec.Backend != "" {
		t.Errorf("rc=%d record=%+v; want exit 3 and an empty backend", rc, rec)
	}
}

func TestCodeVerbLoggedBackend_AUsageErrorRecordsNoBackend(t *testing.T) {
	t.Parallel()

	rc, rec := loggedBackendOf(t, gotypesAndTrail(nil), "callers")

	if rc != codeExitUsage || rec.Backend != "" {
		t.Errorf("rc=%d record=%+v; want exit 10 and an empty backend", rc, rec)
	}
}

func seedBackendUsage(t *testing.T) codeUsageFixture {
	t.Helper()
	f := emptyFixture(t)
	path, _ := usagelog.UserPath(f.cacheDir)
	now := time.Now()
	for _, rec := range []usagelog.Record{
		{TS: now, Command: "find", Phase: "scout", Backend: "trail", Hits: 2},
		{TS: now, Command: "find", Phase: "scout", Backend: "trail", Hits: 0, Exit: 1},
		{TS: now, Command: "callers", Phase: "scout", Backend: "gotypes", Hits: 5},
		{TS: now, Command: "callers", Phase: "scout", Hits: 3},
		{TS: now, Command: "callers", Phase: "scout", Backend: "gotypes", Hits: 1},
	} {
		if err := usagelog.Append(path, rec, 0); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCodeUsageBackend_TheTableGroupsTheRecordsByBackend(t *testing.T) {
	t.Parallel()

	rc, stdout, stderr := runUsage(seedBackendUsage(t))

	want := "PHASE\tCOMMAND\tBACKEND\tCALLS\tHITS\tFAILURES\n" +
		"scout\tcallers\tunknown\t1\t3\t0\n" +
		"scout\tcallers\tgotypes\t2\t6\t0\n" +
		"scout\tfind\ttrail\t2\t2\t1\n"
	if rc != 0 || stdout != want || stderr != "" {
		t.Errorf("rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
	}
}

func TestCodeUsageBackend_AnOldRecordWithNoBackendShowsAsUnknownAndDoesNotFail(t *testing.T) {
	t.Parallel()
	f := emptyFixture(t)
	path, _ := usagelog.UserPath(f.cacheDir)
	_ = usagelog.Append(path, usagelog.Record{TS: time.Now(), Command: "map", Phase: "builder"}, 0)

	rc, stdout, stderr := runUsage(f)

	if rc != 0 || stderr != "" || !strings.Contains(stdout, "\nbuilder\tmap\tunknown\t1\t0\t0\n") {
		t.Errorf("rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
	}
}

func TestCodeUsageBackend_JSONRowsCarryTheBackend(t *testing.T) {
	t.Parallel()

	rc, stdout, _ := runUsage(seedBackendUsage(t), "--json")

	var rows []usagelog.Row
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil || rc != 0 || len(rows) != 3 {
		t.Fatalf("rc=%d rows=%v err=%v stdout=%q", rc, rows, err, stdout)
	}
	want := []usagelog.Row{
		{Phase: "scout", Command: "callers", Backend: "", Calls: 1, Hits: 3},
		{Phase: "scout", Command: "callers", Backend: "gotypes", Calls: 2, Hits: 6},
		{Phase: "scout", Command: "find", Backend: "trail", Calls: 2, Hits: 2, Failures: 1},
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
	if !strings.Contains(stdout, `"backend":"trail"`) {
		t.Errorf("stdout = %q, want the backend key", stdout)
	}
}

func TestCodeUsageBackend_AWrittenCallIsReadBackUnderItsBackend(t *testing.T) {
	t.Parallel()
	sink, _ := userSink(t, nil)
	index := oneHitFor(codeindex.Request{Query: codeindex.QueryFind, Arg: "run"})
	runLogged(t, sink, codeIndexOf(gotypesAndTrail(index)...), "find", "run")
	cache, _ := sink.cacheDir()

	rc, stdout, _ := runUsage(codeUsageFixture{cacheDir: func() (string, error) { return cache, nil }, root: t.TempDir()})

	if rc != 0 || !strings.Contains(stdout, "\n(none)\tfind\ttrail\t1\t1\t0\n") {
		t.Errorf("rc=%d stdout=%q", rc, stdout)
	}
}

func countTrailMapCalls(t *testing.T, root string) int {
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
		if rec.Command == "map" && rec.Backend == "trail" {
			n++
		}
	}
	return n
}

func TestRunCodeCommandBackend_ARealCallLogsTheBackendTheRouteUsed(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "")
	root := codePolicyRoot(t, map[string]string{"trail_bin": strconvQuote(filepath.Join(t.TempDir(), "no-trail"))})
	before := countTrailMapCalls(t, root)

	rc := runCodeCommand([]string{"map", "--project-root", root}, nil, &bytes.Buffer{}, &bytes.Buffer{})

	if rc != codeExitAbsent || countTrailMapCalls(t, root) != before+1 {
		t.Errorf("rc=%d trail map calls before=%d after=%d", rc, before, countTrailMapCalls(t, root))
	}
}
