# ADR-0100 — The declared-deliverables gate: every agent-owed output and every declared effect is verified at the phase boundary

- **Status:** Accepted (2026-09-12). PR-0 (the claim resolves the plane's inbox; the grant can
  move a file — #573) and PR-1 (this ADR: agent-owed secondaries in the contract gate, the two
  review bypasses closed, the wiring proof) land first; PR-2 (declared effects — `inbox-claim`, landed 2026-09-13 as decision item 8)
  and PR-3 (`SHIPPED_VIA_BUILD` requires this cycle's own ship) follow as their own slices.
- **Driving evidence:** the operator's 2026-09-12 ask — *"let the orchestrator review whether
  each phase agent's output exists (no content judgment, just existence and format), and if not,
  notify the phase agent to build the missing deliverables"* — and the two-wave health batch
  launched the same day (pid 30870), whose first wave produced the failure and its second half:
  - **Cycle 1630**: triage's files existed and were well-formed; the cycle was still empty because
    the *declared effect* — the atomic inbox claim — failed (`rename … operation-not-permitted`),
    triage honestly wrote `top_n: []`, and the cycle ended `triage-empty-commitment` — then was
    labelled `FinalVerdict: SHIPPED_VIA_BUILD` having run only scout and triage. **PR-3 fix:**
    the label now requires this cycle's own ship latch — `CycleState.Shipped`, persisted with the
    checkpoint and set by `latchShippedState` on both dispatch roots → `finalizeOutcome`
    (`go/internal/core/cycle_outcome.go`); main HEAD movement is never evidence. Regression:
    `TestRunCycle_EmptyTriageCommitmentSurvivesSiblingLanding` drives cycle 1630's exact shape
    through `RunCycle` and asserts the no-work label, `IsTriageNoWorkResult`, and zero throughput credit.
  - **Cycle 1631**: triage printed `[inbox-mover] claimed: … → processing/cycle-1631/`; the plane's
    item never moved, and `.evolve/ledger.jsonl` records no `claim` in the triage window. The
    agent had claimed the **worktree's git-tracked copy** of the inbox: `EVOLVE_PROJECT_ROOT` is
    the documented subprocess contract (`core/phase.go`) and nothing exported it.
  - **Cycle 1623** (2026-09-11, the P0 fixed in #572): claim denied, spine ran anyway, twelve
    phases shipped 922 lines against no committed task.
- **Related:** [ADR-0034](0034-unified-deliverable-contract.md) (the contract gate this ADR extends),
  [ADR-0039](0039-failure-floor-and-failure-signal-contract.md) (the correction ladder), ADR-0044 C1
  (every terminal exit records an `abort_reason` → `FAILED_EXPLAINED`),
  [ADR-0058](0058-config-drive-transition-kernel.md) (the trust boundary on registry fields),
  [ADR-0099](0099-deliverable-kinds.md) (the registry as the single declaration of a phase).

## Decision

1. **The registry declares which outputs the agent owes — and only outputs that exist.**
   `outputs.files` is partitioned after the primary: `outputs.agent_owed` (the agent must write
   them — `triage-decision.json`, `carryover-todos.json`) and `outputs.harness_produced` (a harness
   component writes them — `acs-verdict.json`, by `acsrunner`). `files[0]` is always owed.
   `triage-decision.json` — what `committedset` and the router read — was never declared; it is
   now (31 of 36 recent triage runs wrote it; two PASS cycles, 1592 and 1593, did not — the gap
   this gate exists for). The registry also declared `handoff-build.json` and `handoff-scout.json`:
   **0 of 15 recent builds and 0 of 29 recent scouts produced them**, no persona instructs them,
   and `changedpkgs.go` records the build one as extinct since ~cycle 215, replaced by a
   deterministic git derivation. Those declarations are removed — a declared output nobody
   produces is exactly the unverified belief this ADR removes, and gating it would have
   re-dispatched every build and scout (the architecture review caught this before it reached the
   live batch). The partition rule (`phasespec.ValidateOutputsPartition`: every secondary
   classified exactly once, by basename) runs in the registry loader AND on user/overlay specs,
   because an overlay replaces a built-in's spec wholesale (`Catalog.Merge`) — the memo overlay in
   `.evolve/phases/memo/phase.json` declares its own `agent_owed`, and an unclassified secondary
   fails at load, never as a silently ungated phase. `phases[].effects` names lifecycle effects a
   persona performs outside its workspace (`inbox-claim` on triage, PR-2); each binds to one
   deterministic check.
2. **`phasecontract.Contract` projects the declaration; built-ins receive it as an overlay.**
   `Contract.AgentOwedFiles` / `Contract.Effects` come from the registry only (a test asserts no
   built-in sets them). `FromSpec` fills them; `CatalogResolver.Resolve` overlays them onto a
   built-in contract by `phasecontract.RegistryKey` — the ONE home of the name rule (human aliases,
   then `retro → retrospective`), which `core.canonicalCatalogName` now delegates to. Projection
   tests pin the built-ins' `ArtifactName` to the registry's `files[0]` and, in reverse, that every
   built-in with an artifact reaches a registry entry through `RegistryKey` (the control-plane
   contracts that have none are listed explicitly).
3. **The existing contract gate verifies the secondaries.** `deliverable.VerifyWithStage` judges
   the primary as before, then every agent-owed secondary: exists (same write-in-flight grace),
   non-empty, and parses when it is `.json` / `.ndjson`. Markdown *shape* stays the primary
   contract's business. New stable codes — `missing_secondary`, `empty_secondary`,
   `malformed_secondary` — with messages that name the file; the message is the correction
   directive. A read fault that is not absence (EISDIR, permissions) is infra ambiguity and fails
   OPEN, exactly as it does for the primary — re-dispatching an agent cannot fix it.
4. **Correction and termination are the ladder's, unchanged.** A `!Approve` from the gate
   re-dispatches the phase with `PhaseRequest.CorrectionDirective` under
   `correctionLimitFor(phase, ContractCorrectionRetries)` on both loops; exhaustion records the
   `abort_reason` and the cycle ends `FAILED_EXPLAINED` naming the file. The stable codes are what
   `contractBlocksShareIdentity` keys on, so a repeat of the same gap takes the existing
   same-defect path (CLI escalation, then abort) instead of blind re-dispatch.
5. **Two review bypasses are closed.** The remediation re-run (`maybeRemediate`) overwrote the
   dispatch response with the gate re-run's and reached `recordAndBranch` unreviewed; it now goes
   through `reviewAndGuard`. The resume loop skipped review entirely for
   `ExplanationDocumentationVersion == 0` checkpoints; the explanation reviewer already delegates
   on version 0 itself, so the extra skip protected nothing and exempted every other reviewer —
   the twin's skip set is now the fresh loop's.
6. **Wiring is proven, not assumed.** `Orchestrator.DeclaredDeliverablesGateWired()` walks the
   reviewer chain for the capability `VerifiesDeclaredDeliverables()`, which the production
   contract gate implements; the composition-root test asserts a default-policy project has it.
7. **Rollout: enforce immediately, no new dial** (operator decision 2026-09-12). The checks ride
   `gates.contract_gate` (compiled default enforce) and its shadow/advisory semantics; the resumed
   two-wave batch is the soak.
8. **Slice 2 (2026-09-13): the declared effect `inbox-claim` is verified by the same gate.**
   `deliverable/effects.go` binds each `phases[].effects` name to one deterministic check by
   registry lookup (`effectChecks`, one entry today); `verifyEffects` runs after the secondaries so
   one directive names every output *and* effect the phase still owes. A declared name with no
   binding is `unbound_effect` — a registry defect the projection test
   (`TestPhaseRegistry_EveryDeclaredEffectHasACheck`, both directions) catches at commit time,
   reported rather than passed because re-dispatching an agent cannot bind a check.
   `checkInboxClaim` owes a claim for every committed id (`committedset.Committed`, the reader
   closeout reconciles with) **that is an inbox item**: the persona claims the files it ingests
   (Step 0a.4) and nothing else, so a scout- or carryover-originated `top_n` id — which has no
   inbox file — owes nothing; the plan's "every committed id" would have rejected every
   scout-route cycle. Satisfied when `inboxmover.Locate` finds the item under *this* cycle's
   `processing/cycle-N/`; `missing_effect` when it is still pending at the root (1631's phantom
   claim) or held by another cycle (a lane committing to a sibling's item). An empty or
   unrecorded commitment owes nothing. The check needs the cycle, so `phasecontract.Roots` gains
   `Cycle`, carried by all three verifiers — the gate (`ReviewInput.Cycle`), the runner
   (`verifyRootsFor`, the ONE roots translation for its classification and teardown-reconcile
   sites) and the self-check (from the persisted cycle state, only when the contract declares
   effects) — and a verifier that omits it fails OPEN with an error, never decides blind.
   The claim layout has ONE home, the leaf `inboxbatch/layout.go` (`ProcessingDir`,    `ProcessingCycleDir`, `ProcessingCycleDirs`, `ParseProcessingCycle`): the writer `inboxmover.Claim` and the one
   reader walk `inboxmover.Locate` (processing claim first, then the pending root — Promote's
   liveness order; Promote's source resolution and the continuation scope readers now delegate
   to it instead of carrying their own walks) both derive from it, and core's dispatch-time
   claim scan reads through `inboxbatch.LoadDir(inboxbatch.ProcessingCycleDir(...))` — core
   cannot import `inboxmover` (it reaches core through the ledger adapter), which is why the
   plan's "core delegates to `inboxmover.ClaimedIDs`" was withdrawn and the leaf owns the
   belief. The architecture review of the first cut found `Locate` as a fourth hand-typed copy of
   the walk with nothing tying it to where `Claim` writes; the proof `TestLocate_FindsWhatClaimWrote`
   (the production writer, then the reader) goes red when the writer's destination drifts — the
   e2e alone did not, because a vanished item reads as "not an inbox item".

9. **The gate's decisions are signals and its criteria are prompt input (ADR-0101 S2b, 2026-09-13).**
   Every decision `Reviewer.Review` reaches is one Signal Center event under module `gate.contract`
   — `gate.passed` (`GATE_CONTRACT_VERIFIED` naming the artifact, its size, the owed files and the
   effects it found; `_SALVAGED`; WARN `_WOULD_BLOCK` / `_DEMOTED` / `_FAIL_OPEN`) or `gate.rejected`
   (`GATE_CONTRACT_REJECTED`, the reason being the correction directive) — and the ladder's every
   re-dispatch is `gate.corrected` (`ORCHESTRATOR_GATE_CORRECTION`) on both roots, so the
   orchestrator's listener and the triage reader see "checked → advanced" or "checked → rejected:
   <file> → corrected" per phase boundary (design §15.5). The contract block the agent receives now
   names the agent-owed files and the declared effects the gate verifies, and the tail renders
   each owed file at the exact path the gate reads (`phasecontract.OwedPath`, the one join), so the
   prompt and the gate cannot drift on names or locations (they used to live only in persona prose).

10. **Amendment (F36, 2026-09-26): the host performs a declared effect it can, and the gate
    verifies the host's action.** In five cycles (1647, 1671, 1689, 1693 and 1694) the triage agent
    committed its `top_n` and never ran the claim. Each cost one correction re-dispatch on
    `missing_effect` `inbox-claim`. A claim is a deterministic file move (Core Rule 5), so the
    host now performs it.
    - **One registry, two halves.** `deliverable.effects` maps each effect name to
      `{check, perform}`. A nil `perform` leaves the effect to the agent.
    - **One input.** `deliverable.HostEffects.Perform` receives the `core.ReviewInput` the gate
      receives. It resolves the phase through the same `CatalogResolver` and derives roots, cycle
      and inbox directory through the same `rootsFor`. So the host and the gate cannot disagree
      about which effects apply, what is owed, or where. The first cut resolved these separately,
      and review found a legacy resume that would claim into `cycle-0` while the gate checked
      `cycle-N`.
    - **Before every judge.** The same `HostEffects` instance runs at two points, and the claim
      is idempotent, so running it twice is harmless:
      - The runner's verdict engine judges first. `runner.Run` performs the effects from its
        dispatch projection just before `Judge`. The engine's first probe and every settle probe
        therefore see the claim, and the phase is not downgraded to FAIL as
        `DELIVERABLE_UNVERIFIED`. Review found this judge after the first redesign, which had
        performed only at the gate.
      - Then the gate. `core.Orchestrator.performEffectsAndReview` performs the effects, then
        calls the reviewer, and it replaces every reviewer call. On the fresh root those are the
        initial, salvage and correction reviews (`reviewDeliverable`). On the resume root they
        are the initial and correction reviews, and there the gate judges a checkpointed
        response without re-running the runner.
      - Every phase `Config` forwards a late-bound `HostEffects` accessor into `runner.Options`,
        the same way it forwards `ContractVerifier`. That includes the builtin spec-fallback
        runners (`registerBuiltinSpecRunners`: plan-review, doc-sync, spec-verify,
        architecture-design, tester and the rest). Before this change they had neither accessor,
        so they also judged with a different verifier than the gate: the cycle-1685 class.
      - Each point reports its own failure: the runner as `RUNNER_HOST_EFFECT_FAILED`, the gate
        as `ORCHESTRATOR_HOST_EFFECT_FAILED`. The runner performs under
        `context.WithoutCancel`, because a teardown is often what cancelled the context.
      - A claim needs a cycle and an absolute project root. With either missing it fails
        loudly and moves nothing.
    - **The claim.** `inboxmover.ClaimPending` claims the committed ids
      (`committedset.Committed`) still pending at the root. It uses the same ADR-0074 floor as
      `evolve inbox-mover claim` (`guards.IsProtectedScope`), on the root's chained ledger and
      Center. Absent, already-held and console-routed ids are left to the gate; every other
      failure is returned. A returned failure becomes `ORCHESTRATOR_HOST_EFFECT_FAILED` (WARN),
      and the review still runs.
    - **What the gate checks is unchanged.** Only its correction text changed: it now says the
      host's claim did not land and the agent should defer or drop the item.
    - Persona Step 0a.4 is advisory. `Orchestrator.HostEffectsWired()` is the composition-root
      proof, and `--simulate` never binds a performer.
    - **Known seams, not changed here.**
      - The host claims `committedset.Committed` (lane pin first), but closeout promotes
        `inboxmover.CommittedIDs` (top_n and skip_shipped). A pinned id outside `top_n` is claimed,
        then drained back on PASS.
      - The claim happens before judgment. A commitment the gate rejects, or one a correction
        later shrinks, stays in `processing/cycle-N/` until the closeout drain, and sibling lanes
        cannot draw it meanwhile. This matches the era when the agent claimed.
      - Three readers classify where an item sits: `ClaimPending`, `ClaimLaneScope` and
        `checkInboxClaim`. Their rules differ on purpose. The host's claim leaves any held item to
        the gate. The FAIL closeout claims whatever it can so that failure counts are bumped. The
        gate judges.

## Why the existing gate, not a new stage

The first design was a deterministic reviewer *decorated* in front of the semantic one. The
adversarial review showed that would be a second gate beside one that already exists —
`internal/deliverable` resolves the same spec, runs at the same seam on both loops, emits the codes
the ladder keys on, and is already mounted at the composition root. Extending it keeps one gate,
one belief, one wiring, and inherits the breaker, the salvage layer, the shadow stage, and the
CLI-escalation ladder for free. The decorator would also have missed every phase dispatched
through `dispatchEvaluateBatch`, and would have been installed only when *some* other reviewer
was — the same holes, in a second place.

## Why "agent-owed" is declared, not inferred

`acs-verdict.json` is written by the harness. Gating it would re-dispatch the auditor for a
file it cannot produce — up to five deep-tier runs for a defect in `acsrunner`. Inferring
ownership from the file name would be a Go literal belief about config. The registry says which
files the agent owes, and the partition test makes silence a build failure.

## What the gate deliberately does not check

- Markdown sections or verdicts on secondaries — the primary contract owns shape; restating it is
  the duplicated belief the decomposition campaign forbids.
- The four canned harness artifacts (`*-prompt.txt`, `*-events.ndjson`, `*-usage.json`) — bridge-
  written; a missing one is a harness defect the terminal survey reports, not something an agent
  re-dispatch can fix.
- Semantic correctness — the auditor's job (anti-Goodhart, ADR-0034).

## Cost bound

One correction closes a file the agent is instructed to write — which is why only files a persona
names and production actually produces are owed (the evidence check above). A declaration the
prompt never carried would cost a correction on every cycle; that is the failure the extinct
handoff declarations would have caused. The worst case — a persistently
missing secondary on Build — is bounded by `correctionLimitFor` exactly as any contract violation
is today (base 2, scaled up to 5 for Build), and the identity short-circuit escalates rather than
repeats. The gate itself costs O(declared files) stats and parses per phase.

## Filed, not done

- `dispatchEvaluateBatch` (`ParallelEvaluate=enforce`, dormant) bypasses every reviewer,
  including today's contract gate.
- The gate's catalog is captured at construction; a mid-cycle minted phase fails *open* for the
  primary too. `WithCatalogPublisher` is the re-bind seam.
- Cycle worktrees carry a *tracked* copy of `.evolve/inbox` (`worktree-carries-tracked-inbox-copy`).
- Unifying the terminal survey with this projection.

## Verification

Unit: `internal/deliverable` (secondaries table: absent / empty / malformed JSON / NDJSON /
harness-produced not gated / markdown secondary existence), `internal/phasecontract` (partition,
`ArtifactName == files[0]`, overlay by registry key), `internal/core` (remediation re-run reviewed,
legacy-checkpoint resume reviewed, wiring predicate), `cmd/evolve` (composition-root proof).
End-to-end (`declared_deliverables_e2e_test.go`): a real cycle whose builder omits
`handoff-build.json` is re-dispatched with the file named and ends `FAILED_EXPLAINED` naming it,
on both `RunCycle` and `RunCycleFromPhase`; a builder that writes it on the correction round ships.
Every guard was shown to fail by assertion under a reverted mutation.
Slice 2: `internal/deliverable` effects table (pending at root / claimed by this cycle / held by
another cycle / no inbox file / empty commitment / no decision; unbound effect; cycle missing
from roots), the registry↔table projection in both directions, and
`declared_effects_e2e_test.go` — a real cycle whose triage commits to a pending item without
claiming it is re-dispatched with the effect and item named and ends `FAILED_EXPLAINED`; a
triage that claims on the correction round ships; an empty commitment owes no claim and still
ends triage no-work. `internal/phases/runner` proves both verify sites carry the cycle;
`internal/cli/phasecmd` proves the self-check follows the persisted cycle state.

Amendment 10 (F36):
- `internal/phases/runner`: `TestRun_HostEffectsPrecedeTheFirstVerification`. A verifier that refuses until the effects have run classifies PASS on its first probe, with no settle retry, and the effects receive the dispatch's projection once.
- `internal/core`:
  - `performEffectsAndReview` performs host effects before every review, with the identical input.
  - A failure is a coded WARN, and the review still runs.
  - In both `RunCycle` and `RunCycleFromPhase`, host effects precede every review, including a correction re-review.
  - The resume case starts from a checkpoint without a cycle id and claims under the resume point's cycle.
- `internal/deliverable`:
  - `HostEffects` claims exactly what the check owes: top_n minus deferred, a lane pin, nothing recorded, an empty commitment, and a phase without the effect.
  - The declaration is catalog config, a missing cycle fails loudly, and every registry entry has a check.
  - In `TestDeclaredEffects_HostClaim`, the never-claiming agent is accepted on its first review, and a console-routed commitment stays refused and surfaces as the gate's correction.
- `internal/inboxmover`: `ClaimPending` claims only pending ids, raises no false `INBOX_CLAIM_NOT_FOUND`, and returns a failed move.
- `cmd/evolve`: over the repo's real personas and registry, the production orchestrator and every phase runner report `HostEffectsWired()`, and every runner reports `ContractVerifierWired()`. That includes the builtin spec-fallback runners, which a bare temp root never registered, so the old pins could not see them. `--simulate` binds nothing.
- `internal/phases/runner`: `TestRun_AFailedHostEffectIsACodedWarning`. `internal/deliverable`: `TestHostEffects_WithoutAProjectRootIsAnError`.

TDD: the runner fix and the redesign were written test-first, and each test was seen failing before its code. The first cut's `ClaimPending` and end-to-end tests were written after their code, then proven by mutation. Thirteen reverted mutations each failed an assertion: five in core, three in deliverable, two in inboxmover, and three at the runner and wiring level (the pre-judge call, triage's root wiring, the swarm forwarder).
