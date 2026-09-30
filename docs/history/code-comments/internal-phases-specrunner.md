# Comment history: `internal/phases/specrunner`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/specrunner/exported_classify_test.go:3` — above `import (`

```text
// RED-phase contract for cycle-249 task `phase-classify-declarative`:
// the declarative verdict evaluator must be EXPORTED as EvaluateClassify
// so built-in phases (triage, tdd, intent, build) can delegate their
// hand-coded classify logic to the one shared evaluator.
//
// The unexported evaluator's full matrix is already covered by
// TestEvaluateClassify in specrunner_test.go — these tests pin the
// EXPORTED surface only (signature + the contract rows built-in phases
// will rely on), so they complement rather than duplicate.
//
// Fails at baseline: EvaluateClassify is undefined (compile RED).
```

### `go/internal/phases/specrunner/exported_classify_test.go:107` — above `func TestEvaluateClassifyExported_HeadingAwareSections(t *testing.T) {`

```text
// Pins heading-aware require_sections matching (inbox
// classify-heading-prefix-mismatch, 2026-06-07) — semantics documented on
// hasSection in specrunner.go. Includes strict-superset cases proving legacy
// matches are preserved.
```

### `go/internal/phases/specrunner/exported_classify_test.go:119` — above `name:        "bare rule matches h2 heading",`

```text
// The cycle-249/250 regression: bare rule vs "## " heading.
```

### `go/internal/phases/specrunner/exported_classify_test.go:182` — above `func TestEvaluateClassifyExported_DiagnosticNamesMissingSection(t *testing.T) {`

```text
// The missing-section diagnostic must NAME the missing section so a phase
// author can debug a FAIL from the message alone (easy-to-debug scaffold
// is an explicit cycle-249 goal).
```

### `go/internal/phases/specrunner/specrunner.go:103` — above `func EvaluateClassify(artifact string, rules *phasespec.ClassifyRules) (string, []core.Diagnostic) {`

```text
// EvaluateClassify is the declarative verdict evaluator shared by specrunner and
// built-in phases. Pure function (no I/O) so it is exhaustively unit-testable.
//
//   - empty artifact → FAIL when rules are absent or rules.FailIfEmpty is set
//     (rules present with FailIfEmpty unset → an empty artifact is allowed to
//     pass; the operator opted out explicitly)
//   - every require_sections header must be present as a line-anchored markdown
//     header, else FAIL
//   - fail_if_signal is parsed but CANNOT be evaluated here — it needs the
//     Stage 3 signal bus. A non-empty fail_if_signal is a HARD FAIL (the
//     cycle-241 declared-semantics rejection: an inert gate must fail loudly,
//     never silently pass — retro 215-231 Practice 4). The repo-catalog CI
//     guard (phasespec.TestRepoPhaseCatalog_NoInertFailIfSignal) enforces the
//     same invariant at authoring time so the rejection never fires mid-cycle
//     (cycle-263: 15 mis-authored catalog phases hit this in production).
//   - rules.VerdictOnPass overrides the pass verdict, but must be a canonical
//     verdict (guards against silent typos in user phase JSON); else FAIL
//   - rules.VerdictFromSentinel lets a JUDGMENT phase's own stated verdict
//     decide (verdict_from_sentinel.go). Absent = byte-identical legacy
//     behavior; "shadow" records the disagreement without routing on it;
//     "enforce" makes the stated verdict authoritative. Runs LAST, so a stated
//     verdict can never launder a structurally broken artifact, and fails open
//     on an unreadable sentinel. An unknown stage word is a hard FAIL.
```

### `go/internal/phases/specrunner/specrunner.go:200` — above `func hasSection(artifact, section string) bool {`

```text
// hasSection reports whether section appears as a line-anchored markdown
// header (the section text begins a line). Matching is heading-aware:
// markdown heading markers (a "#"-run followed by whitespace) are stripped
// from BOTH the rule and the line before the prefix compare, so
// the rule "Baseline" matches "## Baseline" and the rule "## Findings" matches
// a bare "Findings" line — one semantic, no dual matching modes (inbox
// classify-heading-prefix-mismatch, 2026-06-07). Line anchoring still rejects
// mid-line occurrences.
```

### `go/internal/phases/specrunner/specrunner.go:237` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/specrunner/specrunner_test.go:73` — above `"fail_if_signal without signal bus → FAIL (authoring-time rejection)",`

```text
// cycle-241 declared-semantics-rejection: a fail_if_signal gate
// without the Stage-3 signal bus can never fire — silently passing
// it lets an authoring mistake reach runtime undetected (retro
// 215-231 Practice 4). Loud authoring-time FAIL, not WARN.
```

