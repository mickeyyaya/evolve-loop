//go:build acs

package cycle2633

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

type callKind string

const (
	kindStatic    callKind = "static"
	kindInterface callKind = "interface"
	kindFuncValue callKind = "funcvalue"
)

type fixtureSource struct {
	repo       string
	fset       *token.FileSet
	files      map[string]*ast.File
	topFuncs   map[string]bool
	ifaceNames map[string]bool
}

func materialize(t *testing.T) fixtureSource {
	t.Helper()
	repo := t.TempDir()
	if err := codeindex.WriteIndependentFixture(repo); err != nil {
		t.Fatalf("WriteIndependentFixture: %v", err)
	}
	src := fixtureSource{repo: repo, fset: token.NewFileSet(), files: map[string]*ast.File{}, topFuncs: map[string]bool{}, ifaceNames: map[string]bool{}}
	err := filepath.WalkDir(filepath.Join(repo, "go"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		parsed, perr := parser.ParseFile(src.fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(repo, path)
		src.files[filepath.ToSlash(rel)] = parsed
		src.collect(parsed)
		return nil
	})
	if err != nil {
		t.Fatalf("parse the fixture: %v", err)
	}
	return src
}

func (s fixtureSource) collect(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Recv == nil {
				s.topFuncs[node.Name.Name] = true
			}
		case *ast.InterfaceType:
			for _, m := range node.Methods.List {
				for _, name := range m.Names {
					s.ifaceNames[name.Name] = true
				}
			}
		}
		return true
	})
}

func (s fixtureSource) classify(call *ast.CallExpr) (callKind, string) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if s.ifaceNames[fun.Sel.Name] {
			return kindInterface, fun.Sel.Name
		}
		return kindStatic, fun.Sel.Name
	case *ast.Ident:
		if s.topFuncs[fun.Name] || types.Universe.Lookup(fun.Name) != nil {
			return kindStatic, fun.Name
		}
		return kindFuncValue, fun.Name
	}
	return kindFuncValue, ""
}

func bare(name string) string { return name[strings.LastIndex(name, ".")+1:] }

type siteEvidence struct {
	kind callKind
	ok   bool
}

func (s fixtureSource) verify(sym codeindex.BenchSymbol, site codeindex.TruthSite) siteEvidence {
	file, found := s.files[site.File]
	if !found {
		return siteEvidence{}
	}
	var evidence siteEvidence
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Body == nil || fn.Name.Name != bare(site.Func) {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall || s.fset.Position(call.Pos()).Line != site.Line {
				return true
			}
			kind, callee := s.classify(call)
			agrees := kind == kindFuncValue || callee == bare(sym.Name)
			if agrees && !evidence.ok {
				evidence = siteEvidence{kind: kind, ok: true}
			}
			return true
		})
	}
	return evidence
}

func independentTruth(t *testing.T) []codeindex.BenchSymbol {
	t.Helper()
	truth, err := codeindex.IndependentTruth()
	if err != nil {
		t.Fatalf("IndependentTruth: %v", err)
	}
	if len(truth) == 0 {
		t.Fatal("RED: the independent truth set has no symbols")
	}
	return truth
}

func TestC2633_001_EveryHandCheckedSiteIsARealCallInItsNamedCaller(t *testing.T) {
	src := materialize(t)
	for _, sym := range independentTruth(t) {
		if len(sym.Sites) == 0 {
			t.Errorf("RED: %s has no sites", sym.Name)
		}
		for _, site := range sym.Sites {
			if !src.verify(sym, site).ok {
				t.Errorf("RED: %s site %s:%d (%s) is not a call of it inside that caller, read from the fixture source with go/ast", sym.Name, site.File, site.Line, site.Func)
			}
		}
	}
}

