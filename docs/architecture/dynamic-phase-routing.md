# Dynamic Phase Routing (Go kernel)

> **Status:** Kernel shipped v13.0.0 (PR #4, `53ed48b`), **default `advisory`** since 2026-06-06. Registry-pinned after retro migration steps 1-3 landed; it was default-off from v13.0.0. `EVOLVE_DYNAMIC_ROUTING=off` remains the static escape hatch.

> **Amended 2026-10-27 (X18c):** the dials of this page are keys of `.evolve/policy.json`: `router.dynamic_routing`, `router.routing_mode`, `router.conditional_mandatory` and `router.max_optional_insertions`. No code reads the `EVOLVE_*` names of this page, except `EVOLVE_USE_PHASE_REGISTRY` ([policy-config.md](policy-config.md#routing-and-run-knobs-x18c)).

> **Audience:** Operators experimenting with per-cycle phase selection; persona + router authors.
> **Source:** `go/internal/config` (the `Loader` the composition root calls — ADR-0103 unit 08, [decomposition/08-config.md](decomposition/08-config.md)), `go/internal/router/` (clamp + strategy), `go/internal/core/phase_advisor.go` (LLM brain), `go/internal/core/orchestrator.go` (`WithRouting`), `docs/architecture/phase-registry.json` (registry).
> **Successor design:** ADR-0024 (Proposed) replaces the fixed mandatory-spine model with a conditional floor + PhaseAdvisor — read it before extending this kernel.

## TL;DR

The static state machine runs a fixed phase sequence every cycle. The routing kernel lets a strategy (deterministic *or* LLM-proposed) decide which **optional** phases run per cycle. A pure clamp pass guarantees that nothing can weaken the **integrity floor** (mandatory spine + TDD-pin + ship-needs-real-audit). Since 2026-06-06 it is **advisory by default** (registry-pinned). With `EVOLVE_DYNAMIC_ROUTING=off` (the escape hatch), behavior is byte-identical to pre-routing.

This is **"model proposes, kernel disposes":** the LLM brain only ever produces an advisory plan. `router.Route()` re-validates it against the floor. Any malformed/hallucinated/timed-out proposal degrades cleanly to the deterministic `StaticPreset`.

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

`EVOLVE_DYNAMIC_ROUTING` selects a rollout stage (`config.Stage`). Each tier gives the router more authority, validated incrementally with the soak diff.

| Stage | Value | Who drives | Use |
|---|---|---|---|
| Off | `off` / `0` (escape hatch; was default until 2026-06-06) | Static state machine | Legacy; byte-identical to pre-routing |
| Shadow | `shadow` | Static; router computes + logs only | Soak: diff the plan that the router computed against the static plan, zero behavior change |
| Advisory | `advisory` | **Advisor drives** every non-mandatory phase through the clamped whole-cycle plan; `enforceNext` overrides the static successor | First live tier (cycle-108) |
| Enforce | `enforce` | Same as Advisory (advisor drives, kernel-clamped) | Full routing |

An unknown value resolves to `Off` and emits an `unknown-value` warning (never a hard error — typos must not break autonomy).

**ADR-0024 §1 floor activation (PR-5, live as of this slice).** At `Stage >= Advisory` the orchestrator computes the advisor's whole-cycle plan once at cycle start. It clamps the plan with `router.ClampPlanToFloor`. It threads the clamped plan into every `Route()`, so the plan drives run/skip for non-mandatory phases. Two separate guarantees, deliberately decoupled:

- **Configurable never-skip set** — `EVOLVE_MANDATORY_PHASES` (default `scout,build,audit,ship`). `shouldRun`'s mandatory branch runs first, so the advisor can never skip a mandatory phase.
- **Non-configurable integrity floor** — `ship ⇒ build ∧ audit ∧ (tdd unless trivial or deliverable_kind=document — ADR-0099)`, forced *into the plan* before the orchestrator threads it. This makes a tiny mandatory set safe. An operator is allowed to set `EVOLVE_MANDATORY_PHASES=tdd`, and the advisor still cannot reach ship without a real build+audit, because `ClampPlanToFloor` re-adds them to the plan.

A planner failure (or `routing_mode=static`, which has no advisor) ⇒ nil plan ⇒ the configurable spine drives through the trigger path (**fail-safe to static**). `CanTransition` + the artifact-backed `SpineSatisfiedUpTo` keep re-validating the `enforceNext` override. The audit-binding of the ship phase remains the ultimate gate. These are three layers, and the floor is never the sole one.

> **Known limit (cost, not safety):** the pure kernel honors a no-ship cycle that ends early (`scout → end`), but NOT yet end-to-end. `CanTransition` has no `scout → end` edge, so `enforceNext` declines it and the static spine runs to completion. A wider state-machine graph for early-exit is a follow-up. It cannot weaken ship-safety.

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
| Mandatory spine | `scout, build, audit, ship` are always present and ordered (`EVOLVE_MANDATORY_PHASES`) |
| Weak-spine guard | Omitting `audit` or `ship` from the spine emits a `weak-spine` warning (`validateSpine`) |
| Conditional TDD-pin | `tdd` is mandatory unless `cycle_size == trivial` or the cycle's `deliverable_kind` is `document` (ADR-0099: the code-specific phase is released for solution cycles. Clauses are AND-ed.) (`EVOLVE_CONDITIONAL_MANDATORY`, default `tdd:cycle_size!=trivial&&deliverable_kind!=document` — `config.DefaultTddRuleExpr`) |
| Ship-needs-real-audit | A plan that reaches `ship` without a PASS audit bound to the built tree is rejected/clamped |
| Insertion cap | The router may insert at most `EVOLVE_MAX_OPTIONAL_INSERTIONS` (default 4) optional phases |
| Unknown-phase drop | An entry naming a phase outside the known-phase set is REMOVED, one `drop-unknown-phase` clamp per removal |

### Unknown-phase drop (cycles 1151/1152)

`ValidatePlan` has always flagged an advisor-invented phase as `unknown-phase`, but the documentation calls it **PURE and REPORT-ONLY**, so nothing removed the entry. Cycles 1151 and 1152 both hard-failed at dispatch (`phase gate-wiring-proof: profile not found`). The advisor minted that name out of operating-policy prose. The clamp is the sole plan disposer, and it only ever *added* phases.

`ClampPlanToFloorWith` now drops those entries before applying the floor, reusing `knownPhaseSet` so the two layers can never disagree about what "known" means:

- **Known** = the union of these sets:
  - the canonical order
  - the configured walk (`Cfg.Order`/`Mandatory`/`Triggers`/`Conditional`, into which the composition root splices both `docs/architecture/phase-registry.json` and `.evolve/phases/<name>/`)
  - the catalog offered to the advisor (`RouteInput.Catalog`)
  - phases minted by this plan, with `plan.MintPhases` *or* inline on an entry (`PhasePlanEntry.Mint`)
- **Removal, not `Run=false`**: dispatch keys off the entry's presence.
- Both `run:true` and `run:false` entries are dropped; each removal emits exactly one `Clamp{Rule: "drop-unknown-phase", Forced: "<phase>=drop"}` so the disposition is never silent.
- The floor still runs afterwards, so a drop can never strand the ship chain. `ClampPlanToFloorWith` stays PURE (a new `Entries` slice; the plan of the caller is unmutated for the re-plan comparison).

The drop must never delete a *legitimate* phase. The known-set is therefore deliberately generous: anything that the plan prompt advertised is known by construction.

### Unavailable-phase drop (2026-09-10)

A catalog-Optional phase whose persona doc does not exist is a deterministically
known configuration absence. In the paused batch of 2026-09-09, the selection of one
cost a dispatch, a recorded skip and (before deterministic
learning) a five-minute retrospective agent. This happened twice, for two known absences.

At plan time core asks every runner that implements `core.PersonaProber` if its persona doc exists.
The runner answers with `runner.BaseRunner.PersonaAvailable` — the same loader, name and inline-prompt
exemption the dispatch path uses. Core asks only for
the phases that `optionalInfraSkip` degrades at dispatch: catalog-Optional, not
configured-mandatory, outside the ship floor. The probe never covers mandatory and floor phases.
Their absence must stay a loud dispatch failure, never a silent exclusion.

The absent phases reach the router as ENVIRONMENTAL context beside
`BenchedCLIs` (`RouteInput.UnavailablePhases`):

- The advisor does not get them in its offer. A "Unavailable phases" prompt section names them instead.
- The floor clamp drops a proposed entry under `DropUnavailablePhaseRule`.
- The legacy trigger path never inserts them.

A phase that the plan never selects costs nothing. A mandatory phase with a missing persona still aborts the cycle as before. The system learns the abort deterministically (see audit-repair-isolation.md).


## Configuration surface

`config.Loader.Load(registryPath, env)` is the **single** reader of routing env + registry.

- The composition root wires the `Loader` with its Signal Center. Every resolution warning is therefore a `config.warning` under module `config`.
- The per-phase callers load through `router.PolicyForProject`. It builds its own `Loader` from the `config.Option`s of the caller. There are none today, so it stays Center-less. The loader that the root wires is the one that emits each registry WARN.
- `TestRoutingConfigLoader_ConstructionSitesArePinned` holds the two construction sites.

The root then applies the gate/recovery/router dials of `.evolve/policy.json` through `Loader.ApplyPolicyStages`. Downstream consumers receive the resolved `RoutingConfig` by injection (`WithRouting`) and never call `os.Getenv`. Precedence: **env override > registry file > built-in default**.

| Env var | Default | Maps to |
|---|---|---|
| `EVOLVE_DYNAMIC_ROUTING` | `advisory` (since 2026-06-06; was `off`) | `Stage` |
| `EVOLVE_ROUTING_MODE` | `llm` | `Mode` |
| `EVOLVE_MANDATORY_PHASES` | `scout,build,audit,ship` | `Mandatory` (CSV) |
| `EVOLVE_CONDITIONAL_MANDATORY` | `tdd:cycle_size!=trivial&&deliverable_kind!=document` | `Conditional` (`phase:expr`; clauses joined by `&&`; op ∈ `!= == >= <= > <`) |
| `EVOLVE_MAX_OPTIONAL_INSERTIONS` | `4` | `MaxInsertions` (int) |
| `EVOLVE_USE_PHASE_REGISTRY` | enabled (`0` disables) | if the loader reads the registry file |

The per-phase legacy enable flags were retired (cycle-39; `config_legacyflags_test.go` pins their absence): `PhaseEnable` comes from the registry's `phases[].enabled` over the compiled floor. This keeps `os.Getenv` out of the phase code.

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

At `Stage >= Advisory` the advisor's whole-cycle plan drives the optional phases, and the registry bounds it.

A phase whose `routing` block has an `insert_when` and no `rubric_hint` declares its trigger as its whole admission rule. A plan that runs it while the trigger does not fire is clamped to skip (`insert-when-gates-plan`, recorded in `routing-decision-N.json` and the ledger).

A `rubric_hint` opens the phase to the advisor's judgment beyond the trigger (architecture-design: "a novel/cross-cutting goal also warrants architecture-design"). An operator `enabled: on` runs the phase whatever its trigger says. Floor phases are never gated.

The advisor plans before scout, so it cannot evaluate a `scout.goal_type` trigger itself. The walk evaluates it at the turn of the phase, when the signal exists (cycle 1733).

A trigger that reads a signal nothing produces makes its phase unreachable at advisory. So each `insert_when` and `skip_when` field must have a producer (2026-10-21). These are the producers:

- a typed routing signal (`router.HarnessProduces`);
- a `<phase>.failure_class` or `<phase>.defect_count` field from a failure block;
- an `outputs.signals` entry of a phase earlier in the routing order.

`phasespec.UnproducedTriggerFields` names each field without a producer, with its phase. `TestTheCheckedInRegistry_EveryTriggerFieldHasAProducer` keeps the checked-in registry clean. The digest folds the handoff `signals` of every completed phase, so a declared output reaches the router.

Two typed signals were added for existing triggers. `ship.class` is `cycle` after the ship of a cycle completes. Changelog-sync, post-ship-monitor and memo read it. `run_dir.artifact_bytes` is the byte total of the run directory, and context-condense reads it. That total includes logs and prompts, so the trigger is true on each real cycle; the inbox item `context-condense-trigger-measures-the-whole-run-dir` owns the fix.

Four triggers read signals with no producer, so they were changed. Authz-gap-scan and threat-model now read `scout.goal_type == security`, as the security recipe row does. Fuzz-probe lost its `scout.surface_type` condition, and its `build.files_touched` condition stays. Preliminary-study is an on-demand phase, and it lost its `campaign.mode` trigger.

A phase's `skip_when` also gates the plan (`skip-when-gates-plan`). It is where a phase states the deliverable kinds it does not admit.

Every bugfix-category phase (fault-localization, bug-reproduction, error-handling-scan, flake-rerun-scan, incident-postmortem) declares `{"field": "deliverable_kind", "op": "eq", "value": "document"}` in its `phase.json`. A document lane therefore never runs them, whatever goal type its scout declares (cycle 1692, ADR-0099 amendment 2026-10-01).

Each removal of a planned phase by either gate raises `ORCHESTRATOR_PLAN_PHASE_GATED` on the Signal Center.

The advisor sees the same rules before it plans. The rubric renders each `skip_when` as a skip line. The plan prompt of a fleet lane carries the pinned item of the lane (id, kind, deliverable kind, acceptance) in a "Lane scope" section.

A phase that the plan runs records `plan:<phase>` as its reason. `forced-on:<phase>` means the enable forced it on the trigger path, where there is no plan.

## The LLM proposer

`core.PhaseAdvisor` (`go/internal/core/phase_advisor.go` — since ADR-0103 unit 04 the seam; the brain is `go/internal/core/advisor`, see [decomposition/04-advisor.md](decomposition/04-advisor.md)) is the bridge-backed `DynamicLLM` brain:

- Asks an LLM, through the `core.Bridge` port, which optional phases to insert/skip given the objective digest (`router.Digest`).
- Defaults to a cheap/fast model (`haiku`) on the `claude-tmux` driver — routing is a lightweight read-only judgment, not heavy generation. Override with `WithProposerCLI` / `WithProposerModel`.
- Output is **advisory**: the pure `router.Route()` clamp re-validates it against the floor. A hallucinated or malformed proposal can never weaken the ship guarantee.
- Any failure (bad JSON, timeout, empty) returns an error and degrades to the deterministic `StaticPreset` — fail-safe to the floor.

Each decision is recorded as a `routing-decision-N.json` artifact in the cycle workspace (`Orchestrator.recordRoutingDecision`) so the Off→Shadow soak can diff advisor rationale vs the static path.

**Hybrid cadence (ADR-0024 §2).** Once an upfront whole-cycle plan is driving (`Stage >= Advisory`, `in.Plan != nil`), `LLMProposal.Decide` invokes the per-transition `Propose` ONLY at **branch transitions**. These are post-build and post-audit, where new objective signals (`acs_red`, audit verdict) appear. The signal-poor start-of-cycle plan was unable to foresee them.

The cached plan already decides every other transition. A proposal can never change the `NextPhase` of the kernel regardless (`applyProposal` only annotates + clamps). A call to the LLM there was pure cost.

With NO upfront plan (Shadow, static mode, planner failure) the legacy per-transition cadence stands, so Shadow-soak forensics are unchanged. Gate: `router.shouldPropose(in)` → `isBranchTransition(in.Current)`.

## Integration status

| Aspect | State |
|---|---|
| Kernel on `main` | Yes — `config` + `router` + `core.PhaseAdvisor`, fully unit-tested + the `routingtest` "Lego" scenario framework |
| Default | Off — `WithRouting` is opt-in; absent it, the orchestrator runs legacy Stage:Off |
| First live run | cycle-108 (`advisory`+`llm`) — proved the proposer emits valid strict JSON end-to-end; see `docs/research/cycle-108-routing-live-data.md` |
| Known evolution | ADR-0024 (Proposed): shrink the hard mandatory set to a single conditional invariant + rename `RoutingProposer`→`PhaseAdvisor`, hybrid cadence, `phase-plan.json` |

## Anti-patterns and guards

| Anti-pattern | Why bad | Guard |
|---|---|---|
| Reading a routing env var outside `config.Loader.Load` | Splits the composition root; reintroduces `os.Getenv` sprawl | `config` is a leaf package (stdlib + `signalcenter` + `paths`, import-allowlist-tested); orchestrator/phases take `RoutingConfig` by injection |
| Letting the LLM proposal drive ship directly | Reopens the reward-hacking surface | `router.Route()` clamp re-validates every proposal against the floor before any phase runs |
| Dropping `audit` or `ship` from `EVOLVE_MANDATORY_PHASES` | Breaks audit-before-ship | `validateSpine` emits `weak-spine`; ship-needs-real-audit still clamps |
| Hard-failing on an unknown stage/mode value | A typo can break autonomy | Unknown values resolve to safe defaults (`Off` / `llm`) + a warning |

## See also

- [psmas-phase-scheduling.md](psmas-phase-scheduling.md) — legacy bash Triage-driven phase-skip path (the predecessor concept)
- [phase-architecture.md](phase-architecture.md) — canonical static phase sequence the router operates within
- [sequential-write-discipline.md](sequential-write-discipline.md) — `parallel_eligible` single-writer invariant the registry carries
- [adr/0024-conditional-ship-gate-floor-and-phase-advisor.md](adr/0024-conditional-ship-gate-floor-and-phase-advisor.md) — Proposed PhaseAdvisor evolution
- [control-flags.md](control-flags.md) — full `EVOLVE_*` control surface
- `docs/architecture/phase-registry.json` — the data-only phase catalog (schema v3)
