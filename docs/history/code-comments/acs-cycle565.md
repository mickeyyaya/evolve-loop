# Comment history: `acs/cycle565`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle565/predicates_test.go:3` — above `package cycle565`

```text
// Package cycle565 materialises the cycle-565 acceptance criteria for the
// fleet-lane-committed top_n task `report-size-contracts-jit-artifacts`
// (Slice S1 only — see triage-report.md and
// .evolve/inbox/2026-07-05T14-30-00Z-report-size-contracts-jit-artifacts.json).
//
// Slice S1 scope (per the triage decision): extend the existing contract-gate
// with a per-artifact handoff-summary token/size budget (default ~2K,
// policy-configured, shadow/warn first) and restructure the build/scout/audit
// phase report contracts with a never-evict "## Handoff Summary" section
// (decisions, acceptance criteria, open questions, verdicts). S2 (dependency
// pruning) and S3 (JIT read) are explicitly out of scope for this cycle.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/
// 561/563 precedent) — each predicate shells `go test -run '^Name$' <pkg>`
// over the real RED unit tests authored this cycle in
// go/internal/phasecontract, go/internal/deliverable, and go/internal/policy.
// RED now (undefined identifiers — see test-report.md's RED Run Output);
// GREEN once Builder implements HandoffSummary/EstimateTokens/
// HandoffSectionContent/CheckHandoffBudget/VerifyWithReportSize/the Reviewer
// fields/GatesConfig.ReportSizeGate/Policy.ReportBudgetConfig.
```