func TestC2633_002_TheSetCoversInterfaceMethodCallsFuncValueCallsAndStaticCalls(t *testing.T) {
	src := materialize(t)
	seen := map[callKind]int{}
	for _, sym := range independentTruth(t) {
		for _, site := range sym.Sites {
			seen[src.verify(sym, site).kind]++
		}
	}
	for _, kind := range []callKind{kindInterface, kindFuncValue, kindStatic} {
		if seen[kind] == 0 {
			t.Errorf("RED: no %s call site in the independent set (seen %v)", kind, seen)
		}
	}
}

func TestC2633_003_TheSetIsNotTheGoTypesTruthRestated(t *testing.T) {
	src := materialize(t)
	if err := os.MkdirAll(filepath.Join(src.repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(t.TempDir(), "index.gob")
	if err := gotypes.Build(context.Background(), filepath.Join(src.repo, "go"), indexPath); err != nil {
		t.Fatalf("gotypes.Build of the fixture: %v", err)
	}
	derived, err := gotypes.Truth(indexPath)
	if err != nil {
		t.Fatalf("gotypes.Truth: %v", err)
	}
	derivedSites := map[string]map[string]bool{}
	for _, sym := range derived {
		derivedSites[sym.Name] = map[string]bool{}
		for _, site := range sym.Sites {
			derivedSites[sym.Name][site.File+"\x00"+site.Func] = true
		}
	}
	shared, onlyIndependent := 0, 0
	for _, sym := range independentTruth(t) {
		known, ok := derivedSites[sym.Name]
		if !ok {
			continue
		}
		shared++
		for _, site := range sym.Sites {
			if !known[site.File+"\x00"+site.Func] {
				onlyIndependent++
			}
		}
	}
	if shared == 0 {
		t.Error("RED: no independent symbol is named the way gotypes names it, so no backend can be asked about it")
	}
	if onlyIndependent == 0 {
		t.Error("RED: every independent site is already in the go/types truth: the set is circular")
	}
}

func answering(name string, answers map[string][]codeindex.Hit) codeindextest.Fake {
	index := map[codeindex.Request][]codeindex.Hit{}
	for symbol, hits := range answers {
		index[codeindex.Request{Query: codeindex.QueryCallers, Arg: symbol, Direction: codeindex.DirectionIn}] = hits
	}
	return codeindextest.NewFake(name, codeindex.AllCapabilities(), index)
}

func benchRun(label string, symbols []codeindex.BenchSymbol, backends ...codeindex.Backend) codeindex.BenchRun {
	tick := time.Unix(0, 0)
	return codeindex.BenchRun{TruthSet: label, Backends: backends, Symbols: symbols, Rounds: 1, LoadAvg: func() float64 { return 0.5 }, Now: func() time.Time {
		tick = tick.Add(time.Millisecond)
		return tick
	}}
}

var oneSite = codeindex.TruthSite{File: "a.go", Line: 3, Func: "p.A"}

func symbolWithOneSite(name string) codeindex.BenchSymbol {
	return codeindex.BenchSymbol{Name: name, Sites: []codeindex.TruthSite{oneSite}}
}

func exactAnswer(symbol string) map[string][]codeindex.Hit {
	return map[string][]codeindex.Hit{symbol: {{File: oneSite.File, Line: oneSite.Line, Symbol: oneSite.Func}}}
}

func mustBench(t *testing.T, run codeindex.BenchRun) codeindex.BenchReport {
	t.Helper()
	report, err := codeindex.RunBench(context.Background(), run)
	if err != nil {
		t.Fatalf("RunBench(%q): %v", run.TruthSet, err)
	}
	return report
}

func TestC2633_004_EveryRowCarriesTheNameOfTheTruthSetItWasScoredOn(t *testing.T) {
	first := answering("one", exactAnswer("p.F"))
	second := answering("two", nil)

	report := mustBench(t, benchRun("independent", []codeindex.BenchSymbol{symbolWithOneSite("p.F")}, first, second))

	if len(report.Rows) != 2 {
		t.Fatalf("RED: rows = %d, want one per backend", len(report.Rows))
	}
	for _, row := range report.Rows {
		if row.TruthSet != "independent" {
			t.Errorf("RED: row %s has TruthSet %q, want independent", row.Backend, row.TruthSet)
		}
	}
}

func TestC2633_005_AScoreOnOneSetNeverBleedsIntoTheOther(t *testing.T) {
	backend := answering("one", exactAnswer("p.F"))
	derived := mustBench(t, benchRun("go-types", []codeindex.BenchSymbol{symbolWithOneSite("p.F")}, backend))
	independent := mustBench(t, benchRun("independent", []codeindex.BenchSymbol{symbolWithOneSite("p.Missed")}, backend))

	if got := derived.Rows[0]; got.TruthSet != "go-types" || got.Recall != 1 || got.Precision != 1 {
		t.Errorf("RED: go-types row = %+v, want recall and precision 1", got)
	}
	if got := independent.Rows[0]; got.TruthSet != "independent" || got.Recall != 0 || got.FN != 1 {
		t.Errorf("RED: independent row = %+v, want recall 0 and one missed caller", got)
	}
}

func TestC2633_006_AnEmptySetStillGivesOneZeroedRowPerServingBackendAndNoneForTheOthers(t *testing.T) {
	serving := answering("serving", nil)
	findOnly := codeindextest.NewFake("findonly", codeindex.Capabilities{Find: true}, nil)

	report := mustBench(t, benchRun("empty", nil, serving, findOnly))

	if len(report.Rows) != 1 {
		t.Fatalf("RED: rows = %+v, want one row for the serving backend and none for the find-only one", report.Rows)
	}
	row := report.Rows[0]
	if row.TruthSet != "empty" || row.Backend != "serving" || row.TP+row.FP+row.FN != 0 || row.Precision != 0 || row.Recall != 0 {
		t.Errorf("RED: row = %+v, want a labelled zero row", row)
	}
}

func TestC2633_007_TheTablePrintsTheSetPrecisionRecallAndPercentilesOfEveryRow(t *testing.T) {
	backend := answering("one", exactAnswer("p.F"))
	derived := mustBench(t, benchRun("go-types", []codeindex.BenchSymbol{symbolWithOneSite("p.F")}, backend))
	independent := mustBench(t, benchRun("independent", []codeindex.BenchSymbol{symbolWithOneSite("p.Missed")}, backend))
	report := codeindex.BenchReport{Rows: slices.Concat(derived.Rows, independent.Rows), LoadAvg: derived.LoadAvg}

	lines := strings.Split(report.Table(), "\n")

	header := strings.Fields(lines[0])
	for i, column := range []string{"set", "backend", "query", "precision", "recall", "p50", "p95"} {
		if len(header) <= i || header[i] != column {
			t.Fatalf("RED: header = %v, want it to start set backend query precision recall p50 p95", header)
		}
	}
	wantRows := map[string][]string{
		"go-types":    {"one", "callers", "1.00", "1.00", "1ms", "1ms"},
		"independent": {"one", "callers", "0.00", "0.00", "1ms", "1ms"},
	}
	for set, want := range wantRows {
		var got []string
		for _, line := range lines[1:] {
			if fields := strings.Fields(line); len(fields) > 6 && fields[0] == set {
				got = fields[1:7]
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("RED: %s row = %v, want %v", set, got, want)
		}
	}
}

func TestC2633_008_TheBenchVerbScoresEveryBackendOnBothTruthSets(t *testing.T) {
	names := []string{
		"TestCodeBench_IndependentTruth_EveryBackendIsScoredOnBothTruthSets",
		"TestCodeBench_IndependentTruth_TheHeaderNamesTheSetAndThePercentiles",
		"TestCodeBench_IndependentTruth_GotypesIsNotPerfectOnTheSetItDidNotProduce",
		"TestCodeBench_IndependentTruth_AnAbsentIndexStillFailsBeforeAnySetIsPrinted",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: cmdEvolvePkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}
