package main

import (
	"strings"
	"testing"
)

// A minimal hand-written test2json stream: two packages, a package-level
// summary each, a subtest that must be excluded, and a skip.
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
	// Packages sorted by wall time desc → bridge first.
	bridge := rep.Packages[0]
	if !strings.HasSuffix(bridge.Pkg, "/bridge") {
		t.Fatalf("slowest package = %s, want bridge", bridge.Pkg)
	}
	if bridge.Wall != 12.7 {
		t.Errorf("bridge wall = %.2f, want 12.7", bridge.Wall)
	}
	// TestSlow + TestFast counted; the "/sub" subtest excluded.
	if bridge.NumTests != 2 {
		t.Errorf("bridge NumTests = %d, want 2 (subtest excluded)", bridge.NumTests)
	}
	if bridge.SlowestTest != "TestSlow" || bridge.SlowestSecs != 12.5 {
		t.Errorf("bridge slowest = %s/%.2f, want TestSlow/12.50", bridge.SlowestTest, bridge.SlowestSecs)
	}
	// Serial sum is top-level only: 12.5 + 0.1.
	if bridge.SerialSum < 12.59 || bridge.SerialSum > 12.61 {
		t.Errorf("bridge SerialSum = %.2f, want ~12.6", bridge.SerialSum)
	}
}

// TestParse_FlagsIncompletePackage proves a package that emits test events but
// no terminal pass/fail/skip summary (truncated stream / panic / timeout) is
// recorded as Incomplete and surfaced in the Markdown, not silently dropped.
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
	// budget (0.4s wall) is under the 5s package threshold; its package-table
	// row "| internal/budget | 0.40 |" must be absent (it may still appear in
	// the per-test table, which is fine).
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