### `go/internal/phases/specrunner/specrunner_test.go:115` — above `func TestEvaluateClassify_FailIfSignal_RejectsWithErrorSeverity(t *testing.T) {`

```text
// TestEvaluateClassify_FailIfSignal_RejectsWithErrorSeverity pins the
// severity of the cycle-241 declared-semantics rejection: the diagnostic
// naming fail_if_signal must be Severity "error" (not "warning") AND the
// verdict must be FAIL. The table above checks verdict+message; this test
// is the severity pin the table's shape cannot express.
```

### `go/internal/phases/specrunner/verdict_from_sentinel.go:3` — above `import (`

```text
// verdict_from_sentinel.go — letting a judgment phase's OWN verdict decide.
//
// The defect: EvaluateClassify judged a spec-driven phase from STRUCTURE ONLY.
// Sections present and non-empty meant PASS, so a phase whose entire job is to
// render judgment could conclude "FAIL (BLOCK). The cycle must not proceed as
// framed" — and emit the canonical machine sentinel saying exactly that — while
// the orchestrator classified it PASS and ran the cycle to completion
// (cycle-1528, whose ignored objection was correct: the redesign it forced
// shipped as ADR-0090). The loop paid for a full agent dispatch every cycle and
// discarded its conclusion.
//
// The signal was never missing — every judgment report on disk carries a
// well-formed sentinel. So this is a WIRING fix, and deliberately not a new
// grammar: it reuses phasecontract's sentinel parser — the same one the
// contract gate and the verdict cache already read — rather than inventing a
// second way to say "FAIL" that could drift from the first.
//
// Why a rollout stage and not a switch: the population is UNCALIBRATED. A
// verdict emitted for years into a void does not get corrected, because nothing
// ever contradicted it, and turning it authoritative in one step would halt
// nearly every cycle at that phase. Shadow measures the population first. The
// measured counts live in ADR-0091 (docs/architecture/adr/0091-*) and are
// deliberately NOT repeated here — they move every cycle, and a stale number in
// a comment is worse than a pointer to the place that owns it.
```

### `go/internal/phases/specrunner/verdict_from_sentinel.go:130` — above `return structuralOnly(core.VerdictFAIL, append(o.diags, core.Diagnostic{`

```text
// A typo'd stage silently disables the gate, and an inert gate must
// fail LOUDLY — the cycle-241 declared-semantics rule this classifier
// already applies to fail_if_signal and verdict_on_pass. Failing here
// costs one cycle; passing silently costs however long nobody notices.
```

### `go/internal/phases/specrunner/verdict_from_sentinel_test.go:3` — above `import (`

```text
// verdict_from_sentinel_test.go — a judgment phase's STATED verdict must be able
// to reach the orchestrator.
//
// The defect these tests pin (inbox judgment-phase-semantic-verdict-never-read,
// weight 0.93): EvaluateClassify decided a spec-driven phase's verdict from
// STRUCTURE ONLY. cycle-1528's premise-challenge concluded "FAIL (BLOCK). The
// cycle must not proceed as framed" with premise.severity_max == CRITICAL, AND
// emitted the canonical machine sentinel saying FAIL — and the cycle ran on
// through tdd, build, adversarial-review, audit, retro. Measured across this
// repo's whole run history: 225 of 225 judgment reports carry a well-formed
// sentinel, 100 of them say FAIL, and every one classified PASS.
//
// The fixtures are REAL artifacts, not synthetic ones, because the acceptance
// criterion is that the LIVE population parses — a hand-written fixture proves
// only that the parser handles what its author imagined.
```

### `go/internal/phases/specrunner/verdict_from_sentinel_test.go:30` — above `func realPremiseChallengeFAIL(t *testing.T) string {`

```text
// realPremiseChallengeFAIL is cycle-1528's verbatim report: the live objection
// that was correct (it falsified the plan's load-bearing premise and the
// resulting redesign shipped as ADR-0090) and changed nothing.
```

### `go/internal/phases/specrunner/verdict_from_sentinel_test.go:38` — above `func realAdversarialReviewPASS(t *testing.T) string {`

```text
// realAdversarialReviewPASS is cycle-1453's verbatim report — a genuine PASS,
// so the no-false-positive direction is pinned against live bytes too.
```

### `go/internal/phases/specrunner/verdict_from_sentinel_test.go:141` — above `func TestEvaluateClassify_UnknownStage_FailsLoudly(t *testing.T) {`

```text
// A typo'd stage must FAIL LOUDLY, never silently disable the gate — the same
// cycle-241 declared-semantics rule EvaluateClassify already applies to
// fail_if_signal and verdict_on_pass.
```
