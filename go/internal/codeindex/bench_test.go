package codeindex_test

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
)

func benchClock() func() time.Time {
	now := time.Unix(0, 0)
	return func() time.Time { now = now.Add(time.Millisecond); return now }
}

func benchRun(symbols []codeindex.BenchSymbol, backends ...codeindex.Backend) codeindex.BenchRun {
	return codeindex.BenchRun{Backends: backends, Symbols: symbols, Rounds: 1, Now: benchClock(), LoadAvg: func() float64 { return 1.5 }}
}

func callersFake(name string, caps codeindex.Capabilities, answers map[string][]codeindex.Hit) codeindextest.Fake {
	index := map[codeindex.Request][]codeindex.Hit{}
	for symbol, hits := range answers {
		index[codeindex.Request{Query: codeindex.QueryCallers, Arg: symbol, Direction: codeindex.DirectionIn}] = hits
	}
	return codeindextest.NewFake(name, caps, index)
}

func TestRunBench_ABackendThatCannotServeCallersHasNoRow(t *testing.T) {
	t.Parallel()
	fake := callersFake("findonly", codeindex.Capabilities{Find: true}, nil)

	report, err := codeindex.RunBench(context.Background(), benchRun(nil, fake))

	if err != nil || len(report.Rows) != 0 || report.LoadAvg != 1.5 {
		t.Errorf("rows = %v, loadavg = %v, err = %v; want no row, 1.5 and nil", report.Rows, report.LoadAvg, err)
	}
}

func TestRunBench_ACancelledContextStopsTheRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sym := codeindex.BenchSymbol{Name: "p.F"}

	_, err := codeindex.RunBench(ctx, benchRun([]codeindex.BenchSymbol{sym}, callersFake("fake", codeindex.AllCapabilities(), nil)))

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRunBench_ABackendThatAnswersNothingScoresZeroAndKeepsItsFailureCount(t *testing.T) {
	t.Parallel()
	absent := codeindextest.NewAbsentFake("absent", codeindex.AllCapabilities())
	sym := codeindex.BenchSymbol{Name: "p.F", Sites: []codeindex.TruthSite{{File: "a.go", Line: 1, Func: "p.A"}}}
	run := benchRun([]codeindex.BenchSymbol{sym}, absent)
	run.Rounds = 2

	report, err := codeindex.RunBench(context.Background(), run)

	row := report.Rows[0]
	if err != nil || row.Failures != 2 || row.Precision != 0 || row.Recall != 0 || row.FN != 1 || row.MedianBytes != 0 {
		t.Errorf("row = %+v, err = %v; want 2 failures, zero scores and one missed caller", row, err)
	}
}

func TestRunBench_ZeroRoundsGiveZeroLatencyAndNoCalls(t *testing.T) {
	t.Parallel()
	run := benchRun([]codeindex.BenchSymbol{{Name: "p.F"}}, callersFake("fake", codeindex.AllCapabilities(), nil))
	run.Rounds = 0

	report, err := codeindex.RunBench(context.Background(), run)

	if row := report.Rows[0]; err != nil || row.P50 != 0 || row.P95 != 0 {
		t.Errorf("row = %+v, err = %v; want zero percentiles", row, err)
	}
}

func TestRunBench_TheGrepCheckSkipsSymbolsWithNoGrepFilesAndStopsAtTheSampleSize(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nfunc G() {\n\tF()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := codeindex.BenchSymbol{Name: "p.F", GrepFiles: []string{"a.go"}, Sites: []codeindex.TruthSite{{File: "a.go", Line: 4, Func: "p.G"}}}
	wrong := codeindex.BenchSymbol{Name: "p.F", GrepFiles: []string{"a.go"}}
	run := benchRun([]codeindex.BenchSymbol{{Name: "p.None"}, good, wrong}, callersFake("fake", codeindex.AllCapabilities(), nil))
	run.Root, run.GrepSymbols = root, 1

	if _, err := codeindex.RunBench(context.Background(), run); err != nil {
		t.Fatalf("one checked symbol agrees and the second is past the sample: %v", err)
	}
	run.GrepSymbols = 2
	if _, err := codeindex.RunBench(context.Background(), run); !errors.Is(err, codeindex.ErrTruthDisagrees) {
		t.Errorf("err = %v, want ErrTruthDisagrees from the second checked symbol", err)
	}
}

func TestGrepCheck_AMethodNameMatchesTheCallOnAnyReceiverAndSkipsItsDefinition(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := "package p\n\nfunc (s *S) Do() {}\n\nfunc use(s *S) {\n\ts.Do()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "m.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sym := codeindex.BenchSymbol{Name: "p.S.Do", GrepFiles: []string{"m.go"}, Sites: []codeindex.TruthSite{{File: "m.go", Line: 6, Func: "p.use"}}}

	if err := codeindex.GrepCheck(root, sym); err != nil {
		t.Errorf("GrepCheck = %v, want nil", err)
	}
}

func TestSampleSymbols_AProbeInsideTheHashedPrefixIsNotPickedTwice(t *testing.T) {
	t.Parallel()
	pool := []codeindex.BenchSymbol{
		{Name: "p.Probe", DefFile: "a.go", DefLine: 1},
		{Name: "p.Other", DefFile: "a.go", DefLine: 2},
		{Name: "p.Same", DefFile: "a.go", DefLine: 2},
	}

	got := codeindex.SampleSymbols(pool, 5, []string{"Probe"})

	if len(got) != 3 {
		t.Errorf("got %d symbols, want 3 with the probe once", len(got))
	}
}

func TestSampleSymbols_ASymbolAtTheSamePositionIsOrderedByName(t *testing.T) {
	t.Parallel()
	pool := []codeindex.BenchSymbol{{Name: "b", DefFile: "a.go", DefLine: 1}, {Name: "a", DefFile: "a.go", DefLine: 1}}

	got := codeindex.SampleSymbols(pool, 2, nil)

	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("got %v, want a before b at one position", got)
	}
}

