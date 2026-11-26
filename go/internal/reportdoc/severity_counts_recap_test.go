package reportdoc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	run0962Sha256 = "355ded5c1cac865de7e3aa2ea1e835ddb23f90a098b1cd98befba8c08ee9d7ab"
	run2531Sha256 = "3290815c3686ea4fd1934d54343b58422033d7684fe41ba1c85e2c43c57a52b3"
)

func fixture(t *testing.T, name, wantSha string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != wantSha {
		t.Fatalf("%s sha256 = %s, want %s: the fixture must stay a byte copy of the recovered report", name, got, wantSha)
	}
	return string(raw)
}

func TestSeverityCounts_RecoveredRun0962CountsNoHigh(t *testing.T) {
	t.Parallel()
	report := fixture(t, "review-run-0962.md", run0962Sha256)

	got := CountSeverities(report)

	if want := (SeverityCounts{Low: 2}); got != want {
		t.Errorf("CountSeverities(run 0962) = %+v, want %+v", got, want)
	}
}

func TestSeverityCounts_ARecapSentenceDeclaresNothing(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"Round 1 asked for 1 HIGH, 7 MEDIUM and 3 LOW fixes.",
		"Round 1 fixes: 1 HIGH, 2 MEDIUM",
		"See \"## Findings\" above: 1 CRITICAL, 1 HIGH",
	} {
		if got := CountSeverities(line + "\n"); got != (SeverityCounts{}) {
			t.Errorf("CountSeverities(%q) = %+v, want no count", line, got)
		}
	}
}

func TestSeverityCounts_LabelledCountsLineStillCounts(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"label":                     "docs-review: 1 HIGH, 0 LOW\n",
		"bullet before label":       "- docs-review: 1 HIGH, 0 LOW\n",
		"numbered bullet":           "1. docs-review: 1 HIGH, 0 LOW\n",
		"no label":                  "1 HIGH, 0 LOW\n",
		"bold label":                "**docs-review:** 1 HIGH, 0 LOW\n",
		"double label":              "Counts: docs-review: 1 HIGH, 0 LOW\n",
		"bold count":                "Counts: **1 HIGH**, 0 LOW\n",
		"indented bullet only":      "  * 1 HIGH, 0 LOW\n",
		"paren bullet":              "1) docs-review: 1 HIGH, 0 LOW\n",
		"bold label, colon outside": "**Counts**: 1 HIGH, 0 LOW\n",
	}
	for name, report := range cases {
		if got := CountSeverities(report); got.High != 1 {
			t.Errorf("%s: CountSeverities(%q) = %+v, want 1 HIGH", name, report, got)
		}
	}
}

func TestSeverityCounts_ALabelledLineWithOnePairIsNotACountsLine(t *testing.T) {
	t.Parallel()
	if got := CountSeverities("Counts: 1 HIGH.\n"); got != (SeverityCounts{}) {
		t.Errorf("CountSeverities = %+v, want no count from one pair", got)
	}
}

func TestSeverityCounts_RecapWithOneOpenFindingStillCounts(t *testing.T) {
	t.Parallel()
	report := "Round 1 asked for 1 HIGH, 7 MEDIUM and 3 LOW fixes.\n- [HIGH] a.go:1 — x\n"

	if got, want := CountSeverities(report), (SeverityCounts{High: 1}); got != want {
		t.Errorf("CountSeverities = %+v, want %+v", got, want)
	}
}

func TestSeverityCounts_TableRowStillCounts(t *testing.T) {
	t.Parallel()
	if got := CountSeverities("| HIGH | 1 |\n"); got.High != 1 {
		t.Errorf("CountSeverities = %+v, want 1 HIGH", got)
	}
}

func TestSeverityCounts_RealArchitectureReportKeepsItsCounts(t *testing.T) {
	t.Parallel()
	report := fixture(t, "review-architecture-run-2531.md", run2531Sha256)

	if got, want := CountSeverities(report), (SeverityCounts{High: 2}); got != want {
		t.Errorf("CountSeverities(run 2531) = %+v, want %+v", got, want)
	}
}

