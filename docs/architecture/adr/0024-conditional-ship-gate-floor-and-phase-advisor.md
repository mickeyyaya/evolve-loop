# ADR-0024: Conditional ship-gate floor + PhaseAdvisor

> Status: **Accepted — partially implemented** (2026-05-27). Replaces the fixed `mandatory_spine`
> routing model (ADR-era dynamic-routing, PR #4) with a *minimal conditional floor* + an LLM
> **PhaseAdvisor** that justifies each phase's inclusion. Motivated by the first live `advisory`+`llm`
> cycle (cycle-108) — see `knowledge-base/research/cycle-108-routing-live-data.md`. Builds on the
> dynamic-routing kernel (`go/internal/router`, `core.PhaseAdvisor`) already on `main`.
>
> **Implementation status:** foundational slices (digest widening, PhaseAdvisor rename, `phase-plan.json`
> array shape) landed PR #14; the pure conditional floor `ClampPlanToFloor` landed dormant PR #15; the
> **live-wiring (PR #16)** activates it — the orchestrator computes the upfront plan, clamps it, and
> drives non-mandatory run/skip at `Stage >= Advisory` (`enforceNext` widened Enforce→Advisory). The
> floor (`ship ⇒ build ∧ audit ∧ tdd`) is decoupled from the *configurable* `EVOLVE_MANDATORY_PHASES`
> set, so the operator's "only tdd mandatory" intent is safe. **§2 hybrid cadence (PR-6)** reduces the
> per-transition `Propose` to branch transitions only (post-build, post-audit) once a plan is driving —
> removing the per-transition double-spend (`LLMProposal.Decide`→`shouldPropose`); a proposal never
> changed the kernel's `NextPhase` anyway, so this is forensics-only with zero routing change.
> **Remaining:** §6 `clamp_reason` forensics + Tier A/B registry rollout, and an early-exit
> state-machine edge (`scout → end`) so no-ship cycles can end early end-to-end.

## Context

The dynamic-routing feature (PR #4) shipped with a **fixed mandatory spine** `[scout, build, audit, ship]`
+ a conditional TDD pin; the LLM proposer could only *insert* optional phases (in practice just `tester`)
on top. The first live run (cycle-108, `EVOLVE_DYNAMIC_ROUTING=advisory EVOLVE_ROUTING_MODE=llm`) proved
the proposer works end-to-end (valid strict JSON on Haiku/Max) but surfaced three problems:

1. **Most transitions are signal-free.** The start-transition proposer call had empty objective signals;
   its output was discarded in favor of the deterministic `reason:"spine:scout"`. Pure wasted spend.
2. **The LLM justification is dropped** from the recorded decision — the Off→Shadow soak (the promotion
   gate) needs the would-have-routed rationale to diff against the static path.
3. **The optional-phase universe is trivial** (`tester` only), so the "router" rarely makes a real choice.

The operator's intent: shrink the hard mandatory set to its safety-essential core and let an LLM
**advisor** *justify which phases are needed* per cycle. The risk: the entire integrity model (EGPS gate,
audit-binding, adversarial auditor — hardened across the cycle 102–141 reward-hacking/gaming incidents,
`[[project_incident_history]]`) rests on **not** being able to ship unverified code. A naive "only TDD
mandatory" would let the advisor route `… → ship` skipping audit, reopening that surface.

## Decision

### 1. The floor is a conditional invariant, not a phase list

There is **no unconditionally mandatory phase**. The kernel enforces exactly one implication:

```
reach(ship)  ⇒  audit-PASS bound to the build tree  ⇒  build  ⇒  (tdd, if cycle is non-trivial)
```

- A no-ship cycle (investigation/convergence) may legitimately end after scout — that is an advisor decision, not a violation.
- The advisor may **never** weaken the chain: any proposed plan that reaches `ship` without a PASS audit bound to the built tree (EGPS `red_count==0`, audit-binding SHA match) is **rejected/clamped** by the kernel. `build`-before-`audit` and `tdd`-before-`build` (non-trivial) are part of the same causal clamp.
- This is the existing "model proposes, kernel disposes" guarantee, narrowed to a single load-bearing invariant.

### 2. PhaseAdvisor (rename of RoutingProposer) — hybrid cadence

| Axis | Decision |
|---|---|
| Name | `core.RoutingProposer` → `core.PhaseAdvisor`. Output artifact `routing-proposal.json`/`routing-decision-N.json` → **`phase-plan.json`**: array of `{phase, run: bool, justification: string}`. |
| Cadence | **Hybrid.** One upfront whole-cycle plan at cycle start (cheap, coherent), then **re-consult only at real-branch transitions** (post-build: insert `tester`? / post-audit: `retro`/`memo`?). Signal-free transitions (e.g. start→scout when scout is planned) do **not** call the LLM. Directly fixes problem #1. |
| Justification capture | Each phase's `justification` is recorded in `phase-plan.json` **and** carried into the ledger `routing_decision`/`phase_plan` entry. Fixes problem #2 — the soak can now diff advisor-rationale vs static. |
| Floor validation | Kernel clamps the advisor plan against §1 before any phase runs; a clamp is logged with `clamp_reason` (never silently dropped). |
| Fail-safe | Any advisor error (bad JSON, timeout, empty) degrades to the deterministic static plan — unchanged "fail to the floor" behavior. |

### 3. Widen the digest with per-phase decision inputs

The advisor is only as good as the digest. `router.Digest` gains per-phase decision signals so each
`run/skip` is justifiable (problem #3). Initial set:

| Signal | Drives |
|---|---|
| `carryover_count`, `backlog_size` | skip `scout` (use carryover) vs full discovery |
| `diff_files_touched`, `diff_loc` | insert `plan-review` / `build-planner` on large changes |
| `acs_red`, `severity_max` | insert `tester` after build |
| `audit_verdict`, `audit_confidence` | `retro` / `memo` after audit |
| `cycle_size` (trivial/S/M/L) | the TDD-pin trivial exemption |

### 4. Incremental rollout (not wholesale)

Move phases into advisor-decided **one tier at a time**, validated via the existing
shadow→advisory→enforce machinery and the soak diff:

1. **Tier A first:** `scout`, `triage` become advisor-decided (shadow-soak the would-have-routed diff vs static).
2. **Tier B:** `plan-review`, `build-planner`, `tester`, `retro`, `memo`.
3. **Never:** `build`, `audit`, `ship` — floor-pinned per §1 throughout.

## Prerequisite (hard blocker)

**Bridge artifact-path bug must be fixed first** — no live cycle completes until then. Cycle-108 died at
scout with `exit=81 ExitArtifactTimeout`: the agent wrote `<cycle>/workspace/scout-report.md` (per the
`workspace/`-prefixed convention in `agents/evolve-scout.md:134`) but the runner polls `<cycle>/scout-report.md`
(runner.go:210). Intermittent across cycles (106/108 hit it; 103/104/107 didn't) = agent-compliance variance
against an ambiguous contract. Two-sided fix: (a) unambiguous doc (bare filename for the polled artifact);
(b) runner polls both `<ws>/X` ∥ `<ws>/workspace/X` + emits a louder-than-timeout diagnostic listing what
*did* appear. Details: `knowledge-base/research/cycle-108-routing-live-data.md`.

## Sequence

0. Bridge artifact-path fix (TDD) — unblocks all live cycles.
1. Widen `router.Digest` (§3).
2. PhaseAdvisor: rename + hybrid cadence + `phase-plan.json` + floor-validation (§1, §2), rollout Tier A → B (§4).

## Consequences

- **Positive:** smaller, clearer integrity floor (one invariant); LLM advises with justification (auditable, soak-diffable); no wasted signal-free calls; richer per-phase reasoning.
- **Negative / risk:** advisor plans must be rigorously floor-clamped (the clamp is the entire safety story — needs adversarial tests). Digest widening is the higher-leverage, higher-effort half. Naming churn (`RoutingProposer`→`PhaseAdvisor`, artifact rename) touches tests + the routingtest "Lego" catalogs.
- **Reversible:** `EVOLVE_DYNAMIC_ROUTING=off` reverts to byte-identical legacy; each tier promotes independently via shadow→advisory→enforce.

## References

- `knowledge-base/research/cycle-108-routing-live-data.md` — live data + bug forensics
- `[[project_dynamic_routing_progress]]` — dynamic-routing kernel deep detail (PR #4)
- `[[project_incident_history]]` — why audit-for-ship stays pinned
- PR #4 (routing, `53ed48b`), PR #5 (bridge, `c9302d7`)
