//go:build acs

package cycle2294

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	benchRounds     = 3
	measuredFloor   = 0.95
	floatTolerance  = 1e-9
	planDoc         = "docs/plans/code-index-backends-2026-10.md"
	gotypesBackend  = "gotypes"
	callersRowLabel = "callers"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

type timedBackend struct {
	codeindextest.Fake
	clock   *fakeClock
	latency func(call int) time.Duration
	fail    map[string]error
	calls   int
}

func (b *timedBackend) Callers(ctx context.Context, symbol string, dir codeindex.Direction) ([]codeindex.Hit, error) {
	b.calls++
	if b.latency != nil {
		b.clock.advance(b.latency(b.calls))
	}
	if err, ok := b.fail[symbol]; ok {
		return nil, err
	}
	return b.Fake.Callers(ctx, symbol, dir)
}

func callersRequest(symbol string) codeindex.Request {
	return codeindex.Request{Query: codeindex.QueryCallers, Arg: symbol, Direction: codeindex.DirectionIn}
}

func newFake(name string, answers map[string][]codeindex.Hit) codeindextest.Fake {
	index := map[codeindex.Request][]codeindex.Hit{}
	for symbol, hits := range answers {
		index[callersRequest(symbol)] = hits
	}
	return codeindextest.NewFake(name, codeindex.AllCapabilities(), index)
}

func newTimed(clock *fakeClock, name string, answers map[string][]codeindex.Hit) *timedBackend {
	return &timedBackend{Fake: newFake(name, answers), clock: clock}
}

func hitAt(file string, line int, fn string) codeindex.Hit {
	return codeindex.Hit{File: file, Line: line, Symbol: fn, Text: "static"}
}

func siteAt(file string, line int, fn string) codeindex.TruthSite {
	return codeindex.TruthSite{File: file, Line: line, Func: fn}
}

func symbolOf(name string, sites ...codeindex.TruthSite) codeindex.BenchSymbol {
	return codeindex.BenchSymbol{Name: name, Sites: sites}
}

func newRun(clock *fakeClock, symbols []codeindex.BenchSymbol, backends ...codeindex.Backend) codeindex.BenchRun {
	return codeindex.BenchRun{
		Backends: backends,
		Symbols:  symbols,
		Rounds:   benchRounds,
		Now:      clock.Now,
		LoadAvg:  func() float64 { return 0.5 },
	}
}

func runBench(t *testing.T, run codeindex.BenchRun) codeindex.BenchReport {
	t.Helper()
	report, err := codeindex.RunBench(context.Background(), run)
	if err != nil {
		t.Fatalf("RunBench: %v", err)
	}
	return report
}

func rowOf(t *testing.T, report codeindex.BenchReport, backend string) codeindex.BenchRow {
	t.Helper()
	for _, row := range report.Rows {
		if row.Backend == backend && row.Query == codeindex.QueryCallers {
			return row
		}
	}
	t.Fatalf("no callers row for backend %q in %+v", backend, report.Rows)
	return codeindex.BenchRow{}
}

func near(a, b float64) bool { return math.Abs(a-b) < floatTolerance }

func TestC2294_001_AFakeBackendWithTwoFalsePositivesAndOneFalseNegativeGivesExactPrecisionAndRecall(t *testing.T) {
	truth := symbolOf("pkg.Target",
		siteAt("f1.go", 10, "pkg.A"), siteAt("f2.go", 20, "pkg.B"),
		siteAt("f3.go", 30, "pkg.C"), siteAt("f4.go", 40, "pkg.D"))
	clock := &fakeClock{}
	fake := newTimed(clock, "fake", map[string][]codeindex.Hit{
		"pkg.Target": {
			hitAt("f1.go", 10, "pkg.A"), hitAt("f2.go", 20, "pkg.B"), hitAt("f3.go", 30, "pkg.C"),
			hitAt("f9.go", 90, "pkg.X"), hitAt("f8.go", 80, "pkg.Y"),
		},
	})
	row := rowOf(t, runBench(t, newRun(clock, []codeindex.BenchSymbol{truth}, fake)), "fake")
	if row.TP != 3 || row.FP != 2 || row.FN != 1 {
		t.Errorf("TP/FP/FN = %d/%d/%d, want 3/2/1", row.TP, row.FP, row.FN)
	}
	if !near(row.Precision, 0.6) {
		t.Errorf("precision = %v, want 0.6 (3 true of 5 returned)", row.Precision)
	}
	if !near(row.Recall, 0.75) {
		t.Errorf("recall = %v, want 0.75 (3 found of 4 true)", row.Recall)
	}
}

