# ADR-5: Per-Phase Standalone Init + Input Verification

**Status:** Accepted  
**Cycle:** 58  
**Date:** 2026-05-15  
**Authors:** evolve-builder (cycle 58)

---

## Context

Each phase of the evolve-loop pipeline has a dedicated slash command (`/scout`, `/build`, `/audit`, …), a skill, a persona, a profile, and a phase-gate function. However, invoking any of these commands standalone — outside of a full orchestrator-driven cycle — is fragile. Nothing initializes `cycle-state.json` to indicate which phase should be active, and nothing verifies that the phase's declared input artifacts (scout-report.md, triage-decision.md, build-report.md, etc.) actually exist before the phase agent begins.

The consequence: operators who want to run `/scout` or `/audit` in isolation must perform a brittle manual setup, and if prerequisites are missing, the phase agent may produce misleading output or fail with cryptic errors instead of a clear "input missing" message.

This problem is documented in the architecture redesign plan (§ADR-5, `/Users/danleemh/.claude/plans/100-focus-on-only-smooth-kay.md`) as Debt B — phases not being truly independent components.

---

## Decision

Two new utility scripts make every phase reliably standalone-invocable:

### `scripts/utility/init-standalone-cycle.sh`

Bootstraps a cycle for single-phase execution. Creates:
- `$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json` with the correct `phase` and `active_agent` for the target phase
- `$EVOLVE_PROJECT_ROOT/.evolve/state.json` with minimal defaults (if absent)
- `$EVOLVE_PROJECT_ROOT/.evolve/runs/cycle-N/` workspace directory

Behavior:
- Reads `docs/architecture/phase-registry.json` to get the role (agent) for the phase
- Maps phase-registry `name` → cycle-state internal `phase` value via a hardcoded case statement (see Phase Name Mapping below)
- Clobber guard: refuses to overwrite an active `cycle-state.json` unless `--force-overwrite` is passed
- For `--phase build` only: provisions a git worktree at `.evolve/worktrees/cycle-N`
- Warns (stderr) about missing inputs but exits 0 — input gating is `check-phase-inputs.sh`'s job
- Atomic writes via `mv -f tmp.$$ target` throughout

### `scripts/utility/check-phase-inputs.sh`

Verifies all declared inputs for a phase are present. Reads `phase-registry.json` `inputs.files[]` and `inputs.state_fields[]` for the requested phase, then checks each:
- Required files: checks for existence at `$EVOLVE_PROJECT_ROOT/<path>` with `{cycle}` substituted
- Required state fields: checks `jq 'has($field)' .evolve/state.json`

Exit codes:
- `0` — all inputs present; phase may proceed
- `1` — one or more inputs missing (list printed to stdout with `MISSING:` prefix)
- `2` — registry not found, or unknown phase name, or jq unavailable

---

## Phase Name Mapping

`cycle-state.json` uses internal phase names that differ in two cases from the phase-registry `name`:

| phase-registry `name` | cycle-state `phase` value | `active_agent` |
|---|---|---|
| intent | intent | intent |
| scout | research | scout |
| triage | triage | triage |
| plan-review | plan-review | plan-reviewer |
| tdd | tdd | tdd-engineer |
| build | build | builder |
| tester | test | tester |
| audit | audit | auditor |
| ship | ship | orchestrator |
| retrospective | retrospective | retrospective |
| memo | learn | memo |

This mapping is hardcoded in `init-standalone-cycle.sh` as a `case` statement (bash 3.2 compatible, no associative arrays). The discrepancies (`scout→research`, `tester→test`) exist because `cycle-state.sh` uses historical internal names that predate the phase-registry.

---

## Consequences

### Positive

- `/scout`, `/build`, `/audit` etc. can be invoked individually with clear error messages when prerequisites are missing.
- Phase agents no longer need to perform defensive "does this input exist?" checks — `check-phase-inputs.sh` provides a reusable pre-gate.
- Testing and debugging become easier: operators can init a cycle for a specific phase, pre-populate artifacts, and run the phase agent in isolation.
- Predicates 023/024/025 provide executable regression tests for this behavior.

### Negative

- The phase-name↔cycle-state-name mapping in `init-standalone-cycle.sh` is a second authoritative source that must stay in sync with `cycle-state.sh`. If a new phase is added to the registry with a different internal name, both files need updating.
- `init-standalone-cycle.sh --phase build` provisions a real git worktree; if the working directory is not a git repo (e.g., CI test environments), worktree provisioning will warn and continue without a worktree.

---

## Schema Reference

Both utilities read their phase I/O declarations from `docs/architecture/phase-registry.json`:

```
phases[N].inputs.files[]        — list of required input file paths (with {cycle} template)
phases[N].inputs.state_fields[] — list of state.json fields the phase reads
phases[N].role                  — the agent role (active_agent) for cycle-state.json
```

Registry is resolved in this order: `$EVOLVE_PROJECT_ROOT/docs/architecture/phase-registry.json`, then git toplevel, then `$EVOLVE_PLUGIN_ROOT/docs/architecture/phase-registry.json`. This ensures the tools work correctly when `EVOLVE_PROJECT_ROOT` is overridden to a test temp dir.

---

## Alternatives Considered

1. **Check inputs inside each phase persona** — rejected: distributes the checking logic across 11 persona files, making it hard to keep in sync with the registry as I/O contracts evolve.
2. **Extend `phase-gate.sh` to validate inputs** — rejected: `phase-gate.sh` runs post-init (after cycle-state.json exists), but `init-standalone-cycle.sh` needs to run before any gate is invoked. They operate at different lifecycle points.
3. **Use `cycle-state.sh init` inside `init-standalone-cycle.sh`** — rejected: `cycle-state.sh init` always starts at `phase=calibrate`. Standalone init needs to set an arbitrary starting phase. Writing cycle-state.json directly (via jq + atomic mv) avoids this constraint.

---

## Implementation References

- `scripts/utility/init-standalone-cycle.sh` — bootstrap utility
- `scripts/utility/check-phase-inputs.sh` — input verification utility
- `docs/architecture/phase-registry.json` — I/O contract declarations (ADR-4)
- `acs/cycle-58/023-check-phase-inputs-detects-missing.sh` — predicate verifying check-phase-inputs behavior
- `acs/cycle-58/024-scout-runs-standalone.sh` — predicate verifying init for scout phase
- `acs/cycle-58/025-audit-standalone-builder-artifacts.sh` — predicate verifying audit standalone flow
