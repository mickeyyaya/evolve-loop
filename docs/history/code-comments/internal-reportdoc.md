# Comment history: `internal/reportdoc`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/reportdoc/findings.go:8` — above `type Finding struct {`

````text
// findings.go — the auditor's issue and verdict grammar, single-homed.
//
// The auditor persona fixes the vocabulary (IDs H1/M1/L1/C1, severities
// CRITICAL/HIGH/MEDIUM/LOW) but not the layout. Five layouts are live —
// four observed in recorded reports (cycles 1595, 1596, 1604, 1605, 1606) and
// the reference template's own (agents/evolve-auditor-reference.md):
//
//	### H1 (HIGH) — caller-proof hard floor violated          (heading, severity in parens)
//	### C1 — CRITICAL · the partial landing still consumes … (heading, severity after a dash)
//	### H3 — the validated triage hint skips the tdd phase    (heading, severity implied by the id letter)
//	| **H1** | **HIGH** | **Caller-proof hard floor …** | …  (table row: id, severity, defect; emphasis optional)
//	```tsv  Severity⇥Description⇥File⇥Line  HIGH⇥<issue>⇥…      (the reference template: a fenced TSV under ## Issues)
//
// and, when it does not emit the machine-readable sentinel
// (phasecontract.ParseVerdictSentinel — always authoritative, tried first by
// every consumer), declares the verdict as the canonical two-line heading
// `## Verdict` + `**PASS**` or as an inline `**Verdict: PASS**`.
//
// Consumers today: the audit gate's regex-on-prose fallback
// (phases/audit extractAuditVerdict → Verdict) and the dashboard's failure
// panel (Findings, Verdict, FindingKey). The repair-brief seed in core is the
// intended third consumer (research proposal R2) so that what the gate scores,
// what the operator is shown, and what the rebuilding agent is told cannot
// drift; until it lands, this package is still the only place the grammar is
// spelled.
//
// Like every extractor in this package, both functions scan only VISIBLE
// lines: fenced code, indented code and HTML comments are stripped first, so
// a template or example embedded in a report cannot declare a verdict or raise
// a phantom finding. The one deliberate exception is the reference template's
// own fenced TSV block directly under `## Issues`, which IS the issues table.
````

### `go/internal/reportdoc/findings.go:230` — above `func FindingKey(title string) string {`

```text
// FindingKey reduces a finding title to a cross-round comparison key: the
// lead clause, lower-cased, alphanumerics only, capped at 80 characters — so
// a table cell's long defect text in round 1 and a terse heading restating the
// same lead in round 3 still meet.
```

### `go/internal/reportdoc/findings_test.go:9` — above `const reportBothShapes = '# Audit Report — Cycle 1605 (round 3)`

