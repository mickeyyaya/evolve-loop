package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type oneLineThenErrorReader struct{ sent bool }

func (r *oneLineThenErrorReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("reader broke")
	}
	r.sent = true
	return copy(p, "{}\n"), nil
}

var characterizationStream = strings.Join([]string{
	`{"Action":"start","Package":"x/c"}`,
	`{"Action":"run","Package":"x/b","Test":"TestB"}`,
	`{"Action":"run","Package":"x/a","Test":"TestA"}`,
	`{"Action":"run","Package":"","Test":"TestNoPkg"}`,
	`{"Action":"output","Package":"x/b","Output":"` + strings.Repeat("o", 100*1024) + `"}`,
	`{"Action":"pass","Package":"example.com/m/p1","Test":"TestTieA","Elapsed":2.5}`,
	`{"Action":"pass","Package":"example.com/m/p1","Test":"TestTieB","Elapsed":2.5}`,
	`{"Action":"fail","Package":"example.com/m/p1","Test":"TestFail","Elapsed":1.0}`,
	`{"Action":"fail","Package":"example.com/m/p1","Elapsed":6.25}`,
	`{"Action":"pass","Package":"x/thr","Test":"TestThr","Elapsed":0.5}`,
	`{"Action":"pass","Package":"x/thr","Elapsed":5.0}`,
}, "\n")

func TestParseCharacterization_StatusesTiesAndSortedIncomplete(t *testing.T) {
	for i := 0; i < 20; i++ {
		rep, err := Parse(strings.NewReader(characterizationStream))
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"x/a", "x/b", "x/c"}; !reflect.DeepEqual(rep.Incomplete, want) {
			t.Fatalf("Incomplete %q, want %q", rep.Incomplete, want)
		}
		if p1 := rep.Packages[0]; p1.Pkg != "example.com/m/p1" || p1.Status != "fail" || p1.SlowestTest != "TestTieA" {
			t.Fatalf("slowest package %+v", p1)
		}
		if thr := rep.Packages[1]; thr.Pkg != "x/thr" || thr.Status != "pass" {
			t.Fatalf("second package %+v", thr)
		}
	}
}

func TestParseCharacterization_ScanErrorIsWrapped(t *testing.T) {
	_, err := Parse(&oneLineThenErrorReader{})
	if err == nil || err.Error() != "scan test2json stream: reader broke" {
		t.Errorf("err %v, want scan test2json stream: reader broke", err)
	}
}

func TestMarkdownCharacterization_Golden(t *testing.T) {
	rep, err := Parse(strings.NewReader(characterizationStream))
	if err != nil {
		t.Fatal(err)
	}
	got := rep.Markdown(MarkdownOptions{Title: "T", Top: 2, ThresholdPkg: 5, ThresholdTst: 1})
	want := "# T\n\n" +
		"- Packages: **5**\n" +
		"- Top-level tests: **4**\n" +
		"- Aggregate package wall time: **11.2s** (sum of parallel-aware per-package times)\n" +
		"- Fully-serial upper bound (Σ test elapsed): **6.5s**\n\n" +
		"> ⚠ **3 package(s) had no terminal summary** (truncated stream / panic / timeout) — their wall time is missing and they are NOT counted above: x/a, x/b, x/c\n\n" +
		"## Slow packages (> 5.0s wall) — optimization targets\n\n" +
		"| Package | Wall (s) | Tests | Σserial (s) | Slowest test | Slowest (s) |\n" +
		"|---|--:|--:|--:|---|--:|\n" +
		"| example.com/m/p1 | 6.25 | 3 | 6.00 | TestTieA | 2.50 |\n\n" +
		"## Slowest 2 tests\n\n" +
		"| Test | Package | Elapsed (s) |\n|---|---|--:|\n" +
		"| TestTieA | example.com/m/p1 | 2.50 |\n" +
		"| TestTieB | example.com/m/p1 | 2.50 |\n\n" +
		"_2 tests exceed the 1.0s per-test threshold._\n"
	if got != want {
		t.Errorf("markdown:\n%s\nwant:\n%s", got, want)
	}
}

func TestMarkdownCharacterization_CompleteRunHasNoIncompleteWarning(t *testing.T) {
	rep, err := Parse(strings.NewReader(`{"Action":"pass","Package":"x/q","Elapsed":0.1}`))
	if err != nil {
		t.Fatal(err)
	}
	if md := rep.Markdown(MarkdownOptions{Title: "C", Top: 1, ThresholdPkg: 5, ThresholdTst: 1}); strings.Contains(md, "no terminal summary") {
		t.Errorf("complete report warns about incomplete packages:\n%s", md)
	}
}
