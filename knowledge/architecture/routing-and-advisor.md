# Routing & the Phase Advisor — "model proposes, kernel disposes"

> Dynamic routing lets an LLM **advisor** decide *which phases a cycle needs* —
> skipping discovery on a carryover cycle, inserting a `tester` after a risky
> build, ending early on an investigation cycle. The safety story is one
> non-bypassable invariant: the advisor can never reach `ship` without a real
> `build ∧ audit ∧ (tdd, unless trivial)`. This document describes the Go routing
> kernel (`go/internal/router`), the `PhaseAdvisor`, the integrity floor, and the
> rollout stages. Authoritative ADR: **ADR-0024**. Current design (v13.0.0).

Related: [phase-pipeline.md](phase-pipeline.md) ·
[trust-kernel-and-egps.md](trust-kernel-and-egps.md) ·
[state-and-ledger.md](state-and-ledger.md) ·
[glossary](../00-overview/glossary.md)

---

## 1. Why dynamic routing exists

The static state machine ([phase-pipeline.md](phase-pipeline.md) §2) runs the
*same* sequence every cycle. That is wasteful and rigid: a one-line carryover fix
doesn't need a full Scout; an investigation cycle shouldn't be forced to ship; a
large diff might want a `plan-review` or a `tester`. ADR-0024's intent: **shrink
the hard mandatory set to its safety-essential core and let an LLM advisor justify
which phases are needed per cycle.**

The risk this creates: the entire integrity model (EGPS gate, audit-binding,
adversarial auditor — see [trust-kernel-and-egps.md](trust-kernel-and-egps.md))
rests on *not being able to ship unverified code*. A naïve "only TDD mandatory"
would let the advisor route `… → ship` skipping audit, re-opening that whole
surface. So routing is governed by **"model proposes, kernel disposes."**

---

## 2. The routing kernel is pure

`router.Route` (`go/internal/router/router.go`) is a deterministic function:
`RouteInput → RouterDecision`. The caller (the orchestrator) does all I/O and
hands in plain values — completed phases, the digested signals, the config, the
budget remaining, and (optionally) the advisor's plan. `Route` never touches git,
the network, or an LLM. **Why:** the clamp is the entire safety story, so it must
be exhaustively table-testable with no side effects (ADR-0004).

`Route` is an ordered rule set:

- **Rule 0 — retro delegation.** A `retrospective` current defers to the
  deterministic failure-adapter (never duplicate failure logic).
- **Rule 1 — audit branch.** `audit + FAIL` → `retrospective`; `PASS`/`WARN` fall
  through toward ship.
- **Rules 2–4 — the walk.** Advance the canonical order from `current+1` to the
  first runnable phase, accumulating chosen inserts and declined skips; apply the
  LLM proposal as advisory, clamped to the legal next.

The canonical order (`router.go:canonicalOrder`):
`intent → scout → triage → plan-review → tdd → build-planner → build → tester →
audit → ship → retrospective → memo`. The mandatory spine
(`scout → build → audit → ship`) is a *subset*; optional phases sit between
anchors and run only when triggered, enabled, or planned.

---

## 3. `shouldRun` — the per-phase decision

`shouldRun` (router.go) is where a phase's run/skip is decided, in priority order:

1. **Configurable-mandatory** (`cfg.Mandatory`, default `scout,build,audit,ship`).
   Always run; an `enable=off` cannot disable them (recorded as a
   `mandatory-never-skipped` clamp).
2. **Conditional-mandatory** (`cfg.Conditional`, default `tdd:cycle_size!=trivial`).
   tdd is pinned on unless the cycle is trivial.
3. **Plan-driven** (Stage ≥ Advisory with an advisor plan). The
   *already-floor-clamped* whole-cycle plan drives run/skip for every
   non-mandatory phase. A phase the advisor scheduled runs; one it omitted is
   genuinely optional this cycle (scout/triage included, when the operator shrinks
   the mandatory set). The insertion cap is intentionally **not** applied here —
   the plan is the advisor's coherent, budget-aware selection, clamped by the
   floor rather than capped.
