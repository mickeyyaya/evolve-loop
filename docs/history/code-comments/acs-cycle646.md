# Comment history: `acs/cycle646`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle646/predicates_test.go:3` — above `package cycle646`

```text
// Package cycle646 materialises the acceptance criteria for cycle 646's two
// triage-committed top_n tasks with a genuinely new (not pre-existing) slice:
//
//   - report-handoff-size-contract-scout: cycle-565 Slice S1 already shipped
//     CheckHandoffBudget/VerifyWithReportSize/the Reviewer wiring, but left
//     "shadow" and "advisory" behaviourally identical (both fully silent) —
//     not the staged WARN-then-enforce rollout the source spec and this
//     cycle's scout-report Task 2 describe. This cycle's slice makes
//     "advisory" the missing WARN rung: it records the violation
//     (non-blocking) instead of staying silent.
//   - persona-stop-criterion-dedupe: agents/evolve-{scout,builder,auditor}.md
//     each duplicate a structurally-identical "## STOP CRITERION" block with
//     zero shared wording (751 combined lines) — extract the shared structure
//     into one reference doc without losing any gate name or banned pattern.
//
// Task 1 (cache-stable-prompt-prefix-audit) is NOT predicated here: its
// underlying mechanism (go/internal/phases/runner's cycleContextBoundary /
// BaseCycleContext / StaticPrefix, cycle-535) is already shipped and covered
// by go/internal/phases/runner/staticprefix_test.go (pre-existing GREEN). The
// one remaining ordering gap — go/internal/adapters/bridge.Adapter.Launch
// puts CorrectionDirective/OperatorDirectives OUTERMOST, ahead of the static
// Rules/Policy/Contract/persona block — is INTENTIONAL, tested behavior
// (bridge_correction_test.go: TestCorrectionDirectiveComposesWithRules /
// TestLaunch_InjectsCorrectionBlock; bridge_directives_test.go:
// TestOperatorDirectivesComposeOrder / TestLaunch_InjectsOperatorDirectives —
// all assert "correction < directives < rules < body" as the REQUIRED order,
// for retry/directive salience). Inverting it to satisfy a literal
// "static-always-precedes-dynamic" reading would regress that shipped,
// tested salience feature. See test-report.md's disposition table — flagged
// manual+checklist for Auditor, not predicated.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…637
// precedent) — each predicate shells `go test -run '^Name$' <pkg>` over the
// real RED unit tests authored this cycle. RED now: TestVerifyWithReportSize_
// AdvisoryRecordsWarnViolation (deliverable) and TestPersonaStopCriterionDedupe_
// CombinedLineCountReduced (prompts) fail; see test-report.md's RED Run
// Output for the other three (pre-existing GREEN — they already hold and
// serve as regression/scope guards for the Builder's change).
```

### `go/acs/cycle646/predicates_test.go:79` — above `func TestC646_002_ReportSizeGateShadowStaysSilent(t *testing.T) {`

```text
// TestC646_002_ReportSizeGateShadowStaysSilent — negative/scope guard: shadow
// must remain fully dormant (cycle-565 contract) — only advisory gains WARN
// behavior this cycle. Pre-existing GREEN; guards against an over-broad fix.
```
