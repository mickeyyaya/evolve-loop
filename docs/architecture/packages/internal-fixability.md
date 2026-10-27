# internal/fixability

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 1 to 6. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) items 2, 3 and 11. Case: [cycle 2035](../../incidents/2026-10-27-cycle-2035-repair-loop.md). Callers: `internal/core` (`Orchestrator.repairHandoff`, before a repair round) and `internal/inboxmover` (`RankLaneMenu`, the lane menu of triage, the census, the wave seed and `evolve inbox batches`).

## Purpose

`internal/fixability` answers one question before a repair round: can the lane fix each finding that blocks? A repair round that cannot converge costs about 50 LLM minutes (cycle 2035, rounds 1 and 2). The package is pure. It reads no file, and the caller gives the findings, the gate failures, the open console items and the fence.

## Design

- **A finding.** `parseFinding` reads the id and the severity from the prefix of a defect line (`M1: ...`). `C` is CRITICAL, `H` is HIGH, `M` is MEDIUM and `L` is LOW. It collects each cited path under `go/`, `docs/`, `skills/`, `agents/`, `schemas/` and `.evolve/`.
- **Hand-off or lane.** A hand-off finding has a path on the fence (`Input.Protected`), or it names an open console item. In production, the fence is `guards.IsProtectedSurface`. The id must be whole: `x-console-two` does not name `x-console` (`TestNamesItem_NeedsTheWholeID`). All other findings are lane findings.
- **A finding that blocks.** The rules run in this order, and `Finding.Blocks` names the first that holds:

  | Rule | Value | Example in cycle 2035 |
  |---|---|---|
  | a gate failure names a path or a test of the finding | `gate` | round 3: the ship gate names `go/internal/core/failurelearning` |
  | the severity is HIGH or CRITICAL | `severity` | H1 is HIGH, but it is a lane finding |
  | the words "tests fail" or "predicate failed", next to each other, with at most a count between them | `red_test` | round 1: "floor tests fail (3) with this diff" |

  The `red_test` rule is the deterministic form of "the audit proves a red test" (ADR-0158 decision 4). The audit has no structured field for its proof, so the rule reads the defect line. The fail verb must follow the noun, so a prescription such as "add a test that fails before the fix" and a negation such as "the tests no longer fail" do not block (`TestClassify_ARedTestNeedsTheFailVerbNextToTheTests`).
- **The decision.** `Classify` returns the lane findings, the hand-off findings, the blockers, the console items and the salvage pointer. A hand-off finding that blocks gives `Stop` with the reason `handoff_blocker`. A hand-off finding that does not block (M2 of cycle 2035) is listed and does not stop.
- **Open console items.** `OpenConsole` keeps each inbox item that `inboxbatch.ConsoleRouted` routes to the console: a console route, or a file on the fence. It is the one owner of the rule, so a name that ends with `-console` is not enough (`TestOpenConsole_KeepsTheRoutedAndTheProtectedItemsAndNotANameAlone`). In cycle 2035, `retro-minting-files-through-inbox-add-lane-ac-5-console` named the protected `go/internal/core/failurelearning` floor tests. That path was on the fence at the base of the cycle, so the item was routed.
- **Companions.** `Companions(laneID, open)` gives the open console items that a lane item waits on. A console item is a companion in three cases:
  - its id starts with the lane id and a dash;
  - its id starts with the lane id without its `-lane` suffix, and a dash;
  - its `deps` or `connects_to` names the lane item. The inbox item has no notes field, so `connects_to` takes the place of "notes" in the item acceptance.
- **The signal names.** `SignalCode`, `FieldDecision`, `FieldReason`, `FieldBlockers` and the three decisions are the names that the writer (`internal/core`) and the readers (`internal/wave`, `internal/pipelinerates`) share.

## Where the decision acts

| Caller | Stage `shadow` | Stage `enforce` |
|---|---|---|
| `core.Orchestrator.repairHandoff`, called by `decideAfterAuditFail` when the envelope offers a repair round | `REPAIR_HANDOFF` with `decision: would_stop`; the repair round of today runs | `REPAIR_HANDOFF` with `decision: stop`; `repair-handoff.json`; retro with no repair round and no adjudicator |
| `inboxmover.RankLaneMenu` in the triage prompt, in `orchestrator.Census`, in the wave seed (`triagecap`) and in `evolve inbox batches` | the lane item stays ready; the census records `would defer <id>: console_companion_pending: <ids>` | the lane item waits with that reason, so triage cannot select it and the census counts it as `waiting_deps` |

The inputs of `repairHandoff` are the failure block of the audit, the gate failures and the open console items of the inbox. The gate failures are `AuditFailReasons` and the message of `ship-error.json`. `repairHandoff` emits the signal only when a hand-off finding exists, so a cycle with lane findings alone records nothing.

The hand-off seal (`core.sealHandedOff`) does three things:

- It sets the termination `handed-off`.
- It puts a `HANDOFF_ROUTE` diagnostic for the committed item on the audit timing entry. `cycleRun.consoleRouteItem` resolves the item, the same rule as the blocker exit. A cycle with no single item gets `ORCHESTRATOR_BLOCKER_REFUSED` and no route. `HANDOFF_ROUTE` has the disposition of `BLOCKER_ROUTE`: task level, a route to the console, and it outranks the audit sentinel. Its own code lets a reader tell the fixability stop from a proven blocker. The failure closeout thus routes the item to the console and charges no capability failure.
- The loop does not count the cycle as a code FAIL (`core.IsHandedOffResult`). The resume path keeps the termination through its cursor and gets the same diagnostic.

`evolve wave status` prints `handed-off (<reason>: <blockers>)` on the line of the cycle. The wave measures and PQ5 count the `stop` decisions, the `would_stop` decisions and the false stops ([internal-wavemeasure](internal-wavemeasure.md)).

## Invariants

- The four failure blocks of cycle 2035 are the fixtures (`testdata/cycle-2035`). Round 1 stops on M1 with the console item and the salvage pointer (`TestClassify_Cycle2035Round1StopsOnTheProtectedBlockerM1`). Round 3 has the lane findings H1 and M3 and the hand-off findings M1 and M2, and it stops on M1 by the gate rule. Round 4 stops on M1.
- H1 alone does not stop (`TestClassify_TheLaneFindingH1AloneDoesNotStop`). M2 alone is a hand-off finding and does not stop (`TestClassify_ANonBlockingHandoffFindingIsListedAndDoesNotStop`).
- At `enforce`, the first audit of cycle 2035 ends the cycle with no repair round (`core` `TestDecideAfterAuditFail_Cycle2035AtEnforceHandsOffAtItsFirstAudit`). At `shadow`, round 3 records `would_stop` and repairs as today (`TestDecideAfterAuditFail_Cycle2035AtShadowRecordsWouldStopAndRepairsAsToday`).
- At `enforce`, triage defers `retro-minting-files-through-inbox-add-lane` while its `-ac-5-console` companion is open (`inboxmover` `TestRankLaneMenu_EnforceDefersALaneItemWithAPendingConsoleCompanion`). The wave seed leaves the item out at `enforce` (`triagecap` `TestReadRankedBacklog_AtEnforceTheWaveSeedLeavesOutALaneItemWithAConsoleCompanion`).
- The tests use a local fence, because `guards` imports `core` and `core` imports this package.

## Limits

- The `red_test` rule trusts the words of the audit. A defect line that says "tests fail" for a reason outside the diff can stop a cycle. The shadow wave counts each such case as a false stop.
- A finding with no id prefix has no severity, so only the gate rule and the `red_test` rule can make it block.
