# Retired ACS predicates: the recurrence weight writer (2026-11-21)

> Archived by the console for the inbox item `inbox-prioritization-p5-reason-on-priority-change` (plan [P5](../../../../plans/inbox-prioritization-2026-10.md#p5-landing-notes-2026-11-21)). P5 removes the second writer of an item weight. These predicates asserted what that writer did. They are archived under the A2 retention rule of [the quality gate](../../../../architecture/acs-predicate-quality-gate.md).
>
> The rule: never delete, never edit silently. Each file keeps its original package clause and body. The files live outside the Go module and never build. The rest of each package stays live in `go/acs/`.

## What was archived and why

| Predicate | It asserted | Why it is retired | What pins the contract now |
|---|---|---|---|
| `go/acs/cycle1932`: `TestC1932_001_RecurrenceReweightAppendsAnInboxLifecycleEditLine` | A recurrence escalation appends one inbox lifecycle `edit` line. | The boundary applier writes no weight, so it writes no `edit` line. | `cmd/evolve` `TestApplyEscalationBoundary_RecurrenceMovesRankAndLeavesWeight` counts zero `edit` lines for the item. |
| `TestC1932_002_RecurrenceReweightRefusesAWeightTheCuratorRuleRejects` | The reweight seam refuses a weight that the curator rule rejects. | The seam is gone. | `lifecycle` `TestMover_Edit_RefusesAResultInboxAddWouldRefuse` pins the rule for every weight write. |
| `TestC1932_003_RecurrenceReweightNeverLowersAndLeavesTheItemUntouched` | The reweight never lowers a weight, and a refused reweight leaves the item untouched. | No recurrence weight write exists. | `recurrence` `TestApplyBoundary_AnEscalationWritesNothingIntoTheInbox`. |
| `TestC1932_005_RecurrenceDelegatesTheItemWriteToTheInjectedReweightSeam` | `ApplyBoundary` calls the injected `Reweight` once with the target weight. | The `Reweight` option is deleted. | `TestNoProductionCodeWritesAnItemWeightOutsideTheLifecycleWriter`. |
| `TestC1932_006_ARefusedSeamWriteIsNotReportedBumped` | A refused seam write leaves the item out of `Bumped`. | An open escalated item is always in `Escalated` now, because nothing can refuse. | `recurrence` `TestApplyBoundary_EscalatesThenIsIdempotent`. |
| `TestC1932_007_WithoutAReweightSeamRecurrenceWritesNoItem` | With no seam, recurrence writes no item. | It is now the rule for every case. | `TestApplyBoundary_AnEscalationWritesNothingIntoTheInbox`. |
| `TestC1932_008_TheLoopBoundaryCallerReweightsThroughInboxmover` | The loop boundary caller reweights through `inboxmover.Edit`. | `moverReweight` is deleted. | `TestRunLoop_EscalatesAtIterationBoundary` (it asserts the report lists the item and the weight is unchanged). |
| `go/acs/cycle1062`: `TestC1062_005_ApplyEscalationIdempotentPerCycleStamp` | The first apply raises the weight to `Target(0.80, 4)`. The second apply in the same cycle changes nothing. | The first half asserted the weight bump. The idempotency half is pinned elsewhere. | `recurrence` `TestApplyBoundary_EscalatesThenIsIdempotent`. |

## Amended, not retired

| Predicate | Change | Why |
|---|---|---|
| `go/acs/cycle1062` `TestC1062_007_PlanEscalationSkipsClaimedItems` and the `applyOpts` helper | `res.Bumped` is `res.Escalated`. The `Policy` field is dropped from the options. | The result field was renamed and the options field removed. The assertion is unchanged. |
| `go/acs/cycle2478` `fileThenEdit` | It calls `EditBecause` with a reason and a ledger that records nothing. | A weight edit needs a reason now. The predicates still prove that a rewrite keeps the filed format byte for byte. |

## Notes

- `TestC1932_004_InboxWriteScanEnrollsNoRecurrenceWriter` stays live in `go/acs/cycle1932`.
- The archived `cycle1932` file holds the helpers that its predicates used, so it reads as it ran. The live file lost those helpers and the imports that only they used.
