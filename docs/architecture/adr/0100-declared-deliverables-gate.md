# ADR-0100 — The declared-deliverables gate: every agent-owed output and every declared effect is verified at the phase boundary

- **Status:** Accepted (2026-09-12). PR-0 (the claim resolves the plane's inbox; the grant can
  move a file — #573) and PR-1 (this ADR: agent-owed secondaries in the contract gate, the two
  review bypasses closed, the wiring proof) land first; PR-2 (declared effects — `inbox-claim`)
  and PR-3 (`SHIPPED_VIA_BUILD` requires this cycle's own ship) follow as their own slices.
- **Driving evidence:** the operator's 2026-09-12 ask — *"let the orchestrator review whether
  each phase agent's output exists (no content judgment, just existence and format), and if not,
  notify the phase agent to build the missing deliverables"* — and the two-wave health batch
  launched the same day (pid 30870), whose first wave produced the failure and its second half:
  - **Cycle 1630**: triage's files existed and were well-formed; the cycle was still empty because
    the *declared effect* — the atomic inbox claim — failed (`rename … operation-not-permitted`),
    triage honestly wrote `top_n: []`, and the cycle ended `triage-empty-commitment` — then was
    labelled `FinalVerdict: SHIPPED_VIA_BUILD` having run only scout and triage.
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