func TestTraceBlocking_NamesTheLineThatSetTheLargestDeclaredCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		report string
		want   BlockingSource
	}{
		{"none", "## Findings\nNone.\n", BlockingSource{}},
		{"high declared by a counts line", "Counts: 1 HIGH, 2 MEDIUM.\n", BlockingSource{HighDeclared: 1, DeclaredLine: "Counts: 1 HIGH, 2 MEDIUM."}},
		{"the first line with the largest count wins", "docs-review: 2 HIGH, 0 LOW\nCounts: 2 HIGH, 1 LOW\n", BlockingSource{HighDeclared: 2, DeclaredLine: "docs-review: 2 HIGH, 0 LOW"}},
		{"a larger later count replaces the line", "docs-review: 1 HIGH, 0 LOW\nCounts: 3 HIGH, 1 LOW\n", BlockingSource{HighDeclared: 3, DeclaredLine: "Counts: 3 HIGH, 1 LOW"}},
		{"a table row is a declaring line", "| HIGH | 2 |\n", BlockingSource{HighDeclared: 2, DeclaredLine: "| HIGH | 2 |"}},
		{"critical wins over high", "a: 5 HIGH, 0 LOW\nb: 1 CRITICAL, 0 LOW\n", BlockingSource{CriticalDeclared: 1, HighDeclared: 5, DeclaredLine: "b: 1 CRITICAL, 0 LOW"}},
		{"high is named when no critical is declared", "a: 0 CRITICAL, 0 LOW\nb: 2 HIGH, 0 LOW\n", BlockingSource{HighDeclared: 2, DeclaredLine: "b: 2 HIGH, 0 LOW"}},
		{"listed findings carry no declared line", "- [HIGH] a.go:1 — x\n- [HIGH] a.go:2 — y\n", BlockingSource{HighListed: 2}},
		{"listed and declared both show", "- [HIGH] a.go:1 — x\nCounts: 3 HIGH, 0 LOW\n", BlockingSource{HighListed: 1, HighDeclared: 3, DeclaredLine: "Counts: 3 HIGH, 0 LOW"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TraceBlocking(tc.report); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TraceBlocking = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTraceBlocking_TrimsTheIndentAndTrailingSpaceOfTheDeclaredLine(t *testing.T) {
	t.Parallel()
	if got := TraceBlocking("  - Counts: 1 HIGH, 0 LOW  \n").DeclaredLine; got != "- Counts: 1 HIGH, 0 LOW" {
		t.Errorf("declared line = %q, want the trimmed line", got)
	}
}

func TestTraceBlocking_CutsTheLineAndRemovesControlCharacters(t *testing.T) {
	t.Parallel()
	long := "Counts: 1 HIGH, 0 LOW " + strings.Repeat("é", 300)
	if got := []rune(TraceBlocking(long + "\n").DeclaredLine); len(got) != MaxSourceLineRunes {
		t.Errorf("declared line has %d runes, want %d", len(got), MaxSourceLineRunes)
	}
	prefix := "Counts: 1 HIGH, 0 LOW "
	atLimit := prefix + strings.Repeat("a", MaxSourceLineRunes-len(prefix)+1)
	if got := []rune(TraceBlocking(atLimit + "\n").DeclaredLine); len(got) != MaxSourceLineRunes {
		t.Errorf("declared line of limit+1 runes has %d runes, want %d", len(got), MaxSourceLineRunes)
	}
	got := TraceBlocking("Counts: 1 HIGH, 0 LOW \x1b[31mred\x07\n").DeclaredLine
	if want := "Counts: 1 HIGH, 0 LOW [31mred"; got != want {
		t.Errorf("declared line = %q, want %q", got, want)
	}
}

func TestCountSeverities_AgreesWithTraceBlocking(t *testing.T) {
	t.Parallel()
	report := "- [HIGH] a.go:1 — x\n- [CRITICAL] b.go:2 — y\nCounts: 3 HIGH, 2 CRITICAL\n"

	counts, src := CountSeverities(report), TraceBlocking(report)

	if counts.High != max(src.HighListed, src.HighDeclared) || counts.Critical != max(1, src.CriticalDeclared) {
		t.Errorf("counts %+v disagree with source %+v", counts, src)
	}
}
