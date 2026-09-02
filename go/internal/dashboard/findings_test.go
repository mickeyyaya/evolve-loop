package dashboard

import (
	"reflect"
	"testing"
)

// The auditor writes findings as `### <ID> (<SEVERITY>[, qualifier]) — <title>`
// under `## Issues`. Both heading shapes observed live (cycle-1605 final round
// and cycle-1604 round 1) must parse; the verdict may be declared as
// `## Verdict` + a bold line or inline `**Verdict: X**`.

const reportBothShapes = `# Audit Report — Cycle 1605 (round 3)

## Verdict

**FAIL**

## Issues

### H1 (HIGH) — caller-proof hard floor violated for the third consecutive round
body

### M1 (MEDIUM, narrative-fidelity) — three lead claims are measured against the salvage snapshot
body

### L1 (LOW, scope — disclosed)
body

## Reflection
### What slowed this phase (required)
not a finding
`

func TestParseFindings_BothHeadingShapes(t *testing.T) {
	t.Parallel()
	got := parseFindings(reportBothShapes)
	want := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "caller-proof hard floor violated for the third consecutive round"},
		{ID: "M1", Severity: "MEDIUM", Title: "three lead claims are measured against the salvage snapshot"},
		{ID: "L1", Severity: "LOW", Title: "scope — disclosed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFindings = %+v, want %+v", got, want)
	}
}

func TestParseFindings_NoIssuesSection(t *testing.T) {
	t.Parallel()
	if got := parseFindings("# Audit\n\n**Verdict: PASS**\n\n## Reflection\n### What slowed (required)\n"); len(got) != 0 {
		t.Fatalf("parseFindings on a PASS report = %+v, want none", got)
	}
}

func TestParseVerdict_Shapes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"## Verdict\n\n**FAIL**\n":        "FAIL",
		"## Verdict\n**PASS**":            "PASS",
		"**Verdict: WARN**":               "WARN",
		"**Verdict:** PASS":               "PASS",
		"## Verdict: FAIL":                "FAIL",
		"Verdict: PASS (confidence 0.93)": "PASS",
		"no verdict here":                 "",
		"## Verdict\n\nsomething else\n":  "",
	}
	for in, want := range cases {
		if got := parseVerdict(in); got != want {
			t.Errorf("parseVerdict(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiffRounds_ResolvedNewCarried(t *testing.T) {
	t.Parallel()
	prev := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "caller-proof hard floor violated"},
		{ID: "H2", Severity: "HIGH", Title: "slug has no grading contract"},
		{ID: "M1", Severity: "MEDIUM", Title: "explanation names an area not in the diff"},
	}
	cur := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "Caller-proof hard floor violated for the third round"},
		{ID: "M3", Severity: "MEDIUM", Title: "brand new defect"},
	}
	resolved, fresh, carried := diffRounds(prev, cur)
	if resolved != 2 || fresh != 1 || carried != 1 {
		t.Fatalf("diffRounds = resolved %d new %d carried %d, want 2/1/1", resolved, fresh, carried)
	}
}

// Reworded but same id ⇒ carried; same lead but renumbered ⇒ carried; both
// different ⇒ new. Resolved counts the previous findings nothing matched.
func TestDiffRounds_MatchesByIDOrLeadClause(t *testing.T) {
	t.Parallel()
	prev := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "Caller-proof hard floor violated: exports have no callers"},
		{ID: "D1", Severity: "CRITICAL", Title: "the partial landing still silently consumes the whole inbox item"},
		{ID: "M1", Severity: "MEDIUM", Title: "explanation names an area not in the diff"},
	}
	cur := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "The base-bound diff still ships a new exported package with zero non-test callers"},
		{ID: "C1", Severity: "CRITICAL", Title: "The partial landing still silently consumes the whole inbox item, fourth filing"},
		{ID: "M4", Severity: "MEDIUM", Title: "brand new"},
	}
	resolved, fresh, carried := diffRounds(prev, cur)
	if resolved != 1 || fresh != 1 || carried != 2 {
		t.Fatalf("diffRounds = resolved %d new %d carried %d, want 1/1/2", resolved, fresh, carried)
	}
}

// One prior finding can be carried by at most one current finding: a new
// defect that merely reuses an old id is new, not a second carry.
func TestDiffRounds_PriorFindingConsumedOnce(t *testing.T) {
	t.Parallel()
	prev := []Finding{{ID: "H1", Severity: "HIGH", Title: "same lead clause defect"}}
	cur := []Finding{
		{ID: "H5", Severity: "HIGH", Title: "Same lead clause defect, restated"},
		{ID: "H1", Severity: "HIGH", Title: "an entirely different new defect"},
	}
	resolved, fresh, carried := diffRounds(prev, cur)
	if resolved != 0 || fresh != 1 || carried != 1 {
		t.Fatalf("diffRounds = resolved %d new %d carried %d, want 0/1/1", resolved, fresh, carried)
	}
}

func TestDiffRounds_FirstRoundIsAllNew(t *testing.T) {
	t.Parallel()
	resolved, fresh, carried := diffRounds(nil, []Finding{{ID: "H1", Title: "x"}})
	if resolved != 0 || fresh != 1 || carried != 0 {
		t.Fatalf("diffRounds(nil, one) = %d/%d/%d, want 0/1/0", resolved, fresh, carried)
	}
}

// TSV findings carry no id: matching must fall back to the lead clause only,
// never to a severity-only key that would call any HIGH the same defect.
func TestDiffRounds_IDLessFindingsMatchByLeadClauseOnly(t *testing.T) {
	t.Parallel()
	prev := []Finding{{Severity: "HIGH", Title: "defect A is untested"}}
	cur := []Finding{{Severity: "HIGH", Title: "defect B leaks a lock"}}
	resolved, fresh, carried := diffRounds(prev, cur)
	if resolved != 1 || fresh != 1 || carried != 0 {
		t.Fatalf("id-less diff = resolved %d new %d carried %d, want 1/1/0", resolved, fresh, carried)
	}
}
