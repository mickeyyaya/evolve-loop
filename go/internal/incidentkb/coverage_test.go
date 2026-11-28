package incidentkb_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

const coverageHead = "# Index\n\n## Summary\n\n<!-- GENERATED:coverage-summary BEGIN -->\nstale\n<!-- GENERATED:coverage-summary END -->\n\n## Coverage map\n\n| Incident | Failure mode | Suspect file | Coverage | Pinning test / proposal |\n|---|---|---|---|---|\n"

const coverageTail = "\n## Prioritized gap backlog\n\n| [not a map row](x.md) | a | b | ✅ | c |\n"

func coverageDoc(rows ...string) []byte {
	return []byte(coverageHead + strings.Join(rows, "\n") + "\n" + coverageTail)
}

func TestCoverageCountsOf_CountsEachStatusAndEachLinkedNameOnce(t *testing.T) {
	t.Parallel()
	doc := coverageDoc(
		"| [2026-08-10 absorbing-FAIL](a.md) | mode 1 | `x.go` | ✅ | `TestA` |",
		"| [2026-08-10 absorbing-FAIL](a.md) | mode 2 | `x.go` | 🟡 | `TestB` |",
		"| [cycle-31](a.md) | mode 3 | `y.go` | ❌ | item `z` |",
		"| [2026-11-20 — the TDD floor](b.md) | a `tdd\\|gate-block` key | `z.go` | ⛔ | none |",
		"| [2026-11-28 — a third](c.md) | mode 5 | `w.go` | ✅ | `TestC` |",
		"| [2026-11-28 — a third](c.md) ([ADR-0090](../adr.md)) | mode 6 | `w.go` | ✅ | `TestD` |",
	)
	got, err := incidentkb.CoverageCountsOf(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := incidentkb.CoverageCounts{Incidents: 4, Rows: 6, Covered: 3, Partial: 1, None: 1, Untestable: 1}
	if got != want {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
}

func TestCoverageCountsOf_RefusesARowThatDoesNotLinkItsIncident(t *testing.T) {
	t.Parallel()
	doc := coverageDoc(
		"| [2026-10-06 — cycle 1811: a lane relocated a sibling's eval](a.md) | mode | `x.go` | ✅ | `TestA` |",
		"| 2026-10-06 — cycle 1811 | mode 2 | `x.go` | ✅ | `TestB` |",
	)
	_, err := incidentkb.CoverageCountsOf(doc)
	var rowErr *incidentkb.CoverageRowError
	if !errors.Is(err, incidentkb.ErrCoverageRow) || !errors.As(err, &rowErr) || rowErr.Line != 14 {
		t.Fatalf("a plain incident cell must fail with its line number 14, got %v", err)
	}
	if out, _, err := incidentkb.RenderCoverageSummary(doc); !errors.Is(err, incidentkb.ErrCoverageRow) || out != nil {
		t.Fatalf("render must refuse the same row and write nothing: %d bytes, %v", len(out), err)
	}
}

func TestCoverageCountsOf_RefusesAnUnknownCoverageCellAndAWrongCellCount(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]string{
		"unknown status":      "| [n](a.md) | mode | `x.go` | done | `TestA` |",
		"unescaped pipe":      "| [n](a.md) | a `tdd|gate` key | `x.go` | ✅ | `TestA` |",
		"conflict marker":     "<<<<<<< HEAD",
		"empty coverage cell": "| [n](a.md) | mode | `x.go` |  | `TestA` |",
		"link not first":      "| see [n](a.md) | mode | `x.go` | ✅ | `TestA` |",
		"too many cells":      "| [n](a.md) | mode | `x.go` | ✅ | `TestA` | extra |",
		"too few cells":       "| [n](a.md) | mode | ✅ |",
		"empty incident cell": "|  | mode | `x.go` | ✅ | `TestA` |",
	} {
		_, err := incidentkb.CoverageCountsOf(coverageDoc(row))
		var rowErr *incidentkb.CoverageRowError
		if !errors.Is(err, incidentkb.ErrCoverageRow) || !errors.As(err, &rowErr) || rowErr.Line != 13 || rowErr.Reason == nil {
			t.Errorf("%s: want a CoverageRowError on line 13 with a reason, got %v", name, err)
		}
	}
}

func TestCoverageCountsOf_AnEqualsRuleThatIsNotAConflictMarkerIsProse(t *testing.T) {
	t.Parallel()
	doc := coverageDoc("| [one](a.md) | mode | `x.go` | ✅ | `TestA` |", "", "========", "", "| [two](b.md) | mode | `y.go` | ❌ | item `z` |")
	if counts, err := incidentkb.CoverageCountsOf(doc); err != nil || counts.Rows != 2 {
		t.Fatalf("an eight-character equals line is prose, not a conflict marker: %+v, %v", counts, err)
	}
}

func TestCoverageCountsOf_RefusesADocumentWithoutTheMap(t *testing.T) {
	t.Parallel()
	if _, err := incidentkb.CoverageCountsOf([]byte("# Index\n\n## Summary\n")); !errors.Is(err, incidentkb.ErrCoverageRow) {
		t.Fatalf("a document without the coverage map must fail, got %v", err)
	}
}

func TestRenderCoverageSummary_RewritesOnlyTheRegion(t *testing.T) {
	t.Parallel()
	doc := coverageDoc(
		"| [one](a.md) | mode | `x.go` | ✅ | `TestA` |",
		"| [two](b.md) | mode | `y.go` | ❌ | item `z` |",
	)
	got, counts, err := incidentkb.RenderCoverageSummary(doc)
	if err != nil || counts != (incidentkb.CoverageCounts{Incidents: 2, Rows: 2, Covered: 1, None: 1}) {
		t.Fatalf("render counts = %+v, %v", counts, err)
	}
	table := "| | Count |\n|---|---|\n| Incidents mapped | 2 |\n| Distinct failure modes (coverage-map rows) | 2 |\n" +
		"| ✅ covered (a test fails if the bug comes back) | 1 |\n| 🟡 partial (related test, doesn't pin the exact mode) | 0 |\n" +
		"| ❌ none (no regression test) | 1 |\n| ⛔ untestable (external infra / live CLI) | 0 |"
	want := strings.Replace(string(doc), "\nstale\n", "\n"+table+"\n", 1)
	if string(got) != want {
		t.Fatalf("render =\n%s\nwant\n%s", got, want)
	}
	again, _, err := incidentkb.RenderCoverageSummary(got)
	if err != nil || string(again) != string(got) {
		t.Fatalf("a second render must be a fixed point: %v", err)
	}
}

func TestRenderCoverageSummary_RefusesADocumentWithoutTheRegion(t *testing.T) {
	t.Parallel()
	doc := []byte(strings.Replace(string(coverageDoc("| [one](a.md) | mode | `x.go` | ✅ | `TestA` |")), "<!-- GENERATED:coverage-summary END -->", "", 1))
	if _, _, err := incidentkb.RenderCoverageSummary(doc); !errors.Is(err, incidentkb.ErrMissingMarker) {
		t.Fatalf("a missing region end must fail with ErrMissingMarker, got %v", err)
	}
}

func TestRenderCoverageSummary_AConflictOnlyInTheRegionResolvesToTheRowsOfBothSides(t *testing.T) {
	t.Parallel()
	merged := coverageHead[:strings.Index(coverageHead, "stale")] +
		"<<<<<<< ours\n| Incidents mapped | 2 |\n=======\n| Incidents mapped | 2 |\n| x | 1 |\n>>>>>>> theirs\n" +
		coverageHead[strings.Index(coverageHead, "<!-- GENERATED:coverage-summary END -->"):] +
		"| [base](a.md) | mode | `x.go` | ✅ | `TestA` |\n| [ours](b.md) | mode | `y.go` | ❌ | item `y` |\n\n" +
		"Prose between the rows.\n\n| [theirs](c.md) | mode | `z.go` | 🟡 | `TestC` |\n" + coverageTail
	if !derived.IsDerivedConflict(incidentkb.CoverageIndexRel, []byte(merged)) {
		t.Fatal("a conflict only inside the summary region must be a derived conflict")
	}
	got, _, err := incidentkb.RenderCoverageSummary([]byte(merged))
	if err != nil {
		t.Fatal(err)
	}
	counts, err := incidentkb.CoverageCountsOf(got)
	if err != nil || counts.Rows != 3 || counts.Incidents != 3 || strings.Contains(string(got), "<<<<<<<") {
		t.Fatalf("the regenerated summary must count the rows of both sides: %+v, %v", counts, err)
	}
}

func TestCoverageIndexRel_IsTheOutputOfTheDerivedEntryAndTheSkippedIndex(t *testing.T) {
	t.Parallel()
	entry, ok := derived.Lookup("coverage-summary")
	if !ok || len(entry.Outputs) != 1 || entry.Outputs[0].Pattern != incidentkb.CoverageIndexRel || entry.Outputs[0].Region != "GENERATED:coverage-summary" {
		t.Fatalf("derived entry coverage-summary = %+v (found %v), want one output %s", entry, ok, incidentkb.CoverageIndexRel)
	}
	if !incidentkb.IsSkipped(incidentkb.CoverageIndexRel) || incidentkb.IsRecordFile(incidentkb.CoverageIndexRel) {
		t.Fatal("the coverage index is not an incident record")
	}
}
