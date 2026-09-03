# ADR-0096 — Repair rounds escalate tier and effort, and carry the auditor's findings

- **Status:** Accepted (2026-09-03). Extends [ADR-0092](0092-audit-repair-loop.md) (the
  in-cycle repair loop) and [ADR-0093](0093-retry-envelope-and-terminal-retro.md) (the retry
  envelope); reuses the ADR-0076 slice D escalation seam and envelope clamp.
- **Driving evidence:** the 2026-09-02 ship-rate investigation
  ([research](../../research/ship-rate-harness-reliability-2026-09-02.md), proposals R1 and R2).
  Over cycles 1560–1605 ship probability by audit-round count ran **100 % → 50 % → 17 % → 0 %**;
  the repair loop was grinding, not converging. Two source-verified causes: (G1) the profile
  key `model_tier_overrides.audit_retry_2plus: "deep"` declared in `builder.json` and
  `tdd-engineer.json` had **no producer** — every repair round re-dispatched at the identical
  tier and effort (cycles 1595–1605: balanced/medium, three times each); (G2) the repair brief
  was `audit-fail-reason.json` alone — deterministic gate strings — so the auditor's HIGH
  findings with root cause and `path:line` never reached the builder. Cycle 1605's H1 (a new
  exported package with zero production callers) survived three rounds while the round-2 builder
  edited one sentence of the explanation document; cycle 1596's round-4 builder received one
  truncated defect.
- **Literature:** self-repair is bottlenecked by feedback quality and a stronger feedback model
  yields "substantially larger gains" (Olausson et al., ICLR 2024); execution-grounded feedback
  beats explanation-only by an order of magnitude (Self-Debug, ICLR 2024); repair gains
  concentrate in the first two rounds and then require changing the inputs — feedback, context
  or tier (arXiv 2604.10508); cascades that escalate on weak-model failure match the strong model
  at a fraction of the cost (FrugalGPT, MoT cascades). Sources in the
  [survey](../../research/ship-rate-harness-reliability-2026-09-02-sources.md).

## Problem

The repair loop changed nothing between rounds except the injected gate strings. Model, tier,
effort, persona and prompt structure were byte-identical, and the one input the literature says
matters most — the specific finding — was dropped in transit. A weaker model asked to fix
"EGPS: ship_eligible=false" fixes the symptom it can see.

## Decision

### R1 — the repair round is a situation, and it escalates

1. `core.Orchestrator.repairRoundTier` (`retry_tier_escalation.go`) raises a **tdd or build**
   re-dispatch to the phase profile's declared `model_tier_overrides["audit_retry_2plus"]` while
   `CycleState.AuditRepairActive` is set — the same persisted flag the repair brief derives from,
   so the live loop and the crash-resume path cannot diverge. Raise-only; clamped through the
   **same** envelope guardrail as the ADR-0076 D floor (`router.ClampPlanModelRouting`, never a
   second clamp); no declared key ⇒ inert. Config decides the target, not Go.
2. Applied on **both dispatch surfaces** (`cyclerun_dispatch.go` beside the ADR-0076 D block,
   and `resume.go`), as `PhaseRequest.ModelRoutingTier` — the soft overlay the phase runner
   already honours as chain primary. That is the production tier path; the older
   `subagent.ResolveModelTier` override table is not on it (resolvellm always returns the
   profile tier, so the adaptive resolver is skipped), which is why the declared key could
   never fire.
3. **Effort follows the tier.** `Profile.effort_overrides` (new, keyed by resolved tier) is
   applied at the bridge launch (`launch.go`: `effortForTier(effectiveModel)`), so the deeper
   tier carries the deeper rung without a second situation-plumbing path: the tier IS the
   situation. `builder.json`: `{"deep": "high"}` (codex deep/top rung per the 2026-09-01 quota
   directive); `tdd-engineer.json`: `{"deep": "xhigh"}` (claude grader rung).

### R2 — the brief carries the findings, and names the ones that persisted

4. `core.composeRepairBrief` (`repair_brief.go`) renders, inside the existing 8 KiB budget:
   the gate reasons (unchanged — deterministic evidence outranks prose); then the rejecting
   round's auditor findings **CRITICAL/HIGH first, MEDIUM after, LOW omitted**, parsed by
   `reportdoc.Findings` — the same grammar the dashboard renders and the audit gate's prose
   fallback uses, so operator and agent read one list; then, for each finding whose lead clause
   (`reportdoc.FindingKey`) also appears in `audit-report.round<N-1>.md`, the marker
   **PERSISTED from the previous round — your last repair did not address this**. A report with
   findings but no gate reasons still briefs (previously: no key, blind rebuild).
5. **Prompt archival.** At a repair re-dispatch the previous attempt's `<phase>-prompt.txt` is
   renamed to `phasecontract.RoundArchiveFilename(name, AuditRepairAttempts)` beside the audit
   archives — never clobbering an existing archive — so what each round was actually told is
   forensically recoverable (gap G10).

## Alternatives considered

- **Plumb the situation into `subagent.ResolveModelTier` / `MODEL_TIER_HINT`.** Built and
  discarded: that resolver is bypassed on the production path, so the change would have shipped
  inert (the I2 wiring invariant). The core seam that already raises the build tier is the one
  the runner consumes.
- **Hardcode the escalation tier.** Rejected: the profiles already declare it; Go literals for
  policy are forbidden (operating policy §3.4).
- **Send the whole audit report.** Rejected: the report is up to 32 KiB and the brief budget is
  8 KiB by design; the findings list is the actionable projection, and LOW is advisory on the
  ship gate.
- **Feed findings only when gate reasons exist.** Rejected: 1596's failure mode was exactly a
  brief with the defect set lost.

## Consequences

- Repair rounds are no longer byte-identical retries: tier, effort and the brief all change
  between rounds. The dashboard's round histogram (ADR-0095) is the instrument for whether the
  3-round column stops being a graveyard.
- `internal/bridge.Profile` and `internal/profiles.Profile` gain `EffortOverrides`
  (`effort_overrides`); `.evolve/profiles/builder.json` and `tdd-engineer.json` declare it. Both
  profiles are protected surfaces — this landed through the console manual ship.
- Wiring proofs: `TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop` and
  `TestRepairRoundDispatch_BriefCarriesAuditorFindingsAndArchivesPrompt` drive `RunCycle` with a
  live-shape auditor and assert the round-2 build request's `ModelRoutingTier` and
  `Context[audit_repair_findings]`; the resume surface carries the same calls.
- Remaining gaps from the research (R3–R8: build-exit deterministic floor, inlined contract,
  harness-minted completion, capability-aware scaffolding, learning re-entry, verifier
  isolation) are filed as inbox items on the runtime plane.