4. **Trigger-driven** (the legacy path, Stage < Advisory or no plan).
   `EnableContent` phases insert when their `insert_when` triggers fire, subject
   to budget and the `MaxInsertions` cap.

> The two-tier mandatory model is the crux: the **configurable** never-skip set is
> `EVOLVE_MANDATORY_PHASES`; the **non-configurable** integrity floor (§4) is
> separate, so an operator may shrink the mandatory set to e.g. `tdd` and the
> kernel still cannot reach ship without a real build + audit.

---

## 4. The integrity floor — one conditional invariant

ADR-0024 §1 replaced the *fixed phase list* with a single **causal implication**,
implemented in `router/floor.go:ClampPlanToFloor`:

```
reach(ship)  ⇒  build ∧ audit ∧ (tdd, unless the cycle is trivial)
```

`ClampPlanToFloor` is a pure plan-level prefilter. Given the advisor's whole-cycle
plan:

- If the plan does **not** run `ship`, nothing is forced — a no-ship cycle may
  legitimately end after scout (investigation / convergence). The antecedent is
  false, so the floor imposes nothing.
- If the plan **does** run `ship`, the clamp *forces* `build` + `audit` on (and
  `tdd` unless trivial, reusing the kernel's `tddPinned` rule so the exemption
  stays consistent with `shouldRun`), recording one `Clamp` per forced phase. It
  returns a **new** plan (input unmutated).

**The clamp can only COMPLETE the ship-chain, never weaken it.** A hallucinated or
adversarial plan that tries to reach ship by omitting audit gets audit forced back
in. This is "model proposes, kernel disposes" narrowed to one load-bearing
invariant.

> Defense in depth: the floor forces audit to *run*, but the "audit must PASS bound
> to the built tree" guarantee remains with ship's audit-binding (tree-SHA match +
> EGPS `red_count==0`) and the artifact-backed `SpineSatisfiedUpTo` gate (§5). The
> floor is never the sole gate (`floor.go` doc comment).

---

## 5. The artifact-backed spine gate

Forcing audit to *run* is not enough — the orchestrator must also refuse to *reach*
ship unless a real audit artifact with a shippable verdict exists.
`StateMachine.SpineSatisfiedUpTo` (`core/statemachine.go`) is that gate. It keys off
`RoutingSignals.<anchor>.Present` — digested from **real on-disk handoffs** — and
requires that every configured-mandatory anchor ordered *before* the target has
produced its artifact this cycle. Audit additionally requires
`Verdict ∈ {PASS, WARN}` (`anchorArtifactPresent`).

> Because it reads real artifacts, the orchestrator cannot reach ship by merely
> *claiming* audit passed — a real audit artifact with a shippable verdict must
> exist on disk. This is the non-gameable floor that survives whichever routing
> Strategy is selected.

The spine gate is fail-open for a *missing* mandatory-predecessor artifact on the
trusted static edge (a louder-than-normal WARN, recorded in the decision) —
because the digest is fail-open and a read miss shouldn't false-block a real cycle.
But on a *divergent* (router-override) edge, `enforceNext` (orchestrator.go)
declines any candidate that fails `SpineSatisfiedUpTo`, so an override can never
leapfrog a mandatory anchor.

---

## 6. The PhaseAdvisor and hybrid cadence

The `PhaseAdvisor` (`core.PhaseAdvisor` / `router.Planner`) is the LLM that
produces the plan. ADR-0024 §2 made its cadence **hybrid**:

- **One upfront whole-cycle plan** at cycle start (cheap, coherent), computed only
  at `Stage >= Advisory` AND `Mode == DynamicLLM` AND a planner is wired
  (orchestrator.go:521). The raw plan is immediately run through
  `ClampPlanToFloor` and persisted to `phase-plan.json` + a hash-bound
  `phase_plan` ledger entry (`recordPhasePlan`).
- **Re-consult only at real branch transitions** (post-build: insert `tester`?
  post-audit: `retro`/`memo`?). Signal-free transitions (e.g. `start→scout` when
  scout is already planned) do **not** call the LLM.

