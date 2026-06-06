# Dynamic Phase Routing (Go kernel)

> **Status:** Kernel shipped v13.0.0 (PR #4, `53ed48b`), **default `advisory`** since 2026-06-06 (registry-pinned after retro migration steps 1-3 landed; was default-off from v13.0.0; `EVOLVE_DYNAMIC_ROUTING=off` remains the static escape hatch).
> **Audience:** Operators experimenting with per-cycle phase selection; persona + router authors.
> **Source:** `go/internal/config/config.go` (composition root), `go/internal/router/` (clamp + strategy), `go/internal/core/phase_advisor.go` (LLM brain), `go/internal/core/orchestrator.go` (`WithRouting`), `docs/architecture/phase-registry.json` (registry).
> **Successor design:** ADR-0024 (Proposed) replaces the fixed mandatory-spine model with a conditional floor + PhaseAdvisor — read it before extending this kernel.

## TL;DR

The static state machine runs a fixed phase sequence every cycle. The routing kernel lets a strategy (deterministic *or* LLM-proposed) decide which **optional** phases run per cycle, while a pure clamp pass guarantees the **integrity floor** (mandatory spine + TDD-pin + ship-needs-real-audit) can never be weakened. Since 2026-06-06 it is **advisory by default** (registry-pinned); with `EVOLVE_DYNAMIC_ROUTING=off` (the escape hatch), behavior is byte-identical to pre-routing.

This is **"model proposes, kernel disposes":** the LLM brain only ever produces an advisory plan; `router.Route()` re-validates it against the floor, and any malformed/hallucinated/timed-out proposal degrades cleanly to the deterministic `StaticPreset`.

> **Not** the same as PSMAS (`EVOLVE_PSMAS_SKIP`, see [psmas-phase-scheduling.md](psmas-phase-scheduling.md)) — that is a legacy bash Triage-driven *skip* path enforced at `phase-gate.sh`. This kernel is the Go composition-root router that supersedes it.

## Contents

- [Rollout stages](#rollout-stages)
- [Routing modes (the brain)](#routing-modes-the-brain)
- [The integrity floor](#the-integrity-floor)
- [Configuration surface](#configuration-surface)
- [The phase registry](#the-phase-registry)
- [The LLM proposer](#the-llm-proposer)
- [Integration status](#integration-status)
- [Anti-patterns and guards](#anti-patterns-and-guards)
- [See also](#see-also)

## Rollout stages

`EVOLVE_DYNAMIC_ROUTING` selects a rollout stage (`config.Stage`). Each tier gives the router more authority, validated incrementally via the soak diff.

| Stage | Value | Who drives | Use |
|---|---|---|---|
| Off | `off` / `0` (escape hatch; was default until 2026-06-06) | Static state machine | Legacy; byte-identical to pre-routing |
| Shadow | `shadow` | Static; router computes + logs only | Soak: diff the would-have-routed plan vs static, zero behavior change |
| Advisory | `advisory` | **Advisor drives** every non-mandatory phase via the clamped whole-cycle plan; `enforceNext` overrides the static successor | First live tier (cycle-108) |
| Enforce | `enforce` | Same as Advisory (advisor drives, kernel-clamped) | Full routing |

An unknown value resolves to `Off` and emits an `unknown-value` warning (never a hard error — typos must not break autonomy).

**ADR-0024 §1 floor activation (PR-5, live as of this slice).** At `Stage >= Advisory` the orchestrator computes the advisor's whole-cycle plan once at cycle start, clamps it with `router.ClampPlanToFloor`, and threads the clamped plan into every `Route()` so it drives run/skip for non-mandatory phases. Two separate guarantees, deliberately decoupled:

- **Configurable never-skip set** — `EVOLVE_MANDATORY_PHASES` (default `scout,build,audit,ship`). `shouldRun`'s mandatory branch runs first, so the advisor can never skip a mandatory phase.
- **Non-configurable integrity floor** — `ship ⇒ build ∧ audit ∧ (tdd unless trivial)`, forced *into the plan* before it is threaded. This is what makes a tiny mandatory set safe: an operator may set `EVOLVE_MANDATORY_PHASES=tdd` and the advisor still cannot reach ship without a real build+audit, because `ClampPlanToFloor` re-adds them to the plan.

A planner failure (or `routing_mode=static`, which has no advisor) ⇒ nil plan ⇒ the configurable spine drives via the trigger path (**fail-safe to static**). The `enforceNext` override stays re-validated by `CanTransition` + the artifact-backed `SpineSatisfiedUpTo`, and the ship phase's audit-binding remains the ultimate gate — three layers, the floor never the sole one.

> **Known limit (cost, not safety):** a no-ship cycle ending early (`scout → end`) is honored by the pure kernel but NOT yet end-to-end — `CanTransition` has no `scout → end` edge, so `enforceNext` declines it and the static spine runs to completion. Widening the state-machine graph for early-exit is a follow-up; it cannot weaken ship-safety.

## Routing modes (the brain)

`EVOLVE_ROUTING_MODE` selects the strategy (`config.Mode`) — a Strategy-pattern swap, not a runtime conditional.

| Mode | Values | Behavior |
|---|---|---|
| DynamicLLM (default) | `llm` / `dynamic` / `dynamic-llm` | LLM proposes optional insertions/skips; kernel clamps |
| StaticPreset | `static` / `static-preset` / `preset` | Deterministic: declarative triggers + spine only, no LLM call |

Unknown value → `DynamicLLM` + warning. The orchestrator depends only on the `router.RoutingStrategy` interface; the mode is resolved once at the composition root.

## The integrity floor

The clamp pass in `go/internal/router` enforces the floor regardless of stage or proposal. The floor is the entire safety story — it is what prevents the routing layer from reopening the cycle 102–141 reward-hacking surface (`[[project_incident_history]]`).

| Invariant | Meaning |
|---|---|
| Mandatory spine | `scout, build, audit, ship` always present + ordered (`EVOLVE_MANDATORY_PHASES`) |
| Weak-spine guard | Omitting `audit` or `ship` from the spine emits a `weak-spine` warning (`validateSpine`) |
| Conditional TDD-pin | `tdd` is mandatory unless `cycle_size == trivial` (`EVOLVE_CONDITIONAL_MANDATORY`, default `tdd:cycle_size!=trivial`) |
| Ship-needs-real-audit | A plan that reaches `ship` without a PASS audit bound to the built tree is rejected/clamped |
| Insertion cap | The router may insert at most `EVOLVE_MAX_OPTIONAL_INSERTIONS` (default 4) optional phases |

## Configuration surface

`config.Load(registryPath, env)` is the **single** reader of routing env + registry. Downstream consumers receive the immutable `RoutingConfig` by injection (`WithRouting`) and never call `os.Getenv`. Precedence: **env override > registry file > built-in default**.

| Env var | Default | Maps to |
|---|---|---|
| `EVOLVE_DYNAMIC_ROUTING` | `advisory` (since 2026-06-06; was `off`) | `Stage` |
| `EVOLVE_ROUTING_MODE` | `llm` | `Mode` |
| `EVOLVE_MANDATORY_PHASES` | `scout,build,audit,ship` | `Mandatory` (CSV) |
| `EVOLVE_CONDITIONAL_MANDATORY` | `tdd:cycle_size!=trivial` | `Conditional` (`phase:expr`; op ∈ `!= == >= <= > <`) |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | `4` | `MaxInsertions` (int) |
| `EVOLVE_USE_PHASE_REGISTRY` | enabled (`0` disables) | whether to read the registry file |

Per-phase legacy enable flags (`EVOLVE_REQUIRE_INTENT`, `EVOLVE_TRIAGE_DISABLE`, `EVOLVE_PLAN_REVIEW`, `EVOLVE_TEST_PHASE_ENABLED`, `EVOLVE_BUILD_PLANNER`, `EVOLVE_DISABLE_AUTO_RETROSPECTIVE`) are absorbed by `config.Load` into `PhaseEnable`, keeping `os.Getenv` out of the phase code.

## The phase registry

`docs/architecture/phase-registry.json` (schema v3) is the data-only phase catalog. Its `config{}` block mirrors the env defaults; each phase entry carries `optional`, `enabled` (`on`/`off`/`content`), and an optional `routing` block of declarative **Specification** triggers.

Example — the content-routed `tester` phase is proposed only when the build's objective signals show failing predicates or a high-severity thrust:

```json
"routing": {
  "insert_when": [
    { "field": "build.acs_red", "op": "gt", "value": 0 },
    { "field": "build.severity_max", "op": "gte", "value": "HIGH" }
  ]
}
```

Triggers are honored only at `Stage >= Advisory`; in `Shadow` they are forensic-only.

## The LLM proposer

`core.PhaseAdvisor` (`go/internal/core/phase_advisor.go`) is the bridge-backed `DynamicLLM` brain:

- Asks an LLM, via the `core.Bridge` port, which optional phases to insert/skip given the objective digest (`router.Digest`).
- Defaults to a cheap/fast model (`haiku`) on the `claude-tmux` driver — routing is a lightweight read-only judgment, not heavy generation. Override with `WithProposerCLI` / `WithProposerModel`.
- Output is **advisory**: the pure `router.Route()` clamp re-validates it against the floor. A hallucinated or malformed proposal can never weaken the ship guarantee.
- Any failure (bad JSON, timeout, empty) returns an error and degrades to the deterministic `StaticPreset` — fail-safe to the floor.

Each decision is recorded as a `routing-decision-N.json` artifact in the cycle workspace (`Orchestrator.recordRoutingDecision`) so the Off→Shadow soak can diff advisor rationale vs the static path.

**Hybrid cadence (ADR-0024 §2).** Once an upfront whole-cycle plan is driving (`Stage >= Advisory`, `in.Plan != nil`), `LLMProposal.Decide` invokes the per-transition `Propose` ONLY at **branch transitions** — post-build and post-audit, where new objective signals (`acs_red`, audit verdict) appear that the signal-poor start-of-cycle plan could not foresee. Every other transition is already decided by the cached plan, and a proposal can never change the kernel's `NextPhase` regardless (`applyProposal` only annotates + clamps), so calling the LLM there was pure cost. With NO upfront plan (Shadow, static mode, planner failure) the legacy per-transition cadence stands, so Shadow-soak forensics are unchanged. Gate: `router.shouldPropose(in)` → `isBranchTransition(in.Current)`.

## Integration status

| Aspect | State |
|---|---|
| Kernel on `main` | Yes — `config` + `router` + `core.PhaseAdvisor`, fully unit-tested + the `routingtest` "Lego" scenario framework |
| Default | Off — `WithRouting` is opt-in; absent it, the orchestrator runs legacy Stage:Off |
| First live run | cycle-108 (`advisory`+`llm`) — proved the proposer emits valid strict JSON end-to-end; see `knowledge-base/research/cycle-108-routing-live-data.md` |
| Known evolution | ADR-0024 (Proposed): shrink the hard mandatory set to a single conditional invariant + rename `RoutingProposer`→`PhaseAdvisor`, hybrid cadence, `phase-plan.json` |

## Anti-patterns and guards

| Anti-pattern | Why bad | Guard |
|---|---|---|
| Reading a routing env var outside `config.Load` | Splits the composition root; reintroduces `os.Getenv` sprawl | `config` is a leaf package (stdlib-only); orchestrator/phases take `RoutingConfig` by injection |
| Letting the LLM proposal drive ship directly | Reopens the reward-hacking surface | `router.Route()` clamp re-validates every proposal against the floor before any phase runs |
| Dropping `audit` or `ship` from `EVOLVE_MANDATORY_PHASES` | Breaks audit-before-ship | `validateSpine` emits `weak-spine`; ship-needs-real-audit still clamps |
| Hard-failing on an unknown stage/mode value | A typo would break autonomy | Unknown values resolve to safe defaults (`Off` / `llm`) + a warning |

## See also

- [psmas-phase-scheduling.md](psmas-phase-scheduling.md) — legacy bash Triage-driven phase-skip path (the predecessor concept)
- [phase-architecture.md](phase-architecture.md) — canonical static phase sequence the router operates within
- [sequential-write-discipline.md](sequential-write-discipline.md) — `parallel_eligible` single-writer invariant the registry carries
- [adr/0024-conditional-ship-gate-floor-and-phase-advisor.md](adr/0024-conditional-ship-gate-floor-and-phase-advisor.md) — Proposed PhaseAdvisor evolution
- [control-flags.md](control-flags.md) — full `EVOLVE_*` control surface
- `docs/architecture/phase-registry.json` — the data-only phase catalog (schema v3)
