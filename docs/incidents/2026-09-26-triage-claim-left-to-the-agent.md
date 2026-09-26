# 2026-09-26 — triage's inbox claim was left to the agent, and a skipped claim cost a correction every time

**Class:** pipeline. This is a deterministic action left to an LLM (Core Rule 5). Every miss was caught, but each one cost a full re-dispatch.
**Surface:** the declared effect `inbox-claim` (ADR-0100 slice 2): `agents/evolve-triage.md` Step 0a.4 performed it, and `internal/deliverable/effects.go` verified it.
**Found by:** the console, filing F36 during the six-consecutive-ships campaign (waves 6–8).

## What happened

ADR-0100 slice 2 made the triage persona's inbox claim a declared effect and had the contract gate verify it. The gate's check owes a claim for every committed id that is an inbox item: `evolve inbox-mover claim <id> <cycle>` must have moved the item into `processing/cycle-N/`.

In five recent cycles the agent committed its `top_n` and never ran the command. The gate rejected it each time with `missing_effect`, exactly as designed. The correction ladder re-dispatched triage, the agent ran the claim, and the re-review passed:

| Cycle | Committed item | Rejected (UTC) | Correction | Re-review passed |
|---|---|---|---|---|
| 1647 | `overlay-family-name-transport-ambiguity` | 08:11:19 | 1 of 2 | 08:13:10 |
| 1671 | `kb-graph-projector` | 21:04:46 | 1 of 2 | 21:05:58 |
| 1689 | `lost-ship-closeout-universal-landing-witness` | 18:35:57 | 1 of 2 | 18:37:08 |
| 1693 | `iteration-state-coherence-sentinel` | 23:04:53 | 1 of 2 | 23:05:40 |
| 1694 | `atomicwrite-linked-state-sweep` | 00:37:19 | 1 of 2 | 00:38:07 |

(`.evolve/runs/cycle-<N>/signals.ndjson`: `gate.rejected` `GATE_CONTRACT_REJECTED` → `gate.corrected` `ORCHESTRATOR_GATE_CORRECTION` correction=1 → `gate.passed` `GATE_CONTRACT_VERIFIED`, all at phase triage.)

None of the five sealed FAIL. Each paid one extra triage dispatch, which means a full prompt's tokens and 47–111 s of wall clock. Each also spent one of the phase's two corrections, and a correction spent here is not available when triage misses a real contract requirement.

## Root cause

The claim is a file move. The inputs are fixed once triage has committed: the ids in `committedset.Committed`, the cycle number and the inbox root. The outcome is equally fixed. It moves, or the ADR-0074 floor refuses it. No judgment is involved, but the pipeline assigned it to the agent's instructions. The gate was right to verify the effect. The defect was in who performed it.

## Fix (F36)

1. **One registry owns both halves of an effect.** `deliverable.effects` maps each declared effect to `{check, perform}`. A nil `perform` leaves the effect to the agent.
2. **The host performs from the gate's own input.** `deliverable.HostEffects.Perform` receives the `core.ReviewInput` the gate receives. It resolves the phase through the same `CatalogResolver` and gets roots, cycle and inbox directory from the same `rootsFor`. So the host and the gate cannot disagree about what is owed or where.
   - The first cut computed these separately. Review found that a legacy resume would claim into `cycle-0` while the gate checked `cycle-N`.
3. **The host performs before every judge.** The same `HostEffects` instance runs at two points, and the claim is idempotent:
   - **The runner's verdict engine judges first.** `runner.Run` performs the effects just before `Judge`, so no settle probe sees a missing claim. Every phase `Config` forwards a late-bound accessor, as it forwards `ContractVerifier`. The first redesign performed only at the gate, and review found that the engine would then downgrade triage to FAIL as `DELIVERABLE_UNVERIFIED`.
   - **Then the gate.** `core.Orchestrator.performEffectsAndReview` performs, then reviews, and replaces every reviewer call. On the fresh root those are the initial, salvage and correction reviews. On the resume root they are the initial and correction reviews.
