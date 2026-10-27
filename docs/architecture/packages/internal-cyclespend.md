# internal/cyclespend

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 14 and 15. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) items 1 and 4. Case: [cycle 2035](../../incidents/2026-10-27-cycle-2035-repair-loop.md). Callers: `internal/wave` (`ReadCycles`) and `internal/pipelinerates` (`LoadCycles`).

## Purpose

`internal/cyclespend` sums the LLM spend of one cycle. The cost measures of `evolve wave status` and the PQ5 readings of `evolve quality report --scope wave` read it. It is the one reader of the spend of a cycle, so the two reports sum each cycle the same way. They can still put a cycle into a different wave (the limit in [llm-quality-indexes.md](../llm-quality-indexes.md#pq5-cost-for-each-shipped-cycle)).

## Design

- **`Spend`.** It holds the LLM time in milliseconds, the fresh tokens, the cache-read tokens, the calls with no measured usage and the audit rounds.
- **Fresh tokens.** They are input, output and cache-write tokens. Cache-read tokens have their own field, because they cost less and grow with the context (ADR-0158, "Policy keys").
- **`Sum`.** It adds the rows of the ledger. A row with no `duration_ms` adds no time. A row whose `usage_status` is not `measured` counts in `UnmeasuredCalls`, so a reader can see when the token sums are low.
- **`Read`.** It reads the ledger of a workspace through `llmcalls.ReadWorkspace`. A workspace with no ledger spent nothing. A ledger that cannot be read returns an error that wraps `ErrLedger`. A malformed line is skipped, as `llmcalls.Read` does.
- **Audit rounds.** The ledger has no audit count. `Read` takes `AuditRounds` from `audit_dispatches` of `cycle-state.json` in the same workspace. A missing state file gives 0 rounds. A state file that cannot be parsed returns an error that wraps `ErrCycleState`, and `Read` keeps the ledger spend. `wave.ReadCycles` warns only on `ErrLedger`, because it already warns on a bad state file.
- **Lines that cannot be read.** A malformed line of the ledger counts as one unmeasured call, so the spend does not claim a full count.
- **`Add` and `LLMMinutes`.** A window sums the spend of its cycles with `Add`. `LLMMinutes` gives the time in minutes.
- **`Totals`.** It holds the started cycles, the shipped cycles and their summed spend. `PerShipped`, `AuditRoundsPerCycle` and `ConvergenceRate` are the one place of the arithmetic, and each returns false when its denominator is 0. `wavemeasure` and `pipelinerates` only round and publish the result.

## The spend limit

- **`Limits` and `Judge`.** `Judge(spend, limits, enforce)` checks the limits in this order: LLM minutes, fresh tokens, cache-read tokens, audit rounds. It names the first limit that the spend passes, with the value and the limit. At the limit, nothing stops: only a value above it counts. With `enforce` false, the action is `would_stop`. With `enforce` true, it is `stop`.
- **Where it acts.** `core.cycleRun.checkCycleSpend` asks `Judge` at each phase boundary of a fresh cycle, at the end of `recordAndBranch`. It reads the ledger of the workspace and takes the audit rounds from the live cycle state. It records `CYCLE_SPEND_LIMIT` (module `cycle`). At `shadow`, it records one `would_stop` for each cycle. At `enforce`, it records `over` at the first boundary over the limit where no stop is legal, and `stop` where the cycle stops.
- **A partial ledger.** When the ledger does not load whole, the check prints a WARN, judges the part that loaded and sets `partial: true` (`TestCheckCycleSpend_AnUnreadableLedgerWarnsAndChecksTheRest`).
- **The stop at `enforce`.** The cycle stops at the first failed phase that has a legal edge to retro, and that is a failed audit. A passing audit still ships, because its spend is already spent. The stop schedules retro and seals the cycle `handed-off` with the reason `spend_limit`, the limit and the salvage worktree ([internal-fixability](internal-fixability.md)). The seal has no findings and no console item, because a spend stop has none. When the fixability check sealed the same failed audit, the cycle keeps that one route (`TestCheckCycleSpend_AFixabilityHandoffOnTheSameAuditKeepsOneRoute`).
- **Cycle 2035.** The ledger passes 90 LLM minutes after audit round 2 (`TestJudge_Cycle2035PassesItsLimitOfLLMMinutesAfterAuditRoundTwo`). Audit round 2 was WARN, so the cycle went to ship, and ship refused (`REPO_CONTRACT_GATE`). At `enforce`, the stop comes at the failed audit round 3. It saves repair round 2 (tdd and build, about 51.6 LLM minutes) and audit round 4 (`core` `TestCheckCycleSpend_Cycle2035AtEnforceStopsAndHandsOff`).
- **The signal names.** `SignalCode` and `FieldDecision` are the names that `internal/core` writes and that `internal/wave` and `internal/pipelinerates` read.

## Invariants

- The ledger of cycle 2035 gives 160.1 LLM minutes, 4,338,241 fresh tokens and 87,431,829 cache-read tokens, as the incident record says (F1.1). The fixture is `testdata/cycle-2035` (`TestRead_Cycle2035SpentTheNumbersOfItsIncidentRecord`).
- Each limit stops alone and names its value (`TestJudge_EachLimitStopsAloneAndNamesItsValue`).
- At `shadow`, the cycle records one `would_stop` and changes no flow (`core` `TestCheckCycleSpend_ShadowRecordsWouldStopOnceAndChangesNoFlow`).
- A missing ledger is zero spend and no error (`TestRead_AWorkspaceWithNoLedgerSpentNothing`). An unreadable ledger is an error (`TestRead_AnUnreadableLedgerIsAnError`).

## Limits

- The resume path does not check the spend limit. A resumed cycle runs at most to its next seal.
