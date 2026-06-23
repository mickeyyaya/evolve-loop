package main

import (
	"errors"
	"strings"
	"testing"
)

// A minimal hand-written test2json stream: two packages, a package-level
// summary each, a subtest that must be excluded, and a skip.
const sample = `
{"Action":"run","Package":"github.com/mickeyyaya/evolveloop/go/internal/bridge","Test":"TestSlow"}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/bridge","Test":"TestSlow","Elapsed":12.5}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/bridge","Test":"TestSlow/sub","Elapsed":12.4}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/bridge","Test":"TestFast","Elapsed":0.1}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/bridge","Elapsed":12.7}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Test":"TestTiny","Elapsed":0.01}
{"Action":"skip","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Test":"TestSkipped","Elapsed":0}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Elapsed":0.4}
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

// TestParse_FlagsIncompletePackage proves a package that starts but emits no
// terminal pass/fail/skip summary (truncated stream / panic / timeout) is
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

// A line that begins with '{' but is not valid JSON must be tolerated and
// skipped (test2json streams can interleave malformed/truncated fragments),
// while the surrounding valid events are still aggregated.
func TestParse_ToleratesMalformedJSONLine(t *testing.T) {
	// Arrange: a valid event, a '{'-prefixed but broken line, another valid event.
	const stream = `
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Test":"TestA","Elapsed":0.2}
{"Action":"pass","Package": not-valid-json
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Elapsed":0.5}
`
	// Act
	rep, err := Parse(strings.NewReader(stream))

	// Assert: parse succeeds; the broken line is dropped, the two valid events kept.
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

// failingReader yields one line of valid data, then a non-EOF read error,
// so Parse exercises the sc.Err() failure path deterministically (no I/O).
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

// Parse must surface a stream read error rather than silently returning a
// partial report — the scanner error path is the only place a malformed pipe
// can be reported to the caller.
func TestParse_ScannerError_Propagates(t *testing.T) {
	// Arrange: a reader that returns data then fails.
	r := &failingReader{data: []byte(`{"Action":"pass","Package":"p","Elapsed":1}` + "\n")}

	// Act
	rep, err := Parse(r)

	// Assert
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

// When no package exceeds the wall-time threshold, the slow-packages table
// must render the explicit "_(none)_" placeholder row rather than an empty
// table body — that placeholder is the signal a reader scans for.
func TestMarkdown_NoSlowPackages_RendersNonePlaceholder(t *testing.T) {
	// Arrange: a fast suite (budget package only, 0.4s wall < 5s threshold).
	const fastOnly = `
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Test":"TestTiny","Elapsed":0.01}
{"Action":"pass","Package":"github.com/mickeyyaya/evolveloop/go/internal/budget","Elapsed":0.4}
`
	rep, err := Parse(strings.NewReader(fastOnly))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Act
	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 10, ThresholdPkg: 5.0, ThresholdTst: 1.0})

	// Assert
	if !strings.Contains(md, "| _(none)_ | | | | | |") {
		t.Errorf("expected the no-slow-packages placeholder row; got:\n%s", md)
	}
	if !strings.Contains(md, "0 tests exceed the 1.0s per-test threshold") {
		t.Errorf("expected zero tests over per-test threshold; got:\n%s", md)
	}
}

// The slowest-tests table must honour the Top cap: with Top=1 and two tests,
// only the single slowest row is rendered.
func TestMarkdown_TopCap_TruncatesSlowestTable(t *testing.T) {
	// Arrange
	rep, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Act: Top=1 forces the i >= o.Top break after the first (slowest) row.
	md := rep.Markdown(MarkdownOptions{Title: "T", Top: 1, ThresholdPkg: 5.0, ThresholdTst: 1.0})

	// Assert: the slowest test (TestSlow) is present, the next-slowest (TestFast)
	// is truncated out of the per-test table.
	if !strings.Contains(md, "| TestSlow | internal/bridge | 12.50 |") {
		t.Errorf("slowest test row TestSlow missing; got:\n%s", md)
	}
	if strings.Contains(md, "| TestFast |") {
		t.Errorf("Top=1 must truncate TestFast from the slowest-tests table; got:\n%s", md)
	}
}
