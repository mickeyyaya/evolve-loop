# 2026-11-20: inbox withdraw refused items that carry lifecycle stamps

Related documents: the review that chose the drops is in the memory note `pipeline_review_2026-11-20`. The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** at the boundary of wave 183, the console applied the inbox review: 81 withdraws, 13 merges and 4 consumes. `evolve inbox withdraw` refused 5 of the withdraws that it ran. 24 more withdraws carried the same stamps, and the console did not run them. The console closed these 29 items with `evolve inbox consume` and a resolution that starts with "retired, not built".
- **Root cause:** the verb refuses an item that has a field which the lifecycle verbs wrote. This refusal is by design. The runbook of the boundary did not check the stamps before it chose the verb.
- **Fix:** the console used `consume` for the 29 items. No code change is needed. The runbook must check the stamps first.
- **Lesson:** a runbook that plans a verb for many items must test the preconditions of the verb on each item before it runs.

## Terms used throughout

- **Lifecycle stamp:** a field that a lifecycle verb wrote to an item, for example `routed_at`, `routed_cycle`, `routed_reason` or `released_continuations`.
- **Withdraw:** remove an item that no cycle built. **Consume:** close an item with a resolution.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| about 19:10 (file times) | The console applies the 13 merges. 2 of the 13 source withdraws are refused. | `apply-merges.log` in the console scratch directory |
| about 19:11 | The console runs 57 plain withdraws. 54 succeed and 3 are refused. | `apply-withdraws.log` |
| about 19:12 | The console consumes the 29 items (24 routed, 5 refused) with a resolution of the form "retired, not built: ...". Each call returns rc 0. | `apply-consume-retired.log` |

The refusals:

```text
refusing to withdraw the item: acs-cycle334-does-not-compile carries routed_at, routed_cycle, routed_reason, which the lifecycle verbs wrote
refusing to withdraw the item: dead-api-sweep carries released_continuations, which the lifecycle verbs wrote
```

The 5 refused ids are `acs-cycle334-does-not-compile`, `wave-integration-i7-recompose-after-console-merge`, `chronicle-s7a-historian-shadow`, `dead-api-sweep` and `egps-regression-tia-selection`.

## Root cause

**F1.1** `refuseLifecycleStamps` (`go/internal/inboxmover/lifecycle/withdraw.go:73`) lists each key that a lifecycle verb owns and refuses the withdraw when it finds one. The filter `notLifecycleOwned` (`:88`) lets the filing stamps pass.
*Implication:* an item that was routed or released has a stamp, and a withdraw of it fails.

**F1.2** The runbook (`boundary-183-plan.md` step 4c) says: "Skip any id that is now processing/consumed". It does not say to check the stamps.
*Implication:* the plan did not catch the 5 refusals before the run.

**F1.3** The console listed 24 more withdraws as routed items in `withdraw-routed.tsv`. It did not run them as withdraws, and it consumed them.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The 29 items are consumed with a retire resolution. | the inbox PR of the boundary | none: a data change |
| The runbook checks each item for lifecycle stamps before it chooses `withdraw` or `consume`. | console practice | none: a practice, not code |

## Limits

- The plan had 94 withdraws: 81 withdraw rows and 13 merge sources. The console found 24 routed rows before it ran the verb and consumed them. It ran the other 70 as withdraws (57 plain rows and 13 merge sources). 65 succeeded and 5 were refused, and the console consumed the 5.
- The refusal is correct. The cost is one wrong verb choice for each item, and no data was lost.

## What we learned

- Run a dry check of the precondition on the whole list before the first state change.
- Use `consume` with an evidence resolution for an item that a lifecycle verb touched.
