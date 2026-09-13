# 2026-09-12 — a no-work cycle labelled `SHIPPED_VIA_BUILD` because a sibling lane moved main

## Impact
Cycle 1630 (two-wave health batch, wave 1, lane
`self-consistency-on-decision-phases`): scout PASSed, triage PASSed with an
honest `top_n: []` (its atomic inbox claim had failed — see ADR-0100), and the
host ended the cycle `triage-empty-commitment` after two phases. The printed
result was nevertheless

```
"FinalVerdict": "SHIPPED_VIA_BUILD", "PhasesRun": ["scout","triage"],
"TerminationReason": "triage-empty-commitment"
```

(`.evolve/loop-20260912-healthcheck.log:197`). Four consumers then misread a
cycle that had shipped nothing:

1. `IsTriageNoWorkResult` was false (it requires `SKIPPED_UNKNOWN`), so the
   sequential loop's no-work ledger floor (`completedTriageNoWork`) did not
   recognise the planned no-work disposition.
2. The throughput recorder credited the cycle (`shippedOutcome` saw a shipping
   verdict plus HEAD movement) — a committed-floor count for zero committed work.
3. The closeout dossier recorded `PASS` (`dossierVerdict` maps
   `SHIPPED_VIA_BUILD` to PASS): `knowledge-base/cycles/cycle-1630.json`.
4. The batch's goal-stall and non-progress breakers saw a shipping verdict, so
   an empty cycle did not count toward either escalation.

## Root cause
`finalizeOutcome` relabelled ANY `SKIPPED` cycle verdict as `SHIPPED_VIA_BUILD`
when the main HEAD probed at cycle start differed from the one probed at
closeout. That belief dates from cycle-107 (2026-05-26), when a build phase
could invoke `evolve ship --class manual` inline and the formal ship phase never
ran. Two facts make the proxy unsound today:

- In fleet mode a sibling lane moves HEAD constantly. Cycle 1630's pre-cycle HEAD
  was `a2e6095`; a sibling landed during its two phases.
- No persona instructs an inline ship any more, and lane commits carry no
  per-cycle trailer, so HEAD movement can never be attributed to THIS cycle.

The repo had already learned this once: `lost_landing_floor.go` deliberately
keys on the cycle's own ship artifacts, "NOT git HEAD movement", for the mirror
case (a PASS that lost its landing). The outcome label kept the old proxy.

## Fix (PR: fix/shipped-via-build-requires-own-ship — ADR-0100 PR-3)
- **The latch has one persisted home.** `cyclestate.CycleState.Shipped`
  (`json:"shipped,omitempty"`, beside `PreCycleHEAD`, the checkpoint field it
  mirrors) records that THIS cycle's ship phase PASSed and survived the
  deliverable review. `latchShippedState(cs, phase, verdict)`
  (`core/cycle_outcome.go`) is the one rule, called by both dispatch roots at
  the same moment: the fresh loop in `cyclerun_postreview.go` (the site that
  used to set the in-memory `cycleRun.shipped`, now removed) and the resume loop
  in `resume_execution.go` right after `reviewResumedDeliverable` approves and
  before the completion record persists the checkpoint.
- **Readers ask the checkpoint.** `finalizeCycle` →
  `finalizeOutcome(lastPhaseVerdict, retroDecision, cs.Shipped)`;
  `postShipObserverSkip(p, cr.cs.Shipped)` on both retry-option sets. The
  `finalizeCycle` signature is unchanged (it already receives `cs`).
- **The HEAD probe survives only as corroboration** inside the throughput hook
  (`shippedOutcome`: a shipping verdict whose landing left no commit is not
  throughput) and was moved next to that use. Its doc comments
  (`orchestrator.go`, `workspace_guard.go`, `cyclerun.go`,
  `cyclestate/verdict.go`; an orphan doc in `git_porcelain.go` and one in
  `cycle_outcome.go` deleted) and the operator reference row
  (`docs/operations/runtime-reference.md`, "Cycle outcome label") now say so.
- **Why the checkpoint and not a field on the closing `cycleRun`:** the first
  draft threaded a `shipped bool` from the fresh root's in-memory field through
  `finalizeCycle`. The architecture review found the resume root builds its own
  `cycleRun` at `resume_execution.go` and would always have forwarded `false`
  — a shipped cycle that paused after ship and resumed into a post-ship phase
  whose SKIPPED verdict became final would have been labelled `SKIPPED_UNKNOWN`
  (false "ended without shipping" WARN, no throughput, dossier WARN, breaker
  counts no progress). The evidence this fix replaced (`PreCycleHEAD`) already
  survived the resume boundary; the replacement had to as well.

