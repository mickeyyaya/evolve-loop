package reportdoc

import (
	"reflect"
	"strings"
	"testing"
)

// Both heading shapes observed live (cycle-1605 final round, cycle-1604 round 1)
// must parse, and the Reflection section's `###` headings must not.
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

// TestFindings_BothHeadingShapes names Finding and Findings.
func TestFindings_BothHeadingShapes(t *testing.T) {
	t.Parallel()
	got := Findings(reportBothShapes)
	want := []Finding{
		{ID: "H1", Severity: "HIGH", Title: "caller-proof hard floor violated for the third consecutive round"},
		{ID: "M1", Severity: "MEDIUM", Title: "three lead claims are measured against the salvage snapshot"},
		{ID: "L1", Severity: "LOW", Title: "scope — disclosed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Findings = %+v, want %+v", got, want)
	}
}

// The two other layouts live in recorded reports: cycle 1605 rounds 1–2 and
// cycle 1596 tabulate issues; cycle 1604 writes `### C1 — CRITICAL · title`.
// Header and separator rows are not findings.
func TestFindings_TableRowsAndDashHeadings(t *testing.T) {
	t.Parallel()
	table := "## Issues\n\n| ID | Severity | Defect | Root cause | Evidence |\n|---|---|---|---|---|\n" +
		"| H1 | HIGH | Caller-proof hard floor violated: `decisionsample.Vote` has no callers | rc | ev |\n" +
		"| M1 | MEDIUM | The wiring half is undelivered | rc | ev |\n\nNo CRITICAL findings.\n\n## Predicate Quality\n"
	got := Findings(table)
	if len(got) != 2 || got[0] != (Finding{ID: "H1", Severity: "HIGH", Title: "Caller-proof hard floor violated: `decisionsample.Vote` has no callers"}) || got[1].ID != "M1" {
		t.Fatalf("table Findings = %+v", got)
	}
	dash := "## Issues\n### C1 — CRITICAL · the partial landing still silently consumes the whole inbox item\n**Root cause:** x\n### M3 - MEDIUM: prescription not applied\n"
	got = Findings(dash)
	if len(got) != 2 || got[0] != (Finding{ID: "C1", Severity: "CRITICAL", Title: "the partial landing still silently consumes the whole inbox item"}) ||
		got[1] != (Finding{ID: "M3", Severity: "MEDIUM", Title: "prescription not applied"}) {
		t.Fatalf("dash Findings = %+v", got)
	}
	// A table row from round 1 and a heading from round 3 about the same defect share a key.
	if FindingKey(got0Title(table)) != FindingKey("caller-proof hard floor violated for the third consecutive round") {
		t.Fatalf("cross-layout key mismatch: %q vs %q", FindingKey(got0Title(table)), FindingKey("caller-proof hard floor violated for the third consecutive round"))
	}
}

func got0Title(md string) string { return Findings(md)[0].Title }

// Cycles 1595/1596 bold the id and severity cells and lead the defect with a
// bold sentence; cycle 1606 writes `### H3 — title` with no severity token.
func TestFindings_BoldCellsAndImpliedSeverity(t *testing.T) {
	t.Parallel()
	bold := "## Issues\n| ID | Sev | Finding | Evidence |\n|---|---|---|---|\n" +
		"| **H1** | **HIGH** | **This cycle's production change is entirely untested.** The floor adds three symbols | ev |\n" +
		"| **H2** | CRITICAL | **Fabricated dossier still staged for ship.** more | ev |\n"
	got := Findings(bold)
	if len(got) != 2 || got[0].Severity != "HIGH" || got[1].Severity != "CRITICAL" ||
		!strings.HasPrefix(got[0].Title, "This cycle's production change is entirely untested.") {
		t.Fatalf("bold rows = %+v", got)
	}
	implied := "## Issues\n### H3 — the validated triage hint skips the conditional-mandatory `tdd` phase\nbody\n### M2 — a medium one\n### X9 — unknown prefix\n"
	got = Findings(implied)
	if len(got) != 3 || got[0] != (Finding{ID: "H3", Severity: "HIGH", Title: "the validated triage hint skips the conditional-mandatory `tdd` phase"}) ||
		got[1].Severity != "MEDIUM" || got[2].Severity != "" {
		t.Fatalf("implied severities = %+v", got)
	}
}

