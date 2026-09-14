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
  - criterion: "The advisory's aggregate measured operating point is on the record, not just the marginal contribution of one formatting variant"
    max_if_missing: 7
    evidence: "cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eq '59 *(of +(the +)?|/)84'"
  - criterion: "The every-cycle warning is stated in the present tense the measurement supports, not as a conditional future"
    max_if_missing: 8
    evidence: "! cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eqi 'would +warn +every +cycle'"
  - criterion: "The slugLineRE widening's blocking effect is pinned by an executable in-package test, not by prose"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestSlugLineWideningIsNotBlockingNeutral$' ./internal/evalgate | grep -q -- '--- PASS: TestSlugLineWideningIsNotBlockingNeutral'"
  - criterion: "The falsified blocking-neutrality claim is gone from the production comment that originated it"
    max_if_missing: 9
    evidence: "! grep -Eqi 'is (also )?blocking-neutral|no report becomes newly blockable|union[^.]* is unchanged' go/internal/evalgate/slugs.go"
  - criterion: "The falsified blocking-neutrality claim is gone from the explanation document"
    max_if_missing: 8
    evidence: "! cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eqi '(is|was|were) (also |measured )?blocking-neutral|no report becomes newly blockable'"
  - criterion: "The measured union delta and the two falsifying reports are on the record where the false claim stood"
    max_if_missing: 7
    evidence: "grep -Eq '6 *(of +(the +)?|/)84' go/internal/evalgate/slugs.go && grep -Eq 'cycle-1664|settle-wait-stability-shortcircuit' go/internal/evalgate/slugs.go && grep -Eq 'cycle-1669|verdict-tool-call-claudep' go/internal/evalgate/slugs.go"
---

# Eval: evalgate SelectedSlugs nil-blindness (parse-miss vs convergence)

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

### [code] the aggregate operating point is stated
```bash
cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eq '59 *(of +(the +)?|/)84'
```
### [code] the every-cycle warning is not framed as hypothetical
```bash
! cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eqi 'would +warn +every +cycle'
```
Both pin standing audit finding M1 (cycle 1685, audit round 1). The advisory's
operating point, measured 2026-09-15 by calling the shipped detector over
`.evolve/runs/cycle-16*/scout-report.md`: of the 84 reports carrying a
`## Selected Tasks` section, `SelectedTasksParseMiss` fires on **59 (70.2%)**,
and **56** of those 59 have an entirely empty `SelectedSlugs` union — Gate A
really was checking nothing on those cycles. The `~1 cycle in 7` figure the
build report and explanation document originally reasoned with is the *marginal*
contribution of the backticked-slug variant alone (the widening rescued exactly
12 reports: the pre-fix pattern fired on 71 of 84, 84.5%), not the rate at which
the advisory speaks. The corpus is gitignored and grows every cycle, so these
graders pin the DOCUMENTED, dated measurement rather than re-measuring — the
re-derivation lives in `TestC1685_011`.

### [code] the widening's blocking effect is executable, not asserted
```bash
cd go && go test -count=1 -v -run '^TestSlugLineWideningIsNotBlockingNeutral$' ./internal/evalgate | grep -q -- '--- PASS: TestSlugLineWideningIsNotBlockingNeutral'
```
### [code] the falsified neutrality claim is gone from slugs.go
```bash
! grep -Eqi 'is (also )?blocking-neutral|no report becomes newly blockable|union[^.]* is unchanged' go/internal/evalgate/slugs.go
```
### [code] the falsified neutrality claim is gone from the explanation document
```bash
! cat docs/explain/builds/cycle-1685-*.md | tr '\n' ' ' | grep -Eqi '(is|was|were) (also |measured )?blocking-neutral|no report becomes newly blockable'
```
### [code] the measured effect and its two falsifying reports are on the record
```bash
grep -Eq '6 *(of +(the +)?|/)84' go/internal/evalgate/slugs.go && grep -Eq 'cycle-1664|settle-wait-stability-shortcircuit' go/internal/evalgate/slugs.go && grep -Eq 'cycle-1669|verdict-tool-call-claudep' go/internal/evalgate/slugs.go
```
These four pin standing audit finding **H1** (cycle 1685, audit round 2). Round 1
shipped, in three places, the claim that widening `slugLineRE` to tolerate a
backticked slug was *blocking-neutral* — "every one of those 12 also carries a
`## Decision Trace` naming the same slugs, so the union `SelectedSlugs` returns
is unchanged and no report becomes newly blockable". What round 1 measured was
HEADING PRESENCE; what it claimed was union equality, which was never measured.
Re-measured 2026-09-15 by calling the shipped `SelectedSlugs` on every
`.evolve/runs/cycle-16*/scout-report.md` and comparing against the pre-widening
pattern: of the 84 reports carrying a `## Selected Tasks` section, **6** return a
different union (all six empty → non-empty, their `## Decision Trace` stating the
selection as a `"selected_tasks"` array `decisionTraceSelected` does not read),
and **2** of those 6 become newly **blockable** at Gate A because the newly
parsed slug has no eval file: cycle-1664 (`settle-wait-stability-shortcircuit`)
and cycle-1669 (`verdict-tool-call-claudep`). The widening is a deliberate
capability increase — Gate A catching a genuinely missing eval is its cycle-166
job — and reverting it to make the old sentence true is NOT the remedy. The
corpus is gitignored and absent in CI, so the durable grader is the hermetic
in-package test rather than a re-measurement.


## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| advisory-wiring | Gate A surfaces the signal without blocking | 9/10 | `-run TestMaterializationGate_ParseMissAdvisoryIsNonBlocking` |
| drift-vs-convergence | the detector separates the two zero-slug shapes | 8/10 | `-run TestSelectedTasksParseMiss` |
| incident-fixture | the real cycle-1570 shape is pinned | 7/10 | `-run TestCycle1570ReportShape` |
| section-scoping | a malformed Decision Trace does not trigger it | 6/10 | `-run TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged` |
| no-regression | package suite stays green | 5/10 | `go test ./internal/evalgate` |
| operating-point | the aggregate fire rate (59/84, 70.2%) is on the record | 7/10 | `grep` the explanation document |
| present-tense-framing | the every-cycle warning is not framed as hypothetical | 8/10 | `grep -v` the conditional phrasing |
| widening-effect-executable | the widening's blocking effect is pinned by a running test | 9/10 | `-run TestSlugLineWideningIsNotBlockingNeutral` |
| false-claim-removed-source | the falsified neutrality claim is gone from slugs.go | 9/10 | negative `grep` on the production comment |
| false-claim-removed-doc | the falsified neutrality claim is gone from the document | 8/10 | negative `grep` on the explanation document |
| measured-effect-on-record | the 6-of-84 delta and its 2 falsifying reports are stated | 7/10 | `grep` the fraction and both report identifiers |