## Reachability (stated so the next reader need not re-derive it)
A SKIPPED final verdict after a ship PASS is unreachable in the default
pipeline: the floor guard (`final_verdict_floor.go`) declines post-ship
non-floor verdicts, and `loopAbort` returns before `completeCycle`. It is
reachable only through a configured ship-floor override whose post-ship floor
phase SKIPPED, on either root (a pause/resume after ship keeps the latch). That
is why the closeout seam is pinned by a hand-built `cycleRun` →
`completeCycle` table and a checkpoint-driven resume test rather than a
composed default run.

## Regression
- `core/shipped_via_build_own_ship_test.go::TestRunCycle_EmptyTriageCommitmentSurvivesSiblingLanding`
  drives cycle 1630's exact shape through the composed `RunCycle` path — a real
  empty `triage-decision.json`, a HEAD probe that answers differently at start
  and closeout — and asserts `SKIPPED_UNKNOWN`, `IsTriageNoWorkResult`, zero
  throughput credits, and no persisted latch. RED on the old code on the first
  three assertions (observed before the fix), GREEN after.
- `::TestCompleteCycle_ForwardsTheShipLatchToTheOutcomeLabel` — the closeout
  seam: `completeCycle` → `finalizeCycle` reads `cs.Shipped` (both rows).
- `::TestRunCycle_ShipPassPersistsTheShipLatch` — the fresh root's writer site:
  a ship PASS lands `shipped: true` on the persisted cycle state.
- `core/shipped_via_build_own_ship_integration_test.go::TestResumeLifecycle_ShipPassPersistsTheShipLatch`
  (integration-tagged, like its fixture) — the resume root's writer site.
- `::TestResumeLifecycle_PostShipSkippedVerdictReadsTheCheckpointLatch` — the
  resume root labels a post-ship SKIPPED by the checkpoint's latch (both rows).
- `cyclestate/shipped_latch_test.go::TestCycleState_ShippedRoundTripsAndIsOmittedWhenFalse`
  — the field rides the checkpoint under `shipped` and is omitted when false,
  so pre-latch cycle-state files decode/encode unchanged.
- `core/orchestrator_outcome_test.go::TestFinalizeOutcome_SkippedWithoutOwnShip_NeverShippedViaBuild`
  + `::_SkippedWithOwnShip_ShippedViaBuild` + `::_OwnShipTrumpsAdvisory` pin the
  finalizer's table on the latch. The two former tests that encoded the
  HEAD-moved belief (`SkippedWithHeadMoved_ShippedViaBuild`,
  `HeadMovedTrumpsAdvisory`) were flipped, not deleted.
- Mutation proof (each mutant confirmed to BUILD before its run): fresh-root
  latch call dropped → killed by `TestRunCycle_ShipPassPersistsTheShipLatch`;
  resume-root latch call dropped → killed by
  `TestResumeLifecycle_ShipPassPersistsTheShipLatch`; `finalizeCycle` reads
  `false` instead of `cs.Shipped` → killed by the seam table and the resume
  label test; `latchShippedState` never sets the flag → killed by both writer
  tests; `if shipped` → `if true` / `if false` in `finalizeOutcome` → killed by
  the finalizer table, the composed no-work test, the seam table and the resume
  label rows.

## Follow-ups (filed, not done here)
- `SHIPPED_VIA_BUILD` is now producible only when a SKIPPED final verdict follows
  a ship PASS (see Reachability), so the label is nearly retired as a producer.
  Retiring it from the vocabulary needs reader migration (dossiers,
  `state.json` history, `IsShippingVerdict`, the loop breakers) and is its own
  slice.
- `shippedOutcome` still pairs the verdict with HEAD movement; deriving the
  corroboration from the ship binding instead would remove the last HEAD read.
- The resume root's retry-option set (`resume_execution.go`, `retryHooks`)
  carries no `postShipObserverSkip` hook, so a post-ship observer failure on a
  resumed cycle still aborts instead of degrading to WARN — a pre-existing
  parity gap. With the latch on the checkpoint, wiring that hook is a one-line
  follow-up.
