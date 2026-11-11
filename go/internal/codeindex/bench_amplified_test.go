package codeindex_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

func pool(n int) []codeindex.BenchSymbol {
	out := make([]codeindex.BenchSymbol, n)
	for i := range out {
		out[i] = codeindex.BenchSymbol{Name: string(rune('a' + i)), DefFile: "f.go", DefLine: i + 1}
	}
	return out
}

func TestAmp_SampleSymbols_EmptyPoolGivesNothing(t *testing.T) {
	if got := codeindex.SampleSymbols(nil, 5, nil); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestAmp_SampleSymbols_ZeroAndNegativeCount(t *testing.T) {
	for _, n := range []int{0, -3} {
		if got := codeindex.SampleSymbols(pool(4), n, nil); len(got) != 0 {
			t.Fatalf("n=%d want empty, got %d", n, len(got))
		}
	}
}

func TestAmp_SampleSymbols_CountAbovePoolIsBounded(t *testing.T) {
	got := codeindex.SampleSymbols(pool(3), 50, nil)
	if len(got) > 3 {
		t.Fatalf("sample larger than pool: %d", len(got))
	}
}

func TestAmp_SampleSymbols_Deterministic(t *testing.T) {
	a := codeindex.SampleSymbols(pool(20), 5, nil)
	b := codeindex.SampleSymbols(pool(20), 5, nil)
	if len(a) != len(b) {
		t.Fatalf("length differs %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Fatalf("position %d differs: %s vs %s", i, a[i].Name, b[i].Name)
		}
	}
}

func TestAmp_SampleSymbols_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range codeindex.SampleSymbols(pool(20), 10, []string{"a", "b"}) {
		if seen[s.Name] {
			t.Fatalf("duplicate %s", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestAmp_SampleSymbols_DoesNotMutatePool(t *testing.T) {
	p := pool(10)
	before := make([]string, len(p))
	for i := range p {
		before[i] = p[i].Name
	}
	codeindex.SampleSymbols(p, 4, nil)
	for i := range p {
		if p[i].Name != before[i] {
			t.Fatalf("pool mutated at %d", i)
		}
	}
}

func TestAmp_Resolve_UnknownNameErrors(t *testing.T) {
	_, err := codeindex.Resolve([]string{"nope"}, map[string]codeindex.Factory{})
	if err == nil {
		t.Fatal("want error for unknown backend")
	}
}

func TestAmp_Resolve_EmptyNamesGivesNoBackends(t *testing.T) {
	got, err := codeindex.Resolve(nil, map[string]codeindex.Factory{})
	if err == nil && len(got) != 0 {
		t.Fatalf("want none, got %d", len(got))
	}
}

func TestAmp_KnownBackends_SortedAndComplete(t *testing.T) {
	f := func() (codeindex.Backend, error) { return nil, nil }
	got := codeindex.KnownBackends(map[string]codeindex.Factory{"zeta": f, "alpha": f, "mid": f})
	if len(got) != 3 {
		t.Fatalf("want 3, got %v", got)
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("not sorted: %v", got)
	}
}

func TestAmp_KnownBackends_EmptyMap(t *testing.T) {
	if got := codeindex.KnownBackends(nil); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestAmp_Table_EmptyReportDoesNotPanicAndShowsLoad(t *testing.T) {
	out := codeindex.BenchReport{LoadAvg: 1.5}.Table()
	if !strings.Contains(out, "1.5") {
		t.Fatalf("load average missing: %q", out)
	}
}

func TestAmp_Table_ShowsBackendAndColumns(t *testing.T) {
	out := codeindex.BenchReport{Rows: []codeindex.BenchRow{{Backend: "gotypes", Query: codeindex.QueryCallers, TP: 1, Precision: 1, Recall: 0.5, Failures: 2}}}.Table()
	for _, want := range []string{"gotypes", "callers"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestAmp_RunBench_NoBackendsNoSymbolsNoRows(t *testing.T) {
	rep, err := codeindex.RunBench(context.Background(), codeindex.BenchRun{LoadAvg: func() float64 { return 0 }})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.Rows) != 0 {
		t.Fatalf("want no rows, got %d", len(rep.Rows))
	}
}

func TestAmp_RunBench_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = codeindex.RunBench(ctx, codeindex.BenchRun{Symbols: pool(2), Rounds: 1, LoadAvg: func() float64 { return 0 }})
}

func TestAmp_GrepCheck_MissingRootErrors(t *testing.T) {
	err := codeindex.GrepCheck(t.TempDir()+"/absent", codeindex.BenchSymbol{Name: "X", DefFile: "x.go", DefLine: 1, Sites: []codeindex.TruthSite{{File: "y.go", Line: 3, Func: "F"}}, GrepFiles: []string{"y.go"}})
	if err == nil {
		t.Fatal("want error for missing root")
	}
}