// The three phrasings cycle 1605 used for its H1 across rounds 1–3 must share
// one key: the lead clause is the identity, the tail is commentary.
func TestFindingKey_LeadClauseIsTheIdentity(t *testing.T) {
	t.Parallel()
	phrasings := []string{
		"Caller-proof hard floor violated: `decisionsample.Vote`, `decisionsample.Sample` are new exported identifiers",
		"**Caller-proof hard floor violated, and unremediated across a repair dispatch.** `decisionsample.Sample` …",
		"caller-proof hard floor violated for the third consecutive round",
	}
	want := FindingKey(phrasings[0])
	for _, p := range phrasings[1:] {
		if FindingKey(p) != want {
			t.Errorf("FindingKey(%q) = %q, want %q", p, FindingKey(p), want)
		}
	}
	if FindingKey("A scout-selected slug has no eval.") == want {
		t.Fatal("distinct leads must not collide")
	}
}

func TestFindings_NoIssuesSectionScansWholeOrYieldsNone(t *testing.T) {
	t.Parallel()
	if got := Findings("# Audit\n\n**Verdict: PASS**\n\n## Reflection\n### What slowed (required)\n"); len(got) != 0 {
		t.Fatalf("PASS report = %+v, want none", got)
	}
	if got := Findings("### H1 (HIGH) — loose heading without an Issues section\n"); len(got) != 1 || got[0].ID != "H1" {
		t.Fatalf("whole-document scan = %+v", got)
	}
}

// A template or example inside a fence, an indented block, or an HTML comment
// must not raise a phantom finding — the same visibility rule Section/Fields
// apply, and the hole the architecture review found in the first cut.
func TestFindings_IgnoresFencedIndentedAndCommentedExamples(t *testing.T) {
	t.Parallel()
	doc := "## Issues\n\n```markdown\n### H9 (CRITICAL) — example finding in a fence\n```\n\n" +
		"    ### H8 (HIGH) — indented example\n\n<!-- ### H7 (HIGH) — commented example -->\n\n" +
		"### M1 (MEDIUM) — the only real finding\nbody\n"
	got := Findings(doc)
	if len(got) != 1 || got[0].ID != "M1" {
		t.Fatalf("Findings = %+v, want only M1", got)
	}
}