func TestC2294_002_ScoresAreMicroAveragedOverSymbolsAndACallerIsAFileAndFunctionPairCountedOnce(t *testing.T) {
	one := symbolOf("pkg.One",
		siteAt("f.go", 10, "p.F"), siteAt("f.go", 12, "p.F"), siteAt("f.go", 20, "p.G"))
	two := symbolOf("pkg.Two",
		siteAt("c.go", 1, "p.C"), siteAt("d.go", 2, "p.D"), siteAt("e.go", 3, "p.E"), siteAt("g.go", 4, "p.H"))
	clock := &fakeClock{}
	fake := newTimed(clock, "fake", map[string][]codeindex.Hit{
		"pkg.One": {hitAt("f.go", 10, "p.F"), hitAt("f.go", 12, "p.F"), hitAt("f.go", 20, "p.G")},
		"pkg.Two": {hitAt("c.go", 1, "p.C")},
	})
	row := rowOf(t, runBench(t, newRun(clock, []codeindex.BenchSymbol{one, two}, fake)), "fake")
	if row.TP != 3 || row.FP != 0 || row.FN != 3 {
		t.Errorf("TP/FP/FN = %d/%d/%d, want 3/0/3 (two sites of one function count once; rounds do not multiply)", row.TP, row.FP, row.FN)
	}
	if !near(row.Precision, 1.0) || !near(row.Recall, 0.5) {
		t.Errorf("precision/recall = %v/%v, want 1.0/0.5 (micro average over both symbols, not 1.0/0.625 macro)", row.Precision, row.Recall)
	}
}

const libSource = "package pkg\n\nfunc Run() {}\n\nfunc Use() {\n\tRun()\n}\n"

const otherSource = "package other\n\nimport \"example.com/pkg\"\n\nfunc Also() {\n\tpkg.Run()\n}\n"

func writeGrepFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{"pkg/lib.go": libSource, "other/use.go": otherSource} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func grepSymbol(files []string, sites ...codeindex.TruthSite) codeindex.BenchSymbol {
	sym := symbolOf("pkg.Run", sites...)
	sym.GrepFiles = files
	return sym
}

func TestC2294_003_TheGrepCheckFailsWhenTheTruthAndGrepDisagreeInEitherDirection(t *testing.T) {
	root := writeGrepFixture(t)
	both := []string{"pkg/lib.go", "other/use.go"}
	agreeing := grepSymbol(both, siteAt("pkg/lib.go", 6, "pkg.Use"), siteAt("other/use.go", 6, "other.Also"))
	if err := codeindex.GrepCheck(root, agreeing); err != nil {
		t.Fatalf("truth and grep agree (the definition line is not a call; pkg.Run( is a call), got %v", err)
	}
	truthMissesARealCall := grepSymbol(both, siteAt("pkg/lib.go", 6, "pkg.Use"))
	if err := codeindex.GrepCheck(root, truthMissesARealCall); !errors.Is(err, codeindex.ErrTruthDisagrees) {
		t.Errorf("truth lacks the real call other/use.go:6: err = %v, want ErrTruthDisagrees", err)
	}
	siteWithNoCall := grepSymbol(both, siteAt("pkg/lib.go", 5, "pkg.Use"), siteAt("other/use.go", 6, "other.Also"))
	if err := codeindex.GrepCheck(root, siteWithNoCall); !errors.Is(err, codeindex.ErrTruthDisagrees) {
		t.Errorf("truth site pkg/lib.go:5 has no call: err = %v, want ErrTruthDisagrees", err)
	}
	if err := codeindex.GrepCheck(root, grepSymbol([]string{"nosuch.go"}, siteAt("nosuch.go", 1, "x"))); err == nil {
		t.Errorf("a file that cannot be read must fail the check, got nil")
	}
}

