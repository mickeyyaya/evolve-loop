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

## Addendum — lane 1682's FAIL seal, and the no-work loop it would have started (F30, 2026-09-26)

F18 stopped a consumed id from being re-pinned, but it left the reason 1682 **sealed FAIL** untouched. Triage did its job: `triage-decision.json` dropped the one scoped item with `reason: already-shipped-cycle-1679`. The termination check (`hasClaimableInboxWork`) then asked whether the *whole inbox* held claimable work. It found other lanes' items and relabelled the honest no-work end `triage-empty-commitment-claimable-work`. The seal recorded that FAIL as "unexplained … infra death", so it fed the blocker breaker's unexplained-failure count.

The invariant is **"triage answered for every scoped item"**, and a scoped item is wherever triage left it:
- still pending in the inbox root, because triage builds its menu there (`inboxmover.ClaimLaneScope`'s placement note);
- or claimed into this cycle's `processing/cycle-<N>/`, because the persona's Step 0a claims before it selects.

- **One reader.** `committedset.Dispositions` / `DispositionsFrom` sit in the stdlib-only leaf beside `Committed`. An *answer* is an `escalate_block` (its reason, or its `fail_count`), a `skip_rejected`, a `skip_shipped` carrying its sha, or a `dropped[]` entry with a reason. The most severe bucket wins, so a drop's reason never masks a fraudulent-commit escalation. A deferral is not an answer: cycle 1623's triage narrated a claim failure as a deferral, and that row stays pinned.
- **Termination** (`core/triage_termination.go`, `unansweredClaimableWork`, which now takes the cycle).
  - A fleet lane is a claim failure only when a scoped id is dispatchable in the root **or** claimed by this cycle, **and** answered nowhere; otherwise the lane ends as planned no-work (SKIPPED).
  - Judging the root alone would have let triage's own claim turn a loud claim failure into a silent no-work end (architecture review C1).
  - A load warning fails the lane closed only for a scoped item the check cannot find, so one broken, unrelated inbox file never fails an answered lane (m5).
  - A sequential cycle's menu is the whole inbox, so it keeps the whole-inbox check.
- **Loop breaker** (`cycleoutcome.ApplyNoWork`, the sibling of `ApplyFailure`). It hands each scoped item the lane answered for to the console, in place: `RouteConsole`, `routed_reason: lane triage (cycle N) <bucket>: <evidence>`. Then it releases the cycle's claims to the root with no failure bump, so the item arrives console-visible and does not sit stranded in the gitignored claim directory (C1).
  - An item already routed to the console keeps its original evidence (m3).
  - An answered id no longer in the inbox is simply gone, with no route-not-found WARN (m4).
  - An id outside the lane's scope is never touched.
  - An unshipped lane never retires work: the console confirms and retires it.
- **Every root makes the one closeout** (`closeoutCycleOutcome`, `cmd/evolve/cmd_cycle.go`): the cycle-run root every fleet lane runs, the sequential loop, and `evolve loop --resume`, which can resume a fleet lane's checkpoint with its lane pin (M1). The resume root now also runs the failure walk for a FAIL (a cycle-level failure error included), closing a pre-existing gap where resumed FAILs never reached the inbox. Each root WARNs in its own voice.
- **Registry and comments.** `INBOX_ITEM_ROUTED_CONSOLE`'s registry text names both closeouts. The sibling-reader notes on `inboxmover.ClosedDroppedIDs` and `carryover.triageDroppedIDs` now name all three `dropped[]` readers (M2).

**Pins.**
- `core/triage_termination_lane_test.go`, composed `RunCycle` where the decision is consumed:
  - answered ×4 → planned no-work;
  - silent, deferred, reasonless drop, or another id answered → claim-failed;
  - claimed via the real `inboxmover.Claim` then omitted → claim-failed; claimed and answered → no-work;
  - an unrelated malformed file does not fail an answered lane, but still fails closed when the scoped item is missing;
  - a **sequential** triage that claims the queue's last item and then commits nothing stays claim-failed, because the whole-inbox check now reads this cycle's claim directory too (the sequential shape of C1, re-review MINOR 5).
- `committedset/dispositions_test.go`: severity order and evidence rules.
- `cycleoutcome/nowork_route_test.go`: scope, the ledger seam, releasing what the lane claimed with no bump, keeping the console's evidence, skipping what is gone.
- `cmd/evolve/cycle_nowork_outcome_test.go`: the cycle-run root, the sequential loop, the applied labels.
- `cmd/evolve/cmd_loop_resume_nowork_test.go`, integration tier: the resume root's no-work hand-off, and its failure walk for a FAIL verdict and for a cycle-level failure error (re-review MAJOR 1).

**Mutation-checked.** Each of these goes red:
- a deferral counts as an answer;
- claimability is ignored;
- a reasonless drop answers;
- the lane-scope check is dropped from the hand-off;
- the root's no-work branch is removed;
- the root hands over on any SKIPPED;
- the resume root makes no closeout;
- the claim directory is ignored;
- the claims are not released.

**A quota wall at resume.** It pauses before the closeout, so the claims wait for the resumed attempt, while the cycle-run root and the sequential loop walk it as a cycle-level failure. The parity is filed as `quota-pause-closeout-parity`.

**Kept, deliberately.** A lane that *defers* its only scoped item still seals claim-failed, and the seal still reads as unexplained. That is the cycle-1623 rule; giving it an explained, non-infra classification is left to the triage-FAIL diagnostics (review m6).

**Operating note: retiring a routed item** (review M3). `RouteConsole` rewrites a *tracked* inbox file in the plane. A console landing that later retires or edits that item on origin blocks the plane's wave-boundary fast-forward until the plane copy is discarded. Retire routed items at a wave boundary: verify the plane's copy, land it through the curation PR, then discard the plane copy before the sync.

**Filed, not in this change:**
- `decision-document-single-declaration` (M2: one wire-shape declaration, three projections).
- `console-route-stamps-block-wave-sync` (M3: wave-sync names the blocking routed files; route stamps out of tracked files; m7 landing evidence in `routed_reason`).
- `no-work-hand-off-rate-alarm` (M4: systemic over-dropping, now exit-0 SKIPPED lanes, gets a policy-configured INCIDENT threshold).
- The sequential path's claimable check still partitions with a nil predicate, so a *surface*-routed item (a declared protected file) counts as claimable there (the F35 review, MINOR 2). It stays open on `triage-termination-scope-aware`.