// TestVerdict_MatchesTheAuditGatesProseFallback pins that Verdict is the SAME
// grammar phases/audit.extractAuditVerdict falls back to below enforce: the
// canonical heading, the colon-bearing inline forms, SKIPPED in the vocabulary,
// and — deliberately — NOT a prose sentence without a colon, NOT a lowercase
// JSON key, NOT a lowercase heading.
func TestVerdict_MatchesTheAuditGatesProseFallback(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"## Verdict\n\n**FAIL**\n":                                   "FAIL",
		"## Verdict\n**PASS**":                                       "PASS",
		"## Verdict\nSKIPPED":                                        "SKIPPED",
		"**Verdict: WARN**":                                          "WARN",
		"**Verdict:** PASS":                                          "PASS",
		"## Verdict: FAIL":                                           "FAIL",
		"Verdict: PASS (confidence 0.93)":                            "PASS",
		"Verdict PASS is required before shipping.":                  "",
		"{\"verdict\": \"PASS\"}\n\n**Verdict: FAIL**":               "FAIL",
		"## verdict\n**PASS**":                                       "",
		"no verdict here":                                            "",
		"## Verdict\n\nsomething else\n":                             "",
		"```\n## Verdict\n\n**PASS**\n```\n\n## Verdict\n\n**FAIL**": "FAIL",
		"    **Verdict: PASS**":                                      "",
	}
	for in, want := range cases {
		if got := Verdict(in); got != want {
			t.Errorf("Verdict(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindingKey_MatchesAcrossRoundOrdinals(t *testing.T) {
	t.Parallel()
	a := FindingKey("caller-proof hard floor violated")
	b := FindingKey("Caller-proof hard floor violated for the third consecutive round")
	if a == "" || a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
	if FindingKey("slug has no grading contract") == a {
		t.Fatal("distinct findings must not collide")
	}
}

// The reference template (agents/evolve-auditor-reference.md) writes the
// issues as a fenced ```tsv block under ## Issues: Severity⇥Description⇥File⇥Line.
// That block IS the table, not an example, and must yield findings (id-less).
func TestFindings_ReferenceTemplateTSVBlock(t *testing.T) {
	t.Parallel()
	doc := "## Verdict\n\n**FAIL**\n\n<!-- ANCHOR:defects -->\n## Issues\n```tsv\nSeverity\tDescription\tFile\tLine\n" +
		"HIGH\tThe wiring half is undelivered\tgo/internal/x.go\t12\n**MEDIUM**\tA mock budget is exceeded\tgo/internal/y_test.go\t3\n```\n\n## Self-Evolution Assessment\n"
	got := Findings(doc)
	if len(got) != 2 || got[0] != (Finding{Severity: "HIGH", Title: "The wiring half is undelivered"}) || got[1].Severity != "MEDIUM" {
		t.Fatalf("TSV Findings = %+v", got)
	}
	// A tsv fence OUTSIDE ## Issues is an example and stays invisible.
	if got := Findings("## Notes\n```tsv\nHIGH\tnot a finding\n```\n## Issues\n### M1 (MEDIUM) — real\n"); len(got) != 1 || got[0].ID != "M1" {
		t.Fatalf("tsv outside Issues leaked: %+v", got)
	}
}

// Outside an ## Issues section, an id-shaped heading without a severity token
// is not a finding: PASS-shaped or differently organised reports must not
// grow phantom issues.
func TestFindings_NoIssuesSectionRequiresExplicitSeverity(t *testing.T) {
	t.Parallel()
	doc := "## Plan Adherence\n### S1 — the tdd phase was skipped\n### G2 — landed autonomously\n### Q3: follow-up\n### H1 (HIGH) — an explicit one\n"
	got := Findings(doc)
	if len(got) != 1 || got[0].ID != "H1" {
		t.Fatalf("Findings outside Issues = %+v, want only the explicit-severity heading", got)
	}
}

// A report that quotes the whole reference template inside a fence BEFORE its
// own Issues section (the template is one skeleton, ## Issues and its tsv
// included) must yield only the real findings: the section is located on the
// visible stream, so the fenced copy is never mistaken for it.
func TestFindings_QuotedTemplateBeforeRealIssuesSection(t *testing.T) {
	t.Parallel()
	doc := "## Notes\n````markdown\n## Issues\n```tsv\nSeverity\tDescription\tFile\tLine\nHIGH\t<issue>\t<file>\t<line>\n```\n## Verdict\n**PASS**\n````\n\n" +
		"## Issues\n### M1 (MEDIUM) — the real finding\nbody\n\n## Reflection\n### H9 — not one\n"
	got := Findings(doc)
	if len(got) != 1 || got[0].ID != "M1" {
		t.Fatalf("Findings = %+v, want only the real M1", got)
	}
}

func TestSeverityRank_OrdersTheGrammarAndParksUnknownLast(t *testing.T) {
	t.Parallel()
	if SeverityRank("CRITICAL") != 0 || SeverityRank("HIGH") != 1 || SeverityRank("MEDIUM") != 2 || SeverityRank("LOW") != 3 {
		t.Fatalf("known ranks drifted: C=%d H=%d M=%d L=%d", SeverityRank("CRITICAL"), SeverityRank("HIGH"), SeverityRank("MEDIUM"), SeverityRank("LOW"))
	}
	if SeverityRank("INFO") <= SeverityRank("LOW") || SeverityRank("") <= SeverityRank("LOW") {
		t.Fatal("unknown severities must rank after every known one")
	}
}
