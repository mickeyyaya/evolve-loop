package reportdoc

import (
	"reflect"
	"testing"
)

const reportWithLocations = `## Verdict

**FAIL**

## Issues

### H1 (HIGH) — the loop swallows the error at go/internal/core/loop.go:42
body without a path

### M1 (MEDIUM) — the brief omits the predicate
Evidence: go/internal/core/brief.go:17-30 shows the gap.

### M2 (MEDIUM) — no location is named
the prose names a file go/internal/core/brief.go with no line.

## Filed (late)

### M9 (MEDIUM) — old code at docs/plans/old.md:8 keeps a stale claim

## Reflection
text
`

func TestProjectionKeepsLine(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for _, f := range Findings(reportWithLocations) {
		got[f.ID] = f.Location
	}
	want := map[string]string{
		"H1": "go/internal/core/loop.go:42",
		"M1": "go/internal/core/brief.go:17",
		"M2": "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("locations = %v, want %v", got, want)
	}
}

func TestFindings_ExcludesTheFiledLateSection(t *testing.T) {
	t.Parallel()
	for _, f := range Findings(reportWithLocations) {
		if f.ID == "M9" {
			t.Fatalf("a Filed (late) entry is read as an open finding: %+v", f)
		}
	}
}

func TestFiledLate_ReadsOnlyTheFiledSectionWithItsLocation(t *testing.T) {
	t.Parallel()
	got := FiledLate(reportWithLocations)
	want := []Finding{{ID: "M9", Severity: "MEDIUM", Title: "old code at docs/plans/old.md:8 keeps a stale claim", Location: "docs/plans/old.md:8"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FiledLate = %+v, want %+v", got, want)
	}
	if got := FiledLate("## Issues\n### H1 (HIGH) — x\n"); len(got) != 0 {
		t.Fatalf("a report with no Filed (late) section has entries: %+v", got)
	}
	for _, f := range Findings(reportWithLocations) {
		if f.ID == "M9" {
			t.Fatalf("Findings holds the filed-late entry %+v: a filed entry does not decide the verdict", f)
		}
	}
	onlyFiled := "## Verdict\nPASS\n\n## Filed (late)\n### M9 (MEDIUM) — old code at a.go:8 keeps a stale claim\n"
	if got := Findings(onlyFiled); len(got) != 0 {
		t.Fatalf("a report with filed-late entries and no Issues section has findings %+v", got)
	}
}

func TestFindings_TableRowAndTSVKeepTheTitleLocation(t *testing.T) {
	t.Parallel()
	row := "## Issues\n| M1 | MEDIUM | stale claim at a/b.go:9 |\n"
	if got := Findings(row); len(got) != 1 || got[0].Location != "a/b.go:9" {
		t.Fatalf("row = %+v", got)
	}
	tsv := "## Issues\n```tsv\nLOW\tstale claim at a/b.go:11\n```\n"
	if got := Findings(tsv); len(got) != 1 || got[0].Location != "a/b.go:11" {
		t.Fatalf("tsv = %+v", got)
	}
}

func TestLocation_TakesTheFirstPathLineAndItsFirstLineOfARange(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"see go/a/b.go:42 and go/c.go:7": "go/a/b.go:42",
		"cites docs/x.md:8-20":           "docs/x.md:8",
		"a bare file go/a/b.go":          "",
		"no citation":                    "",
		"":                               "",
	} {
		if got := Location(text); got != want {
			t.Errorf("Location(%q) = %q, want %q", text, got, want)
		}
	}
}
