package main

import (
	"errors"
	"strings"
	"testing"
)

const sample = `
{"Action":"run","Package":"github.com/mickeyyaya/evolve-loop/go/internal/bridge","Test":"TestSlow"}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/bridge","Test":"TestSlow","Elapsed":12.5}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/bridge","Test":"TestSlow/sub","Elapsed":12.4}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/bridge","Test":"TestFast","Elapsed":0.1}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/bridge","Elapsed":12.7}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Test":"TestTiny","Elapsed":0.01}
{"Action":"skip","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Test":"TestSkipped","Elapsed":0}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Elapsed":0.4}
not-json build noise
`

func TestParse_AggregatesAndExcludesSubtests(t *testing.T) {
	rep, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rep.Packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(rep.Packages))
	}
	bridge := rep.Packages[0]
	if !strings.HasSuffix(bridge.Pkg, "/bridge") {
		t.Fatalf("slowest package = %s, want bridge", bridge.Pkg)
	}
	if bridge.Wall != 12.7 {
		t.Errorf("bridge wall = %.2f, want 12.7", bridge.Wall)
	}
	if bridge.NumTests != 2 {
		t.Errorf("bridge NumTests = %d, want 2 (subtest excluded)", bridge.NumTests)
	}
	if bridge.SlowestTest != "TestSlow" || bridge.SlowestSecs != 12.5 {
		t.Errorf("bridge slowest = %s/%.2f, want TestSlow/12.50", bridge.SlowestTest, bridge.SlowestSecs)
	}
	if bridge.SerialSum < 12.59 || bridge.SerialSum > 12.61 {
		t.Errorf("bridge SerialSum = %.2f, want ~12.6", bridge.SerialSum)
	}
}

func TestParse_FlagsIncompletePackage(t *testing.T) {
	stream := `
{"Action":"run","Package":"x/crashy","Test":"TestBoom"}
{"Action":"output","Package":"x/crashy","Test":"TestBoom","Output":"panic: boom\n"}
{"Action":"pass","Package":"x/ok","Test":"TestFine","Elapsed":0.2}
{"Action":"pass","Package":"x/ok","Elapsed":0.3}
`
	rep, err := Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rep.Incomplete) != 1 || rep.Incomplete[0] != "x/crashy" {
		t.Fatalf("Incomplete = %v, want [x/crashy]", rep.Incomplete)
	}
	md := rep.Markdown(MarkdownOptions{Title: "t", ThresholdPkg: 5, ThresholdTst: 1, Top: 10})
	if !strings.Contains(md, "no terminal summary") || !strings.Contains(md, "x/crashy") {
		t.Errorf("Markdown missing incomplete-package note:\n%s", md)
	}
}

func TestMarkdown_FlagsSlowPackagesAndTests(t *testing.T) {
	rep, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 10, ThresholdPkg: 5.0, ThresholdTst: 1.0})
	if !strings.Contains(md, "internal/bridge") {
		t.Error("report should flag the slow bridge package")
	}
	if strings.Contains(md, "internal/budget | 0.40") {
		t.Error("budget (0.4s) is under the 5s package threshold; should not be flagged as a slow package")
	}
	if !strings.Contains(md, "1 tests exceed the 1.0s per-test threshold") {
		t.Errorf("expected exactly 1 test over per-test threshold:\n%s", md)
	}
	if strings.Contains(md, "github.com/mickeyyaya") {
		t.Error("module prefix should be trimmed in the report via shortPkg")
	}
}

func TestParse_ToleratesMalformedJSONLine(t *testing.T) {
	const stream = `
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Test":"TestA","Elapsed":0.2}
{"Action":"pass","Package": not-valid-json
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Elapsed":0.5}
`
	rep, err := Parse(strings.NewReader(stream))

	if err != nil {
		t.Fatalf("Parse should tolerate a malformed line, got err: %v", err)
	}
	if len(rep.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(rep.Packages))
	}
	if rep.Packages[0].NumTests != 1 {
		t.Errorf("NumTests = %d, want 1 (only TestA is a valid top-level test)", rep.Packages[0].NumTests)
	}
	if rep.Packages[0].Wall != 0.5 {
		t.Errorf("Wall = %.2f, want 0.50 (package summary event)", rep.Packages[0].Wall)
	}
}

type failingReader struct {
	data []byte
	done bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		n := copy(p, r.data)
		return n, nil
	}
	return 0, errReadBoom
}

var errReadBoom = errors.New("boom")

func TestParse_ScannerError_Propagates(t *testing.T) {
	r := &failingReader{data: []byte(`{"Action":"pass","Package":"p","Elapsed":1}` + "\n")}

	rep, err := Parse(r)

	if err == nil {
		t.Fatalf("Parse should propagate the scanner read error, got nil (rep=%+v)", rep)
	}
	if !errors.Is(err, errReadBoom) {
		t.Errorf("error chain = %v, want to wrap errReadBoom", err)
	}
	if rep != nil {
		t.Errorf("rep = %+v, want nil on error", rep)
	}
}

func TestMarkdown_NoSlowPackages_RendersNonePlaceholder(t *testing.T) {
	const fastOnly = `
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Test":"TestTiny","Elapsed":0.01}
{"Action":"pass","Package":"github.com/mickeyyaya/evolve-loop/go/internal/budget","Elapsed":0.4}
`
	rep, err := Parse(strings.NewReader(fastOnly))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 10, ThresholdPkg: 5.0, ThresholdTst: 1.0})

	if !strings.Contains(md, "| _(none)_ | | | | | |") {
		t.Errorf("expected the no-slow-packages placeholder row; got:\n%s", md)
	}
	if !strings.Contains(md, "0 tests exceed the 1.0s per-test threshold") {
		t.Errorf("expected zero tests over per-test threshold; got:\n%s", md)
	}
}

func TestMarkdown_TopCap_TruncatesSlowestTable(t *testing.T) {
	rep, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 1, ThresholdPkg: 5.0, ThresholdTst: 1.0})

	if !strings.Contains(md, "| TestSlow | internal/bridge | 12.50 |") {
		t.Errorf("slowest test row TestSlow missing; got:\n%s", md)
	}
	if strings.Contains(md, "| TestFast |") {
		t.Errorf("Top=1 must truncate TestFast from the slowest-tests table; got:\n%s", md)
	}
}