func TestC2294_004_ARunWhoseGrepCheckDisagreesWithTheTruthFailsAndReportsNoTable(t *testing.T) {
	root := writeGrepFixture(t)
	both := []string{"pkg/lib.go", "other/use.go"}
	answers := map[string][]codeindex.Hit{"pkg.Run": {hitAt("pkg/lib.go", 6, "pkg.Use")}}
	build := func(sym codeindex.BenchSymbol) codeindex.BenchRun {
		clock := &fakeClock{}
		run := newRun(clock, []codeindex.BenchSymbol{sym}, newTimed(clock, "fake", answers))
		run.Root = root
		run.GrepSymbols = 10
		return run
	}
	missing := grepSymbol(both, siteAt("pkg/lib.go", 6, "pkg.Use"))
	report, err := codeindex.RunBench(context.Background(), build(missing))
	if !errors.Is(err, codeindex.ErrTruthDisagrees) {
		t.Errorf("a truth that misses a real call must fail the run: err = %v", err)
	}
	if len(report.Rows) != 0 {
		t.Errorf("a failed run must not report a table, got %d rows", len(report.Rows))
	}
	agreeing := grepSymbol(both, siteAt("pkg/lib.go", 6, "pkg.Use"), siteAt("other/use.go", 6, "other.Also"))
	report, err = codeindex.RunBench(context.Background(), build(agreeing))
	if err != nil || len(report.Rows) != 1 {
		t.Errorf("an agreeing truth must pass: err = %v, rows = %d, want nil and 1", err, len(report.Rows))
	}
}

func fieldsOfRow(table, backend, query string) []string {
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 8 && fields[0] == backend && fields[1] == query {
			return fields
		}
	}
	return nil
}

func TestC2294_005_TheTableHasPrecisionRecallP50P95BytesAndFailuresForEachBackendAndTheHostLoadAverage(t *testing.T) {
	truth := symbolOf("pkg.Target",
		siteAt("f1.go", 10, "pkg.A"), siteAt("f2.go", 20, "pkg.B"),
		siteAt("f3.go", 30, "pkg.C"), siteAt("f4.go", 40, "pkg.D"))
	clock := &fakeClock{}
	alpha := newTimed(clock, "alpha", map[string][]codeindex.Hit{
		"pkg.Target": {
			hitAt("f1.go", 10, "pkg.A"), hitAt("f2.go", 20, "pkg.B"), hitAt("f3.go", 30, "pkg.C"),
			hitAt("f9.go", 90, "pkg.X"), hitAt("f8.go", 80, "pkg.Y"),
		},
	})
	beta := newTimed(clock, "beta", map[string][]codeindex.Hit{
		"pkg.Target": {
			hitAt("f1.go", 10, "pkg.A"), hitAt("f2.go", 20, "pkg.B"),
			hitAt("f3.go", 30, "pkg.C"), hitAt("f4.go", 40, "pkg.D"),
		},
	})
	run := newRun(clock, []codeindex.BenchSymbol{truth}, alpha, beta)
	run.LoadAvg = func() float64 { return 7.25 }
	report := runBench(t, run)
	if !near(report.LoadAvg, 7.25) {
		t.Errorf("report.LoadAvg = %v, want the injected 7.25", report.LoadAvg)
	}
	table := report.Table()
	header := ""
	for _, line := range strings.Split(table, "\n") {
		if strings.Contains(strings.ToLower(line), "precision") {
			header = strings.ToLower(line)
			break
		}
	}
	last := -1
	for _, column := range []string{"backend", "query", "precision", "recall", "p50", "p95", "bytes", "failures"} {
		at := strings.Index(header, column)
		if at <= last {
			t.Errorf("header %q lacks column %q in order after position %d", header, column, last)
		}
		last = at
	}
	if !strings.Contains(strings.ToLower(table), "load average") || !strings.Contains(table, "7.25") {
		t.Errorf("table must print the host load average 7.25 under the words \"load average\":\n%s", table)
	}
	for _, want := range []struct{ backend, precision, recall string }{{"alpha", "0.60", "0.75"}, {"beta", "1.00", "1.00"}} {
		fields := fieldsOfRow(table, want.backend, callersRowLabel)
		if fields == nil {
			t.Errorf("no %q %q row in table:\n%s", want.backend, callersRowLabel, table)
			continue
		}
		if fields[2] != want.precision || fields[3] != want.recall {
			t.Errorf("%s precision/recall = %s/%s, want %s/%s", want.backend, fields[2], fields[3], want.precision, want.recall)
		}
		for _, i := range []int{4, 5} {
			if _, err := time.ParseDuration(fields[i]); err != nil {
				t.Errorf("%s column %d = %q is not a duration", want.backend, i, fields[i])
			}
		}
		if _, err := strconv.Atoi(fields[6]); err != nil {
			t.Errorf("%s bytes = %q is not an integer", want.backend, fields[6])
		}
		if fields[7] != "0" {
			t.Errorf("%s failures = %q, want 0", want.backend, fields[7])
		}
	}
}