4. **`inboxmover.ClaimPending` performs the claim.**
   - It claims only ids still pending at the inbox root, through the same floor as `evolve inbox-mover claim` (`guards.IsProtectedScope`), on the root's chained ledger and Signal Center.
   - Absent, already-held and console-routed ids are left to the gate and not logged. Claiming them anyway would raise a false `INBOX_CLAIM_NOT_FOUND`, the cycle-1675 signature.
   - Every other failure, such as a failed move, is returned and becomes `ORCHESTRATOR_HOST_EFFECT_FAILED` (WARN). The review still runs.
5. **What the gate checks is unchanged.** A claim the host could not make still reaches the agent as the `missing_effect` directive. The directive now says the host's claim did not land, and tells the agent to defer or drop the item.
6. **Persona Step 0a.4 is advisory.** Running the command stays idempotent, and exit 3 still tells the agent an item cannot be drawn.
7. **The production root binds the performer; `--simulate` never does.** `Orchestrator.HostEffectsWired()` is the wiring proof.

## Pins

- `core/host_effects_test.go`:
  - `TestPerformEffectsAndReview_PerformsHostEffectsBeforeTheReview` (order and the identical input)
  - `TestPerformEffectsAndReview_AFailedHostEffectIsACodedWarningAndStillReviewed`
  - `TestPerformEffectsAndReview_WithoutHostEffectsOnlyReviews`
  - `TestRunCycle_EveryReviewPerformsHostEffectsFirst`
  - `TestRunCycleFromPhase_EveryReviewPerformsHostEffectsFirst` (a correction re-review on both roots, and a checkpoint without a cycle id claims under the resume point's cycle)
- `deliverable/host_effects_test.go`:
  - `TestHostEffects_ClaimsExactlyWhatTheCheckOwes` (top_n minus deferred, a lane pin, nothing recorded, an empty commitment, a phase without the effect)
  - `TestHostEffects_TheDeclarationIsConfig`
  - `TestHostEffects_WithoutACycleIsAnError`
  - `TestEffects_EveryEntryIsVerified`
- `deliverable/declared_effects_e2e_test.go::TestDeclaredEffects_HostClaim`:
  - the never-claiming agent of these cycles is accepted on its first review, with zero corrections;
  - a console-routed commitment stays at the root and surfaces as the gate's correction.
- `inboxmover/claim_pending_test.go`: `TestClaimPending_ClaimsOnlyWhatIsPendingAtTheRoot`, `TestClaimPending_AReadFaultIsReported`, `TestClaimPending_AFailedMoveIsReported`
- `phases/runner/host_effects_test.go::TestRun_HostEffectsPrecedeTheFirstVerification` (a verifier that refuses until the effects ran classifies PASS on its first probe, with no settle retry)
- `cmd/evolve/cmd_cycle_host_claim_test.go::TestWireOrchestrator_HostEffectsWired` (the orchestrator and every phase runner are bound; `--simulate` is not)

TDD: the runner fix and the redesign were written test-first, and each test was seen failing before its code. The first cut's `ClaimPending` and end-to-end tests were written after their code, then proven by mutation. Thirteen reverted mutations each failed an assertion:
- runner and wiring: the pre-judge call, triage's root wiring, and the swarm forwarder;
- core: the effect skip, the swallowed failure, and each of the three reviewer call sites;
- deliverable: the registry's performer, the cycle guard, and the resolver;
- inboxmover: the returned failure and the held-id skip.

## Lesson

Declaring an effect and verifying it at the boundary (ADR-0100) turned silent misses into loud, recoverable ones. It did not decide who should perform the effect. When a declared effect is a deterministic action on inputs the host already holds, the host performs it and the gate checks the host. The agent is left only with the judgment the effect depends on, here which items to commit.