func TestBenchReportTable_NamesTheEmptyTableAndTheLoad(t *testing.T) {
	t.Parallel()

	table := codeindex.BenchReport{LoadAvg: 3}.Table()

	if !strings.Contains(table, "precision") || !strings.Contains(table, "host load average: 3.00") {
		t.Errorf("table = %q", table)
	}
}

func TestResolve_BuildsEachNamedBackendInOrderAndFailsOnEmptyOrDuplicate(t *testing.T) {
	t.Parallel()
	factories := map[string]codeindex.Factory{
		"a": func() (codeindex.Backend, error) {
			return codeindextest.NewFake("a", codeindex.AllCapabilities(), nil), nil
		},
		"b": func() (codeindex.Backend, error) {
			return codeindextest.NewFake("b", codeindex.AllCapabilities(), nil), nil
		},
	}

	got, err := codeindex.Resolve([]string{"b", "a"}, factories)

	if err != nil || len(got) != 2 || got[0].Name() != "b" || got[1].Name() != "a" {
		t.Errorf("Resolve = %v, %v; want b then a", got, err)
	}
	if _, err := codeindex.Resolve(nil, factories); err == nil {
		t.Errorf("an empty list must fail")
	}
	if _, err := codeindex.Resolve([]string{"a", "a"}, factories); err == nil {
		t.Errorf("a duplicate name must fail")
	}
	if names := codeindex.KnownBackends(factories); strings.Join(names, ",") != "a,b" {
		t.Errorf("KnownBackends = %v, want a,b", names)
	}
}

func TestRunBench_ScoresTrueAndFalseCallersAndPrintsTheRow(t *testing.T) {
	t.Parallel()
	sym := codeindex.BenchSymbol{Name: "p.F", Sites: []codeindex.TruthSite{{File: "a.go", Line: 1, Func: "p.A"}, {File: "b.go", Line: 2, Func: "p.B"}}}
	fake := callersFake("fake", codeindex.AllCapabilities(), map[string][]codeindex.Hit{"p.F": {
		{File: "a.go", Line: 1, Symbol: "p.A", Text: "t"}, {File: "z.go", Line: 9, Symbol: "p.Z", Text: "t"},
	}})

	report, err := codeindex.RunBench(context.Background(), benchRun([]codeindex.BenchSymbol{sym}, fake))

	row := report.Rows[0]
	if err != nil || row.TP != 1 || row.FP != 1 || row.FN != 1 || row.Precision != 0.5 || row.Recall != 0.5 || row.MedianBytes != 18 {
		t.Errorf("row = %+v, err = %v; want 1/1/1, 0.5/0.5 and 18 bytes", row, err)
	}
	if line := report.Table(); !strings.Contains(line, "fake     callers  0.50       0.50") || !strings.Contains(line, "1ms") {
		t.Errorf("table = %q", line)
	}
}

func TestGrepCheck_AnUnreadableFileFailsWithoutBeingATruthDisagreement(t *testing.T) {
	t.Parallel()
	sym := codeindex.BenchSymbol{Name: "p.F", GrepFiles: []string{"missing.go"}}

	err := codeindex.GrepCheck(t.TempDir(), sym)

	if err == nil || errors.Is(err, codeindex.ErrTruthDisagrees) {
		t.Errorf("err = %v, want a read error", err)
	}
}

func TestBenchReportTable_PrintsEachRowItHolds(t *testing.T) {
	t.Parallel()
	row := codeindex.BenchRow{Backend: "b", Query: codeindex.QueryCallers, Precision: 0.25, Recall: 1, MedianBytes: 7, Failures: 2}

	table := codeindex.BenchReport{Rows: []codeindex.BenchRow{row}}.Table()

	if !strings.Contains(table, "b        callers  0.25       1.00") {
		t.Errorf("table = %q", table)
	}
}
