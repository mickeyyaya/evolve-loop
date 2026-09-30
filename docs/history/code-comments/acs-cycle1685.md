# Comment history: `acs/cycle1685`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1685/predicates_test.go:3` — above `package cycle1685`

```text
// Package cycle1685 materialises the cycle-1685 acceptance criteria for the one
// fleet-scoped inbox id `evalgate-selectedslugs-nil-blindness` (scout task
// `evalgate-parse-miss-vs-convergence-signal`).
//
// The defect. `evalgate.SelectedSlugs` collapses two categorically different
// zero-slug scout-report shapes into the same `nil`:
//
//  1. genuine convergence — no "## Selected Tasks" section at all (nothing was
//     claimed; fail-open is CORRECT here), and
//  2. format drift — a "## Selected Tasks" section that IS present with real
//     task prose whose slug is stated in a form `slugLineRE` does not
//     recognise, so it parses to zero slugs (the cycle-1570 shape: scout
//     selected `config-gate-default-policy-authority`, no eval file, and Gate
//     A's fail-open path let it through to surface three phases later as an
//     audit H1).
//
// Gate A's `check()` — the only seam an agent or operator actually sees —
// returns ("", false) for BOTH, so nothing distinguishes "nothing to check"
// from "something to check that we failed to read".
//
// The accepted fix is a new exported `SelectedTasksParseMiss(report string) bool`
// scoped strictly to the bounded "## Selected Tasks" section, surfaced through
// Gate A as an ADVISORY WARN. It is deliberately NOT a new hard block: blocking
// every zero-slug report would false-block every genuine convergence cycle and
// contradict the package's own documented fail-open-on-ambiguity contract.
//
// Predicate strategy — every predicate exercises the system under test (the
// cycle-85 degenerate-predicate ban):
//
//   - 001-005 CALL the detector on real report bodies and assert its boolean.
//     001 is the crux positive; 002/003/004/005 are the negatives and the
//     section-bounding edges that a no-op `return true` would fail.
//   - 006 is the CALLER PROOF: it drives the real production reviewer
//     (`evalgate.NewReviewer(...).Review(...)`, the core.WithReviewer seam) and
//     asserts the advisory actually reaches Gate A's emitted log line. A
//     detector nothing calls from production is dead code, and a predicate that
//     only calls it directly would pass on dead code.
//   - 007 pins the contract the fix must NOT break: silence on genuine
//     convergence, and the pre-existing HARD BLOCK on a selected slug with no
//     eval file still fires at enforce.
//   - 008 closes the apicover false-green hole (internal/evalgate is enrolled
//     in go/.apicover-enforce) using apicover's own AST detector.
//   - 009 proves the durable eval's three `[code]` grader tests actually RAN
//     and passed — `go test -run <name>` that matches NOTHING still exits 0,
//     which is the vacuous-pass hole those graders would otherwise carry — and
//     that the files carrying them are git-TRACKED (a gitignored test file is
//     dropped at ship: the cycle-92 shape).
//   - 010 is the no-regression floor for the package.
//   - 011-012 materialise the standing audit finding M1 (audit round 1). They are
//     the ONE class in this file that asserts on a prose deliverable, because the
//     remedy M1 asks for IS prose: the explanation document must state the
//     advisory's aggregate measured operating point instead of the marginal
//     contribution of one formatting variant. They carry an explicit
//     `// acs-predicate: config-check` waiver and are NOT bare greps — 011 parses
//     the numerator/denominator/percentage out of the document and re-derives the
//     arithmetic, so pasting the pre-existing `12 of 84` / `one cycle in seven`
//     figures cannot satisfy them.
```

### `go/acs/cycle1685/predicates_test.go:83` — above `const cycle1570Report = "# Scout Report\n\n## Selected Tasks\n\n" +`

```text
// cycle1570Report is the REAL incident shape, reproduced verbatim from this
// cycle's bug-reproduction phase: a "## Selected Tasks" section holding genuine
// task prose for `config-gate-default-policy-authority`, whose slug is stated as
// free prose ("Task slug: ...") rather than the "- **Slug:** <kebab>" bullet the
// parser requires. Neither empty nor absent — and it parses to zero slugs.
```

### `go/acs/cycle1685/predicates_test.go:97` — above `func TestC1685_001_ParseMissTrueOnCycle1570Shape(t *testing.T) {`

```text
// TestC1685_001_ParseMissTrueOnCycle1570Shape is the crux: the detector must
// return true for a Selected Tasks section that has real content the parser
// could not read. Asserts the fixture PREMISE first (this shape really does
// still parse to zero slugs) so a future parser change that stops reproducing
// the incident fails loudly here instead of the predicate quietly testing
// nothing.
```

### `go/acs/cycle1685/predicates_test.go:310` — above `func TestC1685_009_EvalGraderTestsRanPassedAndAreTracked(t *testing.T) {`

```text
// TestC1685_009_EvalGraderTestsRanPassedAndAreTracked proves the durable eval's
// three [code] graders are not vacuous. `go test -run <name>` whose pattern
// matches NOTHING still exits 0, so each grader is re-run here and the "--- PASS:
// <name>" line is required. It also pins the REAL incident slug as the fixture
// (not a synthetic stand-in, which the eval demands by name) and asserts the
// files carrying these tests are git-TRACKED — a gitignored test file is silently
// dropped at ship.
```

### `go/acs/cycle1685/predicates_test.go:490` — above `const (`

```text
// The advisory's operating point, measured 2026-09-15 over
// `.evolve/runs/cycle-16*/scout-report.md` by calling the shipped detector on
// each report: 84 reports carry a "## Selected Tasks" section (all of them),
// SelectedTasksParseMiss fires on 59 of those (70.2%), and 56 of the 59 have an
// entirely empty SelectedSlugs union — i.e. Gate A really was checking nothing
// on those cycles, which is what makes the number a finding rather than noise.
// For context the pre-fix pattern fired on 71 of 84 (84.5%), so the backtick
// widening rescued exactly 12 reports (71-59), matching the document's own
// "12 of 84" claim.
```

### `go/acs/cycle1685/predicates_test.go:512` — above `var isoDateRE = regexp.MustCompile('\d{4}-\d{2}-\d{2}')`

```text
// isoDateRE matches the YYYY-MM-DD measurement stamp this diff already uses in
// slugs.go ("measured 2026-09-15").
```

### `go/acs/cycle1685/predicates_test.go:516` — above `func TestC1685_011_ExplanationStatesMeasuredOperatingPoint(t *testing.T) {`

```text
// TestC1685_011_ExplanationStatesMeasuredOperatingPoint materialises standing
// audit finding M1. The explanation document must state the advisory's AGGREGATE
// operating point — the rate at which the signal actually speaks — not only the
// marginal contribution of the backticked-slug variant.
//
// acs-predicate: config-check
//
// Waiver rationale: the remedy M1 asks for is a prose correction to a tracked
// deliverable, so the document's text IS the system under test and there is no
// other seam to drive — the cycle-85 "magic string is not the fix" hazard does
// not apply, because here the string is precisely the fix. It is still not a
// bare grep: the numerator, denominator and percentage are parsed out of the
// document and the percentage is RE-DERIVED from the fraction, so the
// pre-existing "12 of 84 / one cycle in seven" figures cannot satisfy it.
```

### `go/acs/cycle1685/predicates_test.go:688` — above `const (`

```text
// --- audit round 2, finding H1 -----------------------------------------------
//
// The round-1 diff attached a NEUTRALITY CLAIM to the `slugLineRE` widening and
// shipped it in three places: the production comment (slugs.go), the explanation
// document (twice) and build-report.md. It said: "every one of those 12 also
// carries a '## Decision Trace' naming the same slugs, so the union
// SelectedSlugs returns is unchanged and no report becomes newly blockable."
//
// What round 1 measured was HEADING PRESENCE — all 12 do carry a
// "## Decision Trace". What it CLAIMED was union equality, which was never
// measured. Re-measured 2026-09-15 by calling the shipped SelectedSlugs on every
// `.evolve/runs/cycle-16*/scout-report.md` and comparing against the
// pre-widening pattern copied verbatim from `git show HEAD:...slugs.go`:
//
//	84  reports carry a "## Selected Tasks" section
//	12  state the slug in the backticked form (the document's own figure)
//	 6  return a DIFFERENT union — all six empty -> non-empty
//	 2  of those six become newly BLOCKABLE at Gate A / StageEnforce, the newly
//	    parsed slug having no eval file at either root evalFilePath checks:
//	      cycle-1664 -> settle-wait-stability-shortcircuit
//	      cycle-1669 -> verdict-tool-call-claudep
//
// The cause is the one the audit names: those reports DO carry a
// "## Decision Trace", but it states the selection as a "selected_tasks" string
// array rather than the decisionTrace[].finalDecision shape decisionTraceSelected
// reads, so the trace supplies nothing and the backticked bullet is the slug's
// only source.
//
// The widening is therefore a deliberate CAPABILITY INCREASE — Gate A now
// catching two genuinely missing evals is the gate doing its cycle-166 job — and
// NOT a neutral edit. 013/014 pin that behaviour hermetically so a later "fix"
// cannot quietly revert the widening instead of correcting the sentence;
// 015/016/017 require the repo to STATE the measured effect and to make the
// statement executable. build-report.md, the third site, is gitignored and lives
// outside the worktree, so it is dispositioned manual+checklist to the Auditor
// rather than pinned by a predicate reading an unreachable path.
```

### `go/acs/cycle1685/predicates_test.go:804` — above `if !strings.Contains(tc.report, "## Decision Trace") {`

```text
// Premise 1: the report carries the "## Decision Trace" whose mere
// presence was round 1's stated evidence for neutrality.
```

### `go/acs/cycle1685/predicates_test.go:1057` — above `func TestC1685_017_WideningEffectIsPinnedByATrackedInPackageTest(t *testing.T) {`

```text
// TestC1685_017_WideningEffectIsPinnedByATrackedInPackageTest is the durable
// half of H1's remedy, and the lesson under it: the round-1 claim was never
// RUN. A corrected sentence that is still only a sentence is the same artifact
// one measurement later — the next reader has no way to re-derive it, and the
// gitignored corpus it was measured over is not in the repo.
//
// So the corrected claim must be carried by a git-TRACKED test inside the
// package it is a claim about, which asserts the widening's effect hermetically
// and passes. The `--- PASS:` line is required because `go test -run <name>`
// whose pattern matches NOTHING still exits 0 — the vacuous-pass hole.
```