func TestC2294_006_LatencyIsMeasuredForEachBackendByNearestRankPercentileOverEveryRound(t *testing.T) {
	symbols := []codeindex.BenchSymbol{
		symbolOf("pkg.One", siteAt("a.go", 1, "p.A")),
		symbolOf("pkg.Two", siteAt("b.go", 2, "p.B")),
	}
	answers := map[string][]codeindex.Hit{
		"pkg.One": {hitAt("a.go", 1, "p.A")},
		"pkg.Two": {hitAt("b.go", 2, "p.B")},
	}
	clock := &fakeClock{}
	rising := newTimed(clock, "rising", answers)
	rising.latency = func(call int) time.Duration { return time.Duration(call) * time.Millisecond }
	slow := newTimed(clock, "slow", answers)
	slow.latency = func(int) time.Duration { return 200 * time.Millisecond }
	run := newRun(clock, symbols, rising, slow)
	run.Rounds = 5
	report := runBench(t, run)
	if rising.calls != 10 || slow.calls != 10 {
		t.Errorf("calls = %d and %d, want 10 each (2 symbols x 5 rounds, each backend measured on its own)", rising.calls, slow.calls)
	}
	if row := rowOf(t, report, "rising"); row.P50 != 5*time.Millisecond || row.P95 != 10*time.Millisecond {
		t.Errorf("rising p50/p95 = %v/%v, want 5ms/10ms (nearest rank over 1..10 ms)", row.P50, row.P95)
	}
	if row := rowOf(t, report, "slow"); row.P50 != 200*time.Millisecond || row.P95 != 200*time.Millisecond {
		t.Errorf("slow p50/p95 = %v/%v, want 200ms/200ms", row.P50, row.P95)
	}
}

