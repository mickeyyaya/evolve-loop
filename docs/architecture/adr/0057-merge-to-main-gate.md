# ADR-0057: Merge-to-main gate + cadence advisor

## Status

Proposed (2026-06-21). Shipping behind a `policy.json` rollout dial defaulting to
**shadow** (behavior-neutral).

## Context

Feature work spans multiple cycles/milestones. In concurrent **campaign/fleet**
mode, cycles run in parallel worktrees and `swarm.RunMergeTrain` fans them into a
shared **integration branch** (each per-worker merge gated by an
`AcceptanceChecker`); `campaign.RunWaves` checkpoints `CampaignProgress.CompletedWaves`
after every wave. What was missing: **nothing promotes a completed wave/milestone's
integration branch back to `main`, gated and advisor-scaled.** That step was a
manual human action (`/release` → `/publish`).

The request: add (1) a **phase agent** that gates the merge of milestone work back
to main, and (2) **advisor logic** that reads code-tree progress and decides the
*cadence/scaling* of invoking that gate.

Web research (Fowler/CI, DORA/Accelerate, Reinertsen, GitHub/GitLab/Bors merge
queues) was decisive on two points: continuous **small-batch** integration beats
milestone batching (doubling unintegrated code ~quadruples conflict cost), and the
mature pattern is a **two-gate model** — an *entry* gate (a branch's own checks
green) distinct from an *integration/promotion* gate (the speculative merged state
green). For AI-agent-authored merges the evidence favors **evidence-bound verdicts**
(never "done" from prose) and a **shadow→advisory→enforce** rollout for any new
control gate.

User decisions (AskUserQuestion, 2026-06-21): milestone-promotion gate (per-merge
integration stays continuous); auto-merge on PASS; default stage shadow; v1 scope =
concurrent campaign/fleet.

## Decision

A **config-defined, read-only LLM gate phase** + a **deterministic kernel
promoter** + a **config-as-code cadence advisor**, reusing the existing hardened
merge-train/ship path. The gate governs *integration-branch → main promotion*; the
continuous per-merge fan-in is untouched (two-gate model).

Trigger refinement: `router.ReconDigest` is prompt-only (not folded into the
routable `RoutingSignals` plane), so the gate is **not** a per-cycle router-inserted
phase. Its trigger is the **campaign wave boundary**, where progress is known and
checkpointed.

Five units (TDD, each behavior-neutral until the final stage flip):

- **Policy (config-as-code, no flags).** `policy.MergeGatePolicy` / `MergeGateConfig()`
  resolver (mirrors `GatesConfig()`); `config.RolloutStages.MergeGate Stage`
  defaulting to `StageShadow`. Thresholds live in `policy.json` `merge_gate`
  (`batch_wave_count`, `batch_churn_loc`, `block_severity`, `carryover_stall_cycles`).
- **Cadence advisor** — `mergegate.DecideCadence(DecisionInput, Thresholds) Decision`.
  Pure. **Defer-wins**: any safety violation (audit≠PASS, CI not green, ledger
  unverified, conflicts, severity≥block) forces `defer`; otherwise a Strategy picks
  `per-wave | batched | feature-complete`. This is the floor the LLM gate can only
  ever tighten ("model proposes, kernel disposes").
- **Gate phase (config-only)** — `.evolve/phases/merge-to-main-gate/phase.json`
  (`archetype: evaluate`, optional, no `insert_when` — campaign-invoked),
  `agents/evolve-merge-to-main-gate.md` (read-only promotion skeptic, 7-step
  evidence-bound checklist), `.evolve/profiles/merge-to-main-gate.json` (read-only,
  `read_only_repo: true`, git verbs denied). Dispatched via the bridge.
- **Promoter** — `mergegate.Promoter` (Humble Object). off/unknown → no-op;
  shadow/advisory → record the would-be promotion; enforce + PASS → promote via the
  injected `Executor` (the production impl drives `swarm.RunMergeTrain` →
  ship/release with armed auto-rollback); a failed promotion auto-rolls-back.
- **Wave-boundary seam** — `campaign.RunOptions.AfterWaveComplete(wave, prog)`, fired
  fail-open after each wave is durably saved. The production hook (assembled at the
  cmd shell) runs the advisor → on Fire dispatches the gate phase → feeds the verdict
  to the promoter.

## Alternatives considered

1. **Per-cycle router-inserted phase via `insert_when` over recon signals**
   (original sketch). Rejected: `ReconDigest` is prompt-only, so this would require
   inventing a recon→routable-signal plane; and a per-cycle phase firing a
   cross-cycle promotion is a scope mismatch. The campaign wave boundary is the
   correct trigger.
2. **Feature-branch integration gate** (cycles accumulate off-main, gate merges the
   whole branch at milestones). Rejected by the trunk-based/DORA evidence (large
   batches raise integration cost) and the user's choice; continuous per-merge
   integration is kept.
3. **Cadence judgment inside the LLM gate only.** Rejected: the safety floor
   (defer-on-red) must be deterministic and tested; only the qualitative cadence
   choice belongs to the model.
4. **Promoter does raw `git merge`.** Rejected: the merge must ride the hardened
   acceptance-gated merge-train + ship path (ship.lock, attestation, EGPS, CI-green,
   auto-rollback). The promoter delegates to an injected `Executor`; the LLM never
   runs a git verb.

## Consequences

- **Integrity floor untouched.** The gate is `optional:true` (un-bypassable),
  never in `DefaultShipFloor()`; `ClampPlanToFloorWith` remains the sole trust
  boundary.
- **Auto-merge is safe by construction.** A PASS verdict only *permits* a kernel
  promotion attempt that the merge-train's own acceptance check can still reject and
  roll back. The dangerous verb is never in the model's hands. Auto-merge activates
  only at `enforce`, after a human-watched shadow soak.
- **Behavior-neutral default.** Shadow records would-be promotions and merges
  nothing; an absent `merge_gate` block resolves to shadow.
- **Remaining integration (follow-up — "Slice 4b"):** the production
  `AfterWaveComplete` hook assembly (advisor → bridge dispatch of the gate phase →
  promoter → ledger `kind:"merge_gate"`), the concrete `swarm`-backed `Executor`,
  the `cfg.MergeGate = parseStage(policy.MergeGateConfig().Stage)` composition-root
  wiring, the `BlockSeverity`→`SeverityBlocks` translation (caller-side
  `router.Severity` compare), and the operator-facing `policy.json` `merge_gate`
  block. These are the only parts that touch real behavior (and only at `enforce`);
  they land together so the operator dial is wired the moment it appears (no
  no-op surface). Until then the foundation is dormant by construction: the dial
  defaults to shadow and nothing consumes `MergeGateConfig` yet.

## References

- Plan: `~/.claude/plans/i-want-you-to-serene-thimble.md`
- Reuses: `swarm.RunMergeTrain` (mergetrain.go), `campaign.RunWaves` (executor.go),
  `policy.GatesConfig` pattern, `config.RolloutStages`, `ApplyArchetypeDefaults`.
- Related: ADR-0052 (advisor-maximization), ADR-0054 (concurrent sibling worktrees).
