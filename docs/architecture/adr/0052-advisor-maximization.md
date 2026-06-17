# ADR-0052: Advisor maximization — re-invokable, debuggable, multi-model PhaseAdvisor

> Status: **Accepted** (2026-06-17). Foundations slice (WS0-S1) landed; behavior workstreams (WS1–WS6) land incrementally behind safe-default rollout dials. Builds on the routing kernel's "model proposes, kernel disposes" pattern ([[dynamic-phase-routing]], ADR-0024), the integrity floor (`router.ClampPlanToFloorWith`), phase recovery (ADR-0044) and corrective interaction (ADR-0045), and the unified phase-I/O substrate (ADR-0050).

## Context

The evolve-loop kernel already runs a live-by-default LLM router — the **PhaseAdvisor** (`go/internal/core/phase_advisor.go`, persona `agents/evolve-router.md`, profile `.evolve/profiles/router.json`, artifact `routing-plan.json`). "Model proposes, kernel disposes": the advisor emits an advisory whole-cycle plan, and a non-LLM clamp (`router.ClampPlanToFloorWith`, `go/internal/router/floor.go`) enforces the integrity floor (`scout→build→audit→ship` + TDD-pin + ship-needs-real-audit). Live data confirms real need-matching (cycle-294 inserted `fault-localization`+`bug-reproduction` on a bugfix goal; `bug-reproduction` advisor-inserted across 60+ cycles).

Four verified capability gaps cap the advisor:

1. **Signal-poor upfront planning.** The whole-cycle `Plan()` that selects the phase set runs with an empty `Signals` struct (`cyclerun.go` `planCycle`, "no handoffs exist yet"); need is *inferred from goal text*, not *measured*.
2. **Reactive proposals are weak.** Per-transition `Propose()` is gated to `{build,audit,retro}` and is annotate-or-clamp only.
3. **Three-source knowledge drift.** Recipe table (persona markdown), registry `insert_when`, and per-phase `when_to_use` are maintained separately; only triggers are projected into the prompt (`writeRubricLines`). Most catalog phases are metadata-thin.
4. **No eval/regression lock + missing forensics.** Routing tests use a scripted proposer (`routingtest.AgentSpec`); the raw advisor prompt and response are not persisted (only the parsed decision is).

**Goal:** maximize the advisor's ability to choose the optimized path to complete the goal, and make the LLM that drives development debuggable, scalable, maintainable, and multi-model configurable via the agent bridge — grounded in researched AI-driven control / harness-debuggability / multi-model-routing best practice (plan-then-act + branch-only reconsult; signal-conditioned re-planning; deterministic floor non-negotiable; OTel-GenAI decision spans; golden-master eval + replay; single-source projection).

## Decision

### D1 — Unified agent substrate + separate control plane

The advisor shares the phase-agent definition substrate (persona + profile + artifact + bridge dispatch + model-config + decision-trace) and becomes **re-invokable** (a post-scout re-plan, in addition to the initial plan). It stays a control-plane **"compose" archetype** — it transforms the *plan*, never the *repo*, and is **never a node in the executed spine**. The non-LLM floor (`ClampPlanToFloorWith`) remains the **sole trust boundary**, strictly above the advisor and above execution. A recursion guard prevents the advisor from minting or dispatching another advisor.

### D2 — Optimize for goal-completion confidence

Compose the path that most reduces the risk of an incomplete or incorrect result; cost/latency is a **soft secondary** constraint. Consequently, the confidence-critical decisions (the whole-cycle plan and the post-scout re-plan) use a **deep** model; the cheap/fast tier is reserved for the lightweight reactive `Propose` and the optional LLM route-quality judge, both off the critical path.

### Architecture (layering invariant)

```
CONTROL  PhaseAdvisor (agent)   AgentIdentity{cli,model,profile,persona,label}
PLANE       Plan()   ─► initial whole-cycle plan (cycle start; + deterministic recon digest)
            RePlan() ─► post-scout re-plan (mismatch-gated, depth ≤ 1)
            Propose()─► reactive tweak at {build,audit,retro} (annotate-only)
            every call: redact → record raw prompt+response → decision span
KERNEL   ValidatePlan ─► ClampPlanToFloorWith   ◄── SOLE TRUST BOUNDARY (unchanged)
DATA     scout → build → audit → ship (+ advisor-selected optional phases)
```

The advisor is never scheduled by the plan it emits (no bootstrap paradox); the floor is always below the advisor and above execution. The post-scout re-plan fires only after scout's handoff has been recorded and before the next phase is selected, so it can never retroactively drop a completed anchor.

### Rollout dials (the flag family)

All behavior-affecting work lands behind dials at safe defaults, mirroring the proven `EVOLVE_DYNAMIC_ROUTING` `off → shadow → advisory` rollout. Nothing flips silently.