```text
// Both heading shapes observed live (cycle-1605 final round, cycle-1604 round 1)
// must parse, and the Reflection section's `###` headings must not.
```

### `go/internal/reportdoc/findings_test.go:47` — above `func TestFindings_TableRowsAndDashHeadings(t *testing.T) {`

```text
// The two other layouts live in recorded reports: cycle 1605 rounds 1–2 and
// cycle 1596 tabulate issues; cycle 1604 writes `### C1 — CRITICAL · title`.
// Header and separator rows are not findings.
```

### `go/internal/reportdoc/findings_test.go:65` — above `if FindingKey(got0Title(table)) != FindingKey("caller-proof hard floor violated for the third consecutive round") {`

```text
// A table row from round 1 and a heading from round 3 about the same defect share a key.
```

### `go/internal/reportdoc/findings_test.go:73` — above `func TestFindings_BoldCellsAndImpliedSeverity(t *testing.T) {`

```text
// Cycles 1595/1596 bold the id and severity cells and lead the defect with a
// bold sentence; cycle 1606 writes `### H3 — title` with no severity token.
```

### `go/internal/reportdoc/findings_test.go:93` — above `func TestFindingKey_LeadClauseIsTheIdentity(t *testing.T) {`

```text
// The three phrasings cycle 1605 used for its H1 across rounds 1–3 must share
// one key: the lead clause is the identity, the tail is commentary.
```

### `go/internal/reportdoc/open_nofollow_unix.go:10` — above `func OpenRegularNoFollow(path string) (*os.File, error) {`

```text
// OpenRegularNoFollow opens path read-only without following a final-component
// symlink (O_NOFOLLOW). Shared belief for every bounded read of an
// agent-writable file — single home; do not copy this helper into another
// package (architecture review 2026-09-01).
```

### `go/internal/reportdoc/reportdoc.go:64` — above `var listValuedFields = map[string]bool{"evidence": true}`

```text
// listValuedFields may repeat: a careful reviewer writes one `- Evidence:` line
// per claim (cycle-1604 wrote four and was rejected as "duplicate evidence
// fields" — five of eleven consecutive FAILs were this section's format). Their
// values are joined with newlines; every other key stays single-valued, so a
// repeated Status is still the ambiguity it always was.
```

### `go/internal/reportdoc/reportdoc.go:84` — above `func Fields(body string, allowed ...string) (map[string]string, error) {`

```text
// Fields parses allowed "- Key: value" metadata lines case-insensitively.
// Unknown prose fields are ignored; a value's surrounding backticks are
// stripped (cycle-1606 wrote `Document: `path“); list-valued keys accumulate,
// single-valued keys reject a repeat.
```

### `go/internal/reportdoc/reportdoc.go:120` — above `func RequireReasoning(evidence string) error {`

```text
// RequireReasoning is the reasoning floor: a review's Evidence must say
// something — token or canned evidence is not a review. Since 2026-09-13
// (ADR-0102) this is the one blocking rule the audit and retro gates keep;
// the citation form checked below is advisory there.
```

### `go/internal/reportdoc/reportdoc.go:131` — above `func ReasonedReview(report, doc, heading string, allowed ...string) (fields map[string]string, advisories []string, err …`

```text
// ReasonedReview is the ONE reading of a review section every phase gate
// shares (ADR-0102, "absence is never laxer than thinness"): the section is
// located by heading, its fields are parsed, and the reasoning floor is
// applied — a missing section is a missing reasoning (error, naming doc), a
// duplicated section is a parser finding with no attributable review text
// (advisory + the floor on ""), a garbled field is a finding with the floor
// on the Evidence as written (advisory, fields nil; no Evidence line is no
// reasoning), and thin Evidence fails the floor. The gates apply their phase policy to the fields on top; the guard
// TestExplanationReviewGates_ShareContractCore forbids them the ladder.
```

### `go/internal/reportdoc/reportdoc.go:237` — above `func citationRangeContinues(rest string) bool {`

```text
// citationRangeContinues accepts the range and column forms reviewers write
// after a line number — `:23-48`, `:12:3`, `#L7-L9` — so the first line still
// counts as the citation (cycle-1605 cited sampler.go:23-48 and was rejected).
```

### `go/internal/reportdoc/reportdoc.go:261` — above `func EvidenceOrBody(body, evidence string) string {`

```text
// EvidenceOrBody returns the review's evidence text: the Evidence field when
// the reviewer wrote one, else the whole section body — a reviewer who put the
// citations under `- Binding:` and `- Correction still required:` (cycle-1606)
// still has to cite every material path at a line; only the field name is
// forgiven, never the grounding.
```

### `go/internal/reportdoc/reportdoc.go:309` — above `func ValidReference(reference string) bool {`

```text
// ValidReference reports whether reference is a safe, clean, relative path —
// the single home of the "is this relative path safe?" belief, shared with
// explanationdocs (architecture review 2026-09-01: the two packages had
// divergent validators judging the same string differently).
```

### `go/internal/reportdoc/reportdoc_test.go:258` — above `func TestRequireReasoning_TokenEvidenceFailsAProseSentencePasses(t *testing.T) {`

```text
// RequireReasoning is the reasoning floor (ADR-0102): the one blocking rule
// the audit and retro review gates keep — a token Evidence is not a review.
```

### `go/internal/reportdoc/reportdoc_test.go:272` — above `func TestReasonedReview_AbsenceIsNeverLaxerThanThinness(t *testing.T) {`

```text
// ReasonedReview is the one reading of a review section both phase gates
// share (ADR-0102): absence is never laxer than thinness.
```

### `go/internal/reportdoc/reportdoc_test.go:301` — above `func TestReasonedReview_GarbledBoilerplateWithoutEvidenceFailsTheFloor(t *testing.T) {`

```text
// A garbled section whose only content is duplicated boilerplate — no
// Evidence line — is no reasoning: the floor measures the Evidence as
// written, never the section's bulk (go review, ADR-0102).
```

### `go/internal/reportdoc/review_fields_test.go:3` — above `import (`

```text
// review_fields_test.go — the explanation-review section as auditors actually
// write it (cycles 1604, 1605, 1606 verbatim). Five of eleven consecutive
// FAILs were this section's FORMAT, not its substance: several `- Evidence:`
// lines, a line range in a citation, backticks around a value, prose under
// other field names. The parser reads what a careful reviewer wrote; the
// substance gates (every material path cited at a line) are unchanged.
```

### `go/internal/reportdoc/safe_read_exports_test.go:11` — above `func TestValidReference_SingleSourcedStrictness(t *testing.T) {`

```text
// TestValidReference_SingleSourcedStrictness names the exported validator that
// is now the single home of the "is this relative path safe?" belief, shared
// with explanationdocs (architecture review 2026-09-01: the two packages
// previously judged the same string differently).
```