func TestC2294_007_BytesAreTheMedianAnswerSizeAndAFailedCallIsCountedNotFatal(t *testing.T) {
	target := symbolOf("pkg.Target", siteAt("a.go", 1, "p.A"))
	clock := &fakeClock{}
	one := newTimed(clock, "one", map[string][]codeindex.Hit{"pkg.Target": {hitAt("a.go", 1, "p.A")}})
	three := newTimed(clock, "three", map[string][]codeindex.Hit{
		"pkg.Target": {hitAt("a.go", 1, "p.A"), hitAt("a.go", 2, "p.A"), hitAt("a.go", 3, "p.A")},
	})
	report := runBench(t, newRun(clock, []codeindex.BenchSymbol{target}, one, three))
	oneBytes, threeBytes := rowOf(t, report, "one").MedianBytes, rowOf(t, report, "three").MedianBytes
	if oneBytes <= 0 || threeBytes != 3*oneBytes {
		t.Errorf("median bytes one/three = %d/%d, want a positive size and exactly 3x for three equal hits", oneBytes, threeBytes)
	}

	symbols := []codeindex.BenchSymbol{
		symbolOf("pkg.Fine", siteAt("a.go", 1, "p.A")),
		symbolOf("pkg.Gone", siteAt("b.go", 2, "p.B"), siteAt("c.go", 3, "p.C")),
		symbolOf("pkg.Boom", siteAt("d.go", 4, "p.D")),
	}
	flaky := newTimed(clock, "flaky", map[string][]codeindex.Hit{"pkg.Fine": {hitAt("a.go", 1, "p.A")}})
	flaky.fail = map[string]error{"pkg.Boom": errors.New("boom")}
	row := rowOf(t, runBench(t, newRun(clock, symbols, flaky)), "flaky")
	if row.Failures != benchRounds {
		t.Errorf("failures = %d, want %d (pkg.Boom errors once per round; pkg.Gone is ErrNotFound, an empty answer, not a failure)", row.Failures, benchRounds)
	}
	if row.TP != 1 || row.FP != 0 || row.FN != 3 {
		t.Errorf("TP/FP/FN = %d/%d/%d, want 1/0/3 (a missing or failed answer loses recall)", row.TP, row.FP, row.FN)
	}
}

func candidates(offset int, nameFmt string) []codeindex.BenchSymbol {
	var out []codeindex.BenchSymbol
	for i := 0; i < 20; i++ {
		out = append(out, codeindex.BenchSymbol{
			Name:    fmt.Sprintf(nameFmt, i),
			DefFile: fmt.Sprintf("pkg/f%d.go", i%4),
			DefLine: offset + 10 + i*7,
		})
	}
	return out
}

func positions(syms []codeindex.BenchSymbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, fmt.Sprintf("%s:%d", s.DefFile, s.DefLine))
	}
	slices.Sort(out)
	return out
}

func names(syms []codeindex.BenchSymbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, s.Name)
	}
	slices.Sort(out)
	return out
}

