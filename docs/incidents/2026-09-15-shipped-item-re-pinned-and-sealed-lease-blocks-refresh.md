# 2026-09-15 — a shipped item was re-pinned to a lane, and a sealed lane's lease blocked the boundary refresh

**Cycles:** 1682 (FAIL, 2 phases), 1683 (FAIL, 2 phases), 1679 (audit rounds 4–5). **Wave:** pipeline-health verification wave 4, plane e9abdc55. **Class:** pipeline defects, console-owned.

## What happened

1. Wave 4's first wave went 0/2. Lane 1682's scope pinned one item, `crossartifact-invariant-stack`, already shipped by 1679 an hour earlier; scout verified the work was delivered and triage committed to nothing (`triage-empty-commitment-claimable-work`). Lane 1683 was handed `integration-tier-tmux-live-dispatch-exclusion` (kind `pipeline-repair`); triage's protected-surface breaker refused it and routed it console-manual — the breaker working, but after a scout and a triage.
2. At both wave boundaries the loop logged `LOOP_BOUNDARY_REFRESH_SKIPPED step=lane_active` with no lane running, so the plane binary stayed behind a HEAD that 1679 had moved in `go/`.
3. 1679's audit round 5 WARNed on three MEDIUM defects two of which round 4 had already named, plus an unsubstituted `FULLSUITE_PLACEHOLDER` in the build report, and classified two full-suite reds as its own sandbox denying writes under `.evolve/profiles`.

## Root causes (research F14–F18)

| # | Root cause | Fix |
|---|---|---|
| F18 | `ResolveDispatchState` had no `consumed/` state and read `processed/` flat while the promoter nests it by cycle — every shipped item resolved `unknown`, which both prunes and the launch probe keep | `StateConsumed`; every retirement dir scanned flat and by `cycle-<N>` via one `inboxbatch.CycleDirs`; fixtures write the real layout |
| F17 | `FleetLaneActive` counted heartbeat freshness; a sealed lane's lease stays fresh for a TTL | owner liveness (`runlease.OwnerLive` + the one `runlease.PIDAlive` probe) |
| F14 | no floor check for the persona scaffold's `_PLACEHOLDER` slots | `core.PlaceholderTokenFailures` in the default floor engine |
| F15 | two tests wrote decoys into the live `.evolve/profiles`; two walkers descended into `docs/private` | temp git mirrors of the tracked profiles; walkers skip a denied dir visibly |
| F16 | a recovery rebuild after a WARN audit carried no audit section (the repair brief seeds only behind a rejection) | `## Standing Audit Findings` seeded on ship-error recovery |

## Cost

Two lanes × (scout + triage) ≈ 12 minutes and two audit rounds' worth of classification on 1679; the ship streak toward five consecutive shipped cycles reset at one.

## Verification

Red-first tests per fix (`dispatchstate_test.go`, `cycledirs_test.go`, `launcher_test.go` beliefs, `driver_test.go` dead-owner, `pidalive_test.go`, `build_placeholder_check_test.go`, `audit_repair_context_test.go`, `standing_findings_prompt_test.go` ×2, the two decoy tests on mirrors), whole-module floor, integration tier on core/ship/cmd. Live proof: the next wave's plan must not pin a `consumed/` id, and the boundary refresh must run once no lane process is alive.

## Rules kept

- The loop is completion-driven: a boundary is the ~40 s `lanes=0` window after `[loop] wave N: x/y lanes ok`; bounce only when a fix PR is parked (`feedback_no_pr_merges_mid_wave`).
- A test that mutates tracked repo config, or walks an operator-private directory, is the defect — never a sandbox exemption.

## Addendum — lane 1684 (research F19)

1684's audit FAILed with an invented class (`superseded-predicate-contradiction`); the gate accepted it, the envelope declined the direct repair with no line anywhere, a retrospective (deep tier, ~10 min) adjudicated a retry, and the retry's build brief carried none of the audit's findings. Fixed at the class: the gate validates the audit's class and the prompt names the vocabulary; the decision is a coded signal; a retro-routed re-entry carries the standing findings.

## Addendum — the boundary halt (research F20)

Stopping the loop at the wave-4 boundary took SIGINT ×2, SIGTERM and finally SIGKILL: the pre-wave usage probe and CLI-health canary ran on `context.Background()`, so the loop's interrupt never reached them, and a wave was still dispatched (and cancelled at spawn) after the interrupt. Fixed at the class: both probes take the loop's context, the prober's wait is bounded by it, and the coordinator re-checks the interrupt after the probes.

## Addendum — lane 1685 (research F22)

A red_count=0 audit with every criterion evidenced sealed FAIL: the auditor's verdict was fenced JSON without the sentinel wrapper; the gate salvaged and approved the repaired report while the runner had classified the unrepaired bytes, and the audit-fail envelope declined a repair for want of a class (the F19 signal named it in one line). Fixed at the class: one verifier — the gate's own Reviewer verifies for the engine (`VerifyForClassification`), salvaging, persisting and reporting before classification; every BaseRunner is handed an accessor to it at the composition root.

## Addendum — lane 1688 (research F25)

The first wave on v22.24.0 burned its first seal on a `pipeline-repair` item with no files list: the plan-time classifier could not see its surface, the triage breaker could, and the refusal came after two LLM phases. The kind is now a routing input in the one ADR-0074 classifier, so pipeline-integrity items never reach a lane.