**Why hybrid:** the first live advisory cycle (cycle-108) proved most transitions
were *signal-free* — the start-transition proposer had empty objective signals and
its output was discarded in favor of the deterministic spine, pure wasted spend.
The upfront plan + branch-only re-consult removes that double-spend; a per-transition
proposal never changed the kernel's `NextPhase` anyway, so the change is
forensics-only with zero routing change.

The advisor output artifact is `phase-plan.json`: a bare array of
`{phase, run, justification}` (`router.PhasePlanEntry`). Each justification is
carried into the ledger so the shadow soak can diff advisor-rationale against the
static path (ADR-0024 problem #2). Any advisor error (bad JSON, timeout, empty)
degrades to the deterministic static plan — **fail to the floor** (`clampedPlan`
stays nil → the legacy trigger path runs unchanged).

---

## 7. Rollout stages — off → shadow → advisory → enforce

`EVOLVE_DYNAMIC_ROUTING` controls how live the kernel is (CLAUDE.md). The
orchestrator never reads the flag itself — `config.Load` (the composition root)
does, and injects `config.RoutingConfig` + a `RoutingStrategy` via `WithRouting`:

| Stage | Behavior |
|---|---|
| `off` / `0` | Static state machine drives. No digest, no ledger entry — **byte-identical** to pre-routing. |
| `shadow` | Router computes + logs the would-have-routed plan; static still drives. The promotion-gate soak diffs shadow-vs-static. |
| `advisory` | The advisor **drives** non-mandatory run/skip: the clamped whole-cycle plan gates every non-mandatory phase, and `enforceNext` can override the static successor (re-validated by `CanTransition` + `SpineSatisfiedUpTo`). |
| `enforce` | Same driving authority; the non-bypassable floor is identical. |

The integrity floor is LIVE from Advisory onward (ADR-0024 PR-16 widened
`enforceNext` from Enforce→Advisory). Companion flags: `EVOLVE_ROUTING_MODE`
(`llm` = advisor proposes + kernel clamps; `static` = triggers + spine only),
`EVOLVE_MANDATORY_PHASES` (CSV — omitting `audit`/`ship` emits a `weak-spine` WARN,
now backstopped by the integrity floor), `EVOLVE_CONDITIONAL_MANDATORY`
(default `tdd:cycle_size!=trivial`), `EVOLVE_MAX_OPTIONAL_INSERTIONS` (default 4,
not applied to plan-driven inserts), `EVOLVE_USE_PHASE_REGISTRY`. Precedence:
env > registry file > built-in default.

**Reversibility** is the design discipline: `EVOLVE_DYNAMIC_ROUTING=off` reverts to
byte-identical legacy, and each tier promotes independently via the
shadow→advisory→enforce machinery.

---

## 8. User-defined phases ride the same rails

The orchestrator accepts user-defined (non-built-in) phases on the dynamic-routing
path *without* hardcoding them in the `Phase` enum or state machine, via the merged
`phasespec.Catalog` injected with `WithCatalog` (orchestrator.go). A user-phase
candidate is legal only if it makes forward progress in the configured order and is
marked `Optional` (`transitionLegal`); leapfrogging a mandatory anchor is
independently blocked by `SpineSatisfiedUpTo`. A user phase that sets
`writes_source` runs in the worktree like built-in `tdd`/`build`. The empty default
catalog keeps behavior byte-identical to the built-in-only pipeline (ADR-0028).

---

## 9. Forensics — every decision is recorded

When routing runs (`Stage != Off`), the orchestrator records each decision to
`routing-decision-<seq>.json` plus a hash-bound `routing_decision` ledger entry,
and emits one `phase_skipped` entry per declined optional phase
(`recordRoutingDecision`). The upfront plan lands in `phase-plan.json` +
`phase_plan`. Every integrity-floor `Clamp` is logged. **Why:** the clamp is the
safety story, so it must be auditable — an operator (or the shadow soak) can always
reconstruct *what the advisor proposed* vs *what the kernel forced*. All of this is
best-effort: a forensics write failure WARNs and is swallowed, never aborting a
cycle.
