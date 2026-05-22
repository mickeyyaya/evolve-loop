# ADR-0019 — Build Planner Phase (Opt C)

| Field | Value |
|-------|-------|
| Status | Proposed |
| Date | 2026-05-22 |
| Cycle (shadow wiring) | 103 |
| Cycle (advisory flip) | 104 (planned) |
| Cycle (enforce) | 105 (planned) |
| Author | Scout cycle-103 via operator inbox spec |

---

## Status

Status: Proposed — cycle-103 wires infrastructure in shadow mode (`EVOLVE_BUILD_PLANNER=0` default). No build-plan.md is produced until cycle-104 flips the default to advisory.

---

## Context

Builder's internal chain-of-thought design step (Step 3 in `evolve-builder.md`) is performed within the same LLM session that writes production code. Research from cycle-40 (`cycle-40-builder-context-crosscheck`) shows quantitative claims in build-reports are not reconciled against the actual approach taken — Builder's planning and execution share a context, so design drift is invisible to the Auditor.

Externalizing the planning step into a dedicated phase creates:
1. An independent Opus session (different family from Builder's Sonnet) for design reasoning
2. An artifact (`build-plan.md`) the Auditor can compare against the actual diff
3. A structural separation that enables Plan Adherence checks in cycle-105

The 3-cycle rollout avoids blast radius:
- **Cycle 103 (shadow):** Infrastructure wired; `EVOLVE_BUILD_PLANNER=0` default; no persona runs
- **Cycle 104 (advisory):** Default flips to `1`; build-plan.md produced; Builder reads it but is not gated on it
- **Cycle 105 (enforce):** Builder's own Step 3 removed; Auditor gates on plan adherence; deviation requires logged rationale

---

## Decision

Introduce a `build-planner` phase between TDD and Build:

**New files:**
- `agents/evolve-build-planner.md` — Opus persona, single-writer, write-once artifact
- `.evolve/profiles/build-planner.json` — `parallel_eligible: false`, `max_turns: 10`, `max_budget_usd: 0.30`
- `docs/architecture/adr/0019-build-planner-phase.md` (this file)

**Edited files:**
- `docs/architecture/phase-registry.json` — build-planner entry between tdd and build; `tdd.gate_out` → `gate_tdd_to_build_planner`
- `scripts/dispatch/list-phase-order.sh` — `build-planner` inserted in `emit_hardcoded_order()`
- `scripts/dispatch/subagent-run.sh` — `build-planner` added to both allowlist regexes
- `scripts/guards/phase-gate-precondition.sh` — `build-planner` recognized; `build-planner` phase arm added
- `scripts/lifecycle/phase-gate.sh` — `gate_tdd_to_build_planner()` and `gate_build_planner_to_build()` declared and wired
- `agents/evolve-orchestrator-reference.md` — conditional step using `${EVOLVE_BUILD_PLANNER:-0}`

**Enable flag:** `EVOLVE_BUILD_PLANNER` — default `0` (shadow). Set `1` to activate.

**Revert path:** Set `EVOLVE_BUILD_PLANNER=0` to revert to pre-cycle-103 behavior. The wired infrastructure is a no-op when the flag is off.

---

## Consequences

**Positive:**
- Build planning externalized to an independent Opus session with a durable artifact
- Auditor gains a Plan vs. Diff comparison surface (cycle-105)
- Single-writer invariant preserved (`parallel_eligible: false`)
- Zero runtime behavior change in shadow mode (`EVOLVE_BUILD_PLANNER=0`)

**Negative / risks:**
- Adds ~1-2 minutes wall time per cycle when enabled
- Adds ~$0.30 per cycle when enabled (Opus, 10 turns)
- If build-plan.md diverges from TDD contract, Builder faces conflicting signals — advisory mode (cycle-104) mitigates by keeping Builder autonomous

**Deferred:**
- Plan Adherence gate (Auditor checks build-plan.md vs diff) deferred to cycle-105 (enforce)
- Builder Step 3 removal deferred to cycle-105 (enforce)

---

## References

- Operator spec: `/Users/danleemh/.claude/plans/opt-c-cycle-1-shadow-build-planner-wiring.md`
- Phase registry: `docs/architecture/phase-registry.json`
- Cycle-40 instinct: `cycle-40-builder-context-crosscheck` (design drift without external artifact)
- Sequential write discipline: `docs/architecture/sequential-write-discipline.md`
