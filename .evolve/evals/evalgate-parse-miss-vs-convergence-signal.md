---
score_cap:
  - criterion: "SelectedTasksParseMiss distinguishes a drifted '## Selected Tasks' section from genuine convergence"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestSelectedTasksParseMiss$' ./internal/evalgate | grep -q -- '--- PASS: TestSelectedTasksParseMiss'"
  - criterion: "The real cycle-1570 report shape is pinned as a named regression fixture"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestCycle1570ReportShape$' ./internal/evalgate | grep -q -- '--- PASS: TestCycle1570ReportShape'"
  - criterion: "The signal is scoped to the Selected Tasks section — a malformed Decision Trace never triggers it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged$' ./internal/evalgate | grep -q -- '--- PASS: TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged'"
  - criterion: "Gate A surfaces the parse-miss as an ADVISORY and never as a new hard block"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestMaterializationGate_ParseMissAdvisoryIsNonBlocking$' ./internal/evalgate | grep -q -- '--- PASS: TestMaterializationGate_ParseMissAdvisoryIsNonBlocking'"
  - criterion: "The evalgate package carries no regression (fail-open contract and scout-template token pins stay green)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/evalgate"
---

# Eval: Selected-Tasks parse-miss signal distinct from convergence

> Pins the distinguishing signal added in cycle 1685 for the inbox item
> `evalgate-selectedslugs-nil-blindness` (scout task
> `evalgate-parse-miss-vs-convergence-signal`). `evalgate.SelectedSlugs`
> collapsed two categorically different zero-slug scout-report shapes into the
> same `nil`: a genuine convergence report with no `## Selected Tasks` section,
> and a report WHOSE section is present and full of real task prose the parser
> cannot read. Gate A's `check()` returned `("", false)` for both, so "nothing
> to check" was byte-identical to "something to check that we failed to read" —
> which is how cycle-1570's missing eval for
> `config-gate-default-policy-authority` sailed past Gate A's fail-open path and
> surfaced three phases later as an audit H1. Source incident: cycle 1570;
> fixed in cycle 1685.

## Behavior under test

`SelectedTasksParseMiss(report string) bool` returns true ONLY for the drift
shape: the `## Selected Tasks` heading is present, its bounded body still has
content after blank lines and comments are stripped, and zero slug bullets parse
out of it. Everything else is false — no section at all, a section that parses
fine, an empty or comment-only section, and any content that lives after the
next `## ` heading.

Gate A (`materializationGate.check`) surfaces it as an additional ADVISORY WARN.
It must NOT flip `check`'s blocking return: `SelectedSlugs`'s fail-open contract
("empty means no claim, never zero work") is deliberate, and hard-blocking every
zero-slug report would false-block every genuine convergence cycle — the exact
risk the package header comment warns against.

## Graders

### [code] parse-miss is detected and distinguished from true convergence
```bash
cd go && go test -count=1 -v -run '^TestSelectedTasksParseMiss$' ./internal/evalgate | grep -q -- '--- PASS: TestSelectedTasksParseMiss'
```
The `grep` is load-bearing: `go test -run <pattern>` that matches NO test still
exits 0, so exit status alone would pass vacuously if the test were renamed away.

### [code] the real cycle-1570 shape is pinned as a regression fixture
```bash
cd go && go test -count=1 -v -run '^TestCycle1570ReportShape$' ./internal/evalgate | grep -q -- '--- PASS: TestCycle1570ReportShape'
```
The fixture must reproduce the ACTUAL incident shape — a `## Selected Tasks`
section naming `config-gate-default-policy-authority` in prose rather than in a
`- **Slug:**` bullet — not a synthetic stand-in that can drift from the defect.

### [code] the signal is scoped to the Selected Tasks section only
```bash
cd go && go test -count=1 -v -run '^TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged$' ./internal/evalgate | grep -q -- '--- PASS: TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged'
```
A report with a readable `## Selected Tasks` section beside a separately
malformed `## Decision Trace` block must NOT be flagged.

### [code] Gate A surfaces it as advisory, never as a new hard block
```bash
cd go && go test -count=1 -v -run '^TestMaterializationGate_ParseMissAdvisoryIsNonBlocking$' ./internal/evalgate | grep -q -- '--- PASS: TestMaterializationGate_ParseMissAdvisoryIsNonBlocking'
```
Must assert both halves: `check()`'s reason names the drift, and its `bool`
return stays false when no slug is missing or ungraded.

### [code] no regression in the package
```bash
cd go && go test -count=1 ./internal/evalgate
```
`TestSelectedSlugs` (the fail-open contract) and `TestSlugParserContract` (the
scout-template token pins) must stay green — this fix is additive.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| advisory-wiring | Gate A surfaces the signal without blocking | 9/10 | `-run TestMaterializationGate_ParseMissAdvisoryIsNonBlocking` |
| drift-vs-convergence | the detector separates the two zero-slug shapes | 8/10 | `-run TestSelectedTasksParseMiss` |
| incident-fixture | the real cycle-1570 shape is pinned | 7/10 | `-run TestCycle1570ReportShape` |
| section-scoping | a malformed Decision Trace does not trigger it | 6/10 | `-run TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged` |
| no-regression | package suite stays green | 5/10 | `go test ./internal/evalgate` |
