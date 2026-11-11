//go:build acs

package cycle2297

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/usagelog"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const noTestsMarker = "no tests to run"

func runCmdEvolveTests(t *testing.T, pattern string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleDir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-run", pattern, "./cmd/evolve")
	if code != 0 || runErr != nil || strings.Contains(stdout+stderr, noTestsMarker) {
		t.Errorf("go test -run %s ./cmd/evolve exit=%d err=%v\n%s", pattern, code, runErr, acsassert.OutputTail(stdout+stderr, 30))
	}
}

func gotypesAndTrail(index map[codeindex.Request][]codeindex.Hit) codeindex.Backends {
	return codeindex.NewBackends(
		codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true, API: true}, index),
		codeindextest.NewFake("trail", codeindex.Capabilities{Find: true, Map: true}, index),
	)
}

func TestC2297_001_SearchReportsGotypesForCallersAndTrailForFind(t *testing.T) {
	callers := codeindex.Request{Query: codeindex.QueryCallers, Arg: "run", Direction: codeindex.DirectionIn}
	find := codeindex.Request{Query: codeindex.QueryFind, Arg: "run"}
	hit := []codeindex.Hit{{File: "a.go", Line: 1, Symbol: "s"}}
	backends := gotypesAndTrail(map[codeindex.Request][]codeindex.Hit{callers: hit, find: hit})

	for req, want := range map[codeindex.Request]string{callers: "gotypes", find: "trail"} {
		_, served, err := backends.SearchServed(context.Background(), req)
		if err != nil || served != want {
			t.Errorf("SearchServed(%s) served=%q err=%v; want %q", req.Query, served, err, want)
		}
	}
}

func TestC2297_001_SearchNamesNoBackendWhenNoneServesTheQuery(t *testing.T) {
	backends := codeindex.NewBackends(codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil))

	_, served, err := backends.SearchServed(context.Background(), codeindex.Request{Query: codeindex.QueryFind, Arg: "x"})

	if !errors.Is(err, codeindex.ErrUnsupported) || served != "" {
		t.Errorf("served=%q err=%v; want ErrUnsupported and no backend", served, err)
	}
}

func TestC2297_002_TheEvolveCodeRouteLogsTheBackendThatAnswered(t *testing.T) {
	runCmdEvolveTests(t, "^(TestCodeVerbLoggedBackend_|TestRunCodeCommandBackend_)")
}

func TestC2297_003_UsageSummaryKeepsEachBackendInItsOwnRow(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	record := func(backend string, hits int) usagelog.Record {
		return usagelog.Record{TS: at, Command: "find", Phase: "scout", Backend: backend, Hits: hits}
	}

	rows := usagelog.Summarize([]usagelog.Record{record("trail", 2), record("gotypes", 5), record("trail", 1), record("", 4)})

	want := []usagelog.Row{
		{Phase: "scout", Command: "find", Backend: "", Calls: 1, Hits: 4},
		{Phase: "scout", Command: "find", Backend: "gotypes", Calls: 1, Hits: 5},
		{Phase: "scout", Command: "find", Backend: "trail", Calls: 2, Hits: 3},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

func TestC2297_004_EvolveCodeUsageGroupsTheRecordsByBackend(t *testing.T) {
	runCmdEvolveTests(t, "^(TestCodeUsageBackend_TheTable|TestCodeUsageBackend_JSON|TestCodeUsageBackend_AWritten)")
}

func TestC2297_005_ALegacyRecordWithNoBackendFieldStillReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), usagelog.FileName)
	legacy := `{"ts":"2026-10-17T09:00:00Z","command":"callers","arg_len":7,"exit":0,"hits":4,"duration_ms":12,"phase":"scout","cycle":3,"cli":"claude","cwd":"/w"}` + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	records, skipped, err := usagelog.Read(path, time.Time{})

	if err != nil || skipped != 0 || len(records) != 1 || records[0].Backend != "" || records[0].Hits != 4 {
		t.Errorf("Read = %+v skipped=%d err=%v; want one record with an empty backend", records, skipped, err)
	}
}

func TestC2297_005_ABackendRecordRoundTripsAndABrokenLineIsStillSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), usagelog.FileName)
	rec := usagelog.Record{TS: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC), Command: "find", Backend: "trail", Hits: 2}
	if err := usagelog.Append(path, rec, 0); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("{torn\n")
	_ = file.Close()

	records, skipped, err := usagelog.Read(path, time.Time{})

	if err != nil || skipped != 1 || len(records) != 1 || records[0] != rec {
		t.Errorf("Read = %+v skipped=%d err=%v; want %+v and one skipped line", records, skipped, err, rec)
	}
}

func TestC2297_006_EvolveCodeUsageShowsAnOldRecordAsUnknown(t *testing.T) {
	runCmdEvolveTests(t, "^(TestCodeUsageBackend_AnOldRecord|TestCodeUsage_MergesThePerUserFile|TestCodeUsage_AnEmptyPhaseShowsAsNone)")
}