func TestC2294_008_SymbolsAreSampledByAHashOfTheirPositionAndTheStandardProbesAreAlwaysAdded(t *testing.T) {
	probe := codeindex.BenchSymbol{Name: "p.Probe", DefFile: "pkg/probe.go", DefLine: 3}
	pool := append(candidates(0, "p.S%d"), probe)
	picked := codeindex.SampleSymbols(pool, 3, []string{"Probe"})
	if len(picked) != 4 || !slices.Contains(names(picked), "p.Probe") {
		t.Fatalf("picked %v, want 3 hashed symbols plus the probe p.Probe", names(picked))
	}
	if again := codeindex.SampleSymbols(pool, 3, []string{"Probe"}); !slices.Equal(positions(picked), positions(again)) {
		t.Errorf("the same input must give the same sample")
	}
	reversed := slices.Clone(pool)
	slices.Reverse(reversed)
	if got := codeindex.SampleSymbols(reversed, 3, []string{"Probe"}); !slices.Equal(positions(picked), positions(got)) {
		t.Errorf("the sample must not depend on the order of the input")
	}
	renamed := append(candidates(0, "q.Other%d"), probe)
	if got := codeindex.SampleSymbols(renamed, 3, []string{"Probe"}); !slices.Equal(positions(picked), positions(got)) {
		t.Errorf("the sample hashes the position, so renaming the symbols must not change it")
	}
	if got := codeindex.SampleSymbols(pool, 0, []string{"Probe"}); len(got) != 1 || got[0].Name != "p.Probe" {
		t.Errorf("n=0 must still add the probe, got %v", names(got))
	}
	if got := codeindex.SampleSymbols(pool, 100, nil); len(got) != len(pool) {
		t.Errorf("n above the pool size must return the whole pool, got %d of %d", len(got), len(pool))
	}
	prefixOrders := map[string]func(a, b codeindex.BenchSymbol) int{
		"input order": func(a, b codeindex.BenchSymbol) int { return 0 },
		"file then line": func(a, b codeindex.BenchSymbol) int {
			return cmp.Or(cmp.Compare(a.DefFile, b.DefFile), cmp.Compare(a.DefLine, b.DefLine))
		},
		"line": func(a, b codeindex.BenchSymbol) int { return cmp.Compare(a.DefLine, b.DefLine) },
		"position text": func(a, b codeindex.BenchSymbol) int {
			return cmp.Compare(fmt.Sprintf("%s:%d", a.DefFile, a.DefLine), fmt.Sprintf("%s:%d", b.DefFile, b.DefLine))
		},
	}
	for label, order := range prefixOrders {
		differs := false
		for _, offset := range []int{0, 1, 2, 3, 4} {
			set := candidates(offset, "p.S%d")
			ordered := slices.Clone(set)
			slices.SortStableFunc(ordered, order)
			if !slices.Equal(positions(codeindex.SampleSymbols(set, 3, nil)), positions(ordered[:3])) {
				differs = true
			}
		}
		if !differs {
			t.Errorf("the sample equals the first 3 symbols by %s for every pool: it is not a hash of the position", label)
		}
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("RED: go build ./cmd/evolve exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 20), acsassert.OutputTail(errOut, 40))
	}
	return bin
}

func TestC2294_009_TheCodeVerbReachesBenchFromTheCommandLineAndRejectsAnUnknownBackendBeforeAnyWork(t *testing.T) {
	bin := buildEvolve(t)
	root := acsassert.RepoRoot(t)
	_, usage, code, _ := acsassert.SubprocessOutput(bin, "code")
	if code != 10 || !strings.Contains(usage, "bench") {
		t.Errorf("evolve code with no verb: exit=%d, want 10 with a usage line that names bench:\n%s", code, usage)
	}
	_, errOut, code, _ := acsassert.SubprocessOutput(bin, "code", "bench", "--backends", "zzz", "--project-root", root)
	if code == 0 || strings.Contains(errOut, "unknown verb") || !strings.Contains(errOut, "unknown backend") {
		t.Errorf("evolve code bench --backends zzz: exit=%d, want non-zero with an unknown-backend message (not unknown verb):\n%s", code, errOut)
	}
}

func parseRate(t *testing.T, label, raw string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("%s %q is not a number: %v", label, raw, err)
	}
	return v
}

func planRecords(t *testing.T, precision, recall string) bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), planDoc))
	if err != nil {
		t.Fatalf("read %s: %v", planDoc, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, gotypesBackend) && strings.Contains(line, precision) && strings.Contains(line, recall) {
			return true
		}
	}
	return false
}

func TestC2294_010_ARunOnTheRealIndexGivesGotypesCallersAtLeastPoint95OrThePlanRecordsTheMeasuredValues(t *testing.T) {
	bin := buildEvolve(t)
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput(bin, "code", "bench", "--backends", gotypesBackend, "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve code bench exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 30), acsassert.OutputTail(errOut, 30))
	}
	fields := fieldsOfRow(out, gotypesBackend, callersRowLabel)
	if fields == nil {
		t.Fatalf("no %s %s row in the bench table:\n%s", gotypesBackend, callersRowLabel, out)
	}
	if !strings.Contains(strings.ToLower(out), "load average") {
		t.Errorf("the table must print the host load average:\n%s", out)
	}
	precision, recall := parseRate(t, "precision", fields[2]), parseRate(t, "recall", fields[3])
	if precision >= measuredFloor && recall >= measuredFloor {
		return
	}
	if !planRecords(t, fields[2], fields[3]) {
		t.Errorf("gotypes callers measured precision %s and recall %s are below %.2f, and %s has no gotypes line with both values", fields[2], fields[3], measuredFloor, planDoc)
	}
}