| Flag | Default | Wires in | Role |
|------|---------|----------|------|
| `EVOLVE_ROUTER_REPLAN` | `shadow` | WS2-S3 | Post-scout re-plan dial: off / shadow (compute+log) / advisory (replace after re-clamp). **Registered + parsed in WS0-S1.** |
| `EVOLVE_ROUTER_RECON_DIGEST` | `off` | WS2-S0b | Inject the deterministic pre-plan recon digest into the initial `Plan()` (off → on; byte-identical off). |
| `EVOLVE_ROUTER_REPLAN_DEPTH` | `1` | WS2-S5 | Re-plan depth cap (escalate to debugger at cap, never thrash). |
| `EVOLVE_ROUTING_JUDGE` | `off` | WS4-S3 | Opt-in LLM-as-judge route-quality scoring, off the build path. A plain bool (not a stage — the judge cannot move behavior). **Registered + parsed in WS4-S3** (`RoutingConfig.RoutingJudge`); `core.PlanJudge.GradePlan` is the scorer, and the composition-root scoring call site reads the flag to gate whether it is invoked (behavior wires in WS4-S3). |
| `EVOLVE_ADVISOR_DEPTH` | unset | WS1-S2 | Defense-in-depth env recursion stamp (primary guard is the mint denylist). |
| `EVOLVE_ROUTER_PLAN_MODEL` / `EVOLVE_ROUTER_PROPOSE_MODEL` | unset | WS6-S1 | Optional per-decision-type model override (default: deep for plan/replan, fast for propose/judge). |

Each flag is added to the `flagregistry` SSOT (projected into `docs/architecture/control-flags.md`) in the slice that adds its reader — not pre-registered — to keep the registry honest. WS0-S1 registers and parses `EVOLVE_ROUTER_REPLAN` (composition-root `RoutingConfig.RouterReplan` view); the remaining dials are documented here as the roadmap-of-record and registered as their slices land.

### Safety seams (load-bearing, guarded)

- **Floor stays the sole boundary:** `ValidatePlan` runs before `ClampPlanToFloorWith` and can only reject-or-pass; the clamp still runs last, unconditionally. `floor.go` is untouched.
- **nil = off everywhere; fail-safe to static:** a nil mismatch detector ⇒ no re-plan; a noop recorder ⇒ no artifacts; any `Plan`/`RePlan`/`Propose` error degrades to the already-clamped plan, itself fail-safe to the static spine.
- **Recursion guard (primary):** a mint denylist in `mintConfigsFrom` rejects any minted phase named or role-typed as the router; the `EVOLVE_ADVISOR_DEPTH` env stamp is defense-in-depth only.
- **Re-plan can't contradict completed work:** it fires only post-scout, pre-build, after `recordAndBranch` has recorded the scout anchor that `SpineSatisfiedUpTo` keys off.
- **Secret redaction:** every captured prompt/response is routed through `panetrust` redaction before persist + ledger-bind.

WS0-S1 installs `TestArchitectureSeams_FoundationsExist` (`go/internal/core/seams_test.go`) — a standing guard that fails the build if any of these seams (`recordAndBranch`, `selectNext`, `registerMintedPhases`, `mintConfigsFrom`, `writeRubricLines`, `SpineSatisfiedUpTo`, `ClampPlanToFloorWith`, `panetrust.redactSecrets`) is renamed or removed before its slice wires it.

## Consequences

- The advisor gains a measured post-scout re-plan and a deterministic pre-plan recon digest, closing gap #1 on two fronts (reactive *and* upfront).
- Raw prompt/response forensics + decision spans + `evolve routing explain|replay` make the LLM driver debuggable.
- A golden route corpus + deterministic replay lock prompt/model regressions; a single-source projection kills the three-way recipe drift.
- The integrity floor's role is unchanged and re-asserted on every path; the rollout dials mean each step is observable in shadow before it can move behavior.

## Alternatives considered

- **Advisor-spawned research agents** (let the advisor gather info before justifying): rejected — breaks the compose-vs-execute layering and the recursion guard, and costs tokens. The LLM-recon case is already served by Scout + the WS2 re-plan; the *deterministic* recon need is served by the WS2-S0b digest (Core Rule 5: deterministic facts belong in code, not a prompt).
- **Contextual-bandit / online-learning route selection** keyed off route-quality history: out of scope — it optimizes cost-first, while D2 is quality-first. Recorded here as future work.
- **Defer WS6 (per-decision multi-model):** reviewed and consciously declined — WS6 lands as committed code with strictly no-op defaults (single model until a profile opts in).

## Future work

Contextual-bandit / online-learning route selection keyed off the WS4 route-quality history (see `EVOLVE_ROUTING_JUDGE`). Explicitly out-of-scope for the quality-first program; revisit only with a cost-pressure motivation and the eval harness to gate it.
