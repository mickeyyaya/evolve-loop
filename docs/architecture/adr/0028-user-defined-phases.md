# ADR-0028: User-Definable Phases (the "Lego Pipeline")

> **Invariant: a phase is a Lego brick behind one uniform contract, and an operator adds one as pure data.** Drop `.evolve/phases/<name>/{phase.json, agent.md, profile.json}`; the engine runs it through the same `core.PhaseRunner` contract every built-in phase uses. The orchestrator advances as a pure driver that never names a phase. The `build → audit → ship` spine + "ship requires audit-PASS bound to the build tree" stay kernel-clamped — a user phase is **optional-only** and can never displace or satisfy the floor.

- **Status:** Accepted — partially implemented. Data model, signal plane, and authoring CLI are SHIPPED (branch `feat/user-defined-phases`); the runtime execution integration is designed below and remains.
- **Date:** 2026-05-27
- **Relates:** ADR-0024 (conditional ship-gate floor + PhaseAdvisor — the floor this builds on), ADR-0027 (commit-as-evidence — the completion signal), ADR-0020 (phasestream normalizer), and the dynamic-routing kernel (`go/internal/router`).

## Context

Phases were a **closed set**: `core.Phase` is a hardcoded enum, `IsValid()` a closed switch, `statemachine.go` hardcodes the transition graph + `mandatoryAnchors`, and the **signal plane** is closed four ways (`digest.go` per-phase `extractX`, the `RoutingSignals` typed struct, `resolveField`'s switch, and bespoke-per-phase `handoff-<phase>.json` shapes). Adding a phase meant editing Go in 5–10 places.

Yet the framework was ~80% present: a uniform `core.PhaseRunner` contract, the `BaseRunner`+`Hooks` Template Method, an open `phases/registry` (Factory Method, OCP), a declarative `phase-registry.json`, and a data-driven router with a kernel clamp. The walls were *identity* and *signals*, not the execution model.

Research confirmed the target is the canonical **Pipes & Filters** pattern (uniform interface, reorderable while I/O schemas match, fault-isolation), realized as an Argo-style **DAG of nodes behind a uniform interface** (uniform contract + *declared* inputs/outputs the engine wires).

## Decision

**Three unified contracts + a kernel-clamped floor; rolled out on the existing `EVOLVE_DYNAMIC_ROUTING` ladder so the live static path stays byte-identical.**

### 1. Unified interface (Go)
Keep `core.PhaseRunner` (`Name()` + `Run(ctx, PhaseRequest) → PhaseResponse`). Additive envelope fields so signals flow *through* the contract instead of being re-parsed from markdown: `PhaseRequest.Spec` + `UpstreamSignals`; `PhaseResponse.Signals` + `CommitSHA`. (`NextPhase → NextHint` demotion deferred to the runtime stage.)

### 2. Unified JSON
- **PhaseSpec** — the brick definition (`phasespec.PhaseSpec`), one schema with two sources: the built-in `phase-registry.json` array (schema v4) and per-phase `.evolve/phases/<name>/phase.json`, merged into one `Catalog` (built-ins win on clash). Fields: `kind` (llm | native|command reserved), `agent`, `model`, `writes_source`, typed `inputs/outputs.{files,signals}`, `prompt_context`, declarative `classify`, `routing`, `gates`.
- **Uniform handoff envelope** — every phase's `handoff-<phase>.json` carries a top-level `signals` object; `Digest.foldGeneric` merges it into a namespaced `<phase>.<key>` bus (`RoutingSignals.Generic`), read generically by `resolveField`. Collapses the four closed signal layers into one.
- **PhasePlan** (`phase-plan.json`) — the queue body the PhaseAdvisor produces and the floor-clamped router mutates (runtime stage).

### 3. Unified pipeline
The orchestrator is a pure driver: pop the next spec, run it through the contract, fold `resp.Signals` onto the bus, let `router.Reconcile` (the sole plan mutator) insert/skip — **clamped by FLOOR**:
```
FLOOR(plan, bus) holds iff:
  reach(ship) ⇒ build precedes ship ∧ audit precedes ship
  run(ship)   ⇒ bus["audit.verdict"] ∈ {PASS,WARN} ∧ bus["audit.tree_sha"] == bus["build.tree_sha"]
  bus["cycle_size"] != "trivial" ⇒ tdd precedes build
  optional/user phases may insert anywhere the above still holds
```

### Generic spec runner
`specrunner` derives a full `runner.Hooks` from a `PhaseSpec` (artifact/agent/model/prompt/classify), so a `kind:llm` phase needs **zero Go**. Built-in phases keep their hand-written runners (explicit factory wins over spec-derived).

### Safety
User phases are **optional-only** (`ValidateUserSpec` rejects `optional:false`); only `kind:llm` is executable; `verdict_on_pass` must be canonical; names are kebab-case-floored before any filesystem write (no traversal).

## Implementation status

| Layer | State | Commit |
|---|---|---|
| PhaseSpec model + Catalog + generic spec runner + envelopes | **shipped** | `ce88e3b` |
| Uniform signal plane (Generic + foldGeneric) + registry v4 | **shipped** | `95d7f54` |
| `resolveField` → generic bus (conditions route on user signals) | **shipped** | `e5acb2a` |
| `evolve phases list/validate/add` + discovery + floor validation | **shipped** | `cc999e0` |
| Runtime execution (catalog-aware IsValid/CanTransition/order, spec-runner registration, WorktreePhase from spec, FLOOR refinement, route user phase via dynamic router) | **designed, remaining** | — |
| Authoring guide `docs/architecture/user-defined-phases.md` | remaining (write once runtime e2e works) | — |

## Consequences

- **Positive:** a phase becomes pure data; the orchestrator stops knowing phases by name; the signal plane is open so user phases participate in routing; the floor is a non-gameable invariant, not a hardcoded list. Built-in/live behavior is unchanged until the runtime stage lands on the dynamic path.
- **Negative / watch:** the static state machine (Stage:Off) is intentionally NOT inverted — user phases require dynamic routing (`advisory`/`enforce`). The runtime pieces are mutually dependent (each is inert alone) and must land + e2e-test together. `kind:native|command` are reserved but unbuilt.
- **Rollout:** `shadow → advisory → enforce` per `EVOLVE_DYNAMIC_ROUTING`; soak `shadow` against the live loop and diff would-have-routed vs static before each tier bump.
