# ADR-0004: Pure-Function Phases via phase-registry.json

**Status**: Accepted  
**Date**: 2026-05-15  
**Cycle**: 55 (Slice B, Cycle 1 of 4)  
**Related**: ADR-0001 (LLM router), ADR-0002 (capability matrix), ADR-0003 (native CLI invocation)

---

## Context

The evolve-loop phase order is currently encoded in three separate places:

1. **`agents/evolve-orchestrator.md`** — narrative Phase Loop section (human-readable prose)
2. **`scripts/lifecycle/phase-gate.sh`** — 13 gate functions enforcing phase transitions
3. **`scripts/dispatch/run-cycle.sh`** — hardcoded phase dispatch sequence

Adding, removing, or reordering phases requires coordinated edits across all three locations. There is no machine-readable source of truth for:
- The canonical phase order
- Per-phase I/O contracts (input files, output files, state fields read/written)
- Which phases are optional and what env var controls them
- Which roles are eligible for parallel fan-out

This fragmentation is the root cause of the v8.55 fan-out incident where `parallel_eligible` was specified in profile JSON but not cross-referenced against the orchestrator's narrative.

---

## Decision

Create `docs/architecture/phase-registry.json` as the **declarative phase order and I/O contract authority**.

In **cycle 55** (this ADR), the registry is **data-only**:
- A new helper `scripts/dispatch/list-phase-order.sh` reads it and emits phase names
- The orchestrator and run-cycle.sh still use their hardcoded sequences
- No behavior changes to any running cycle

In **cycle 56**, the orchestrator and run-cycle.sh will be updated to consume the registry for phase sequencing. The existing hardcoded sequences will be removed.

---

## Schema (v1)

```json
{
  "schema_version": 1,
  "phases": [
    {
      "name": "<phase-name>",
      "role": "<profile-key>",
      "optional": true | false,
      "enable_var": "<env-var-name> | null",
      "enable_var_inverted": true | false,
      "inputs": {
        "files": ["<path-template>"],
        "state_fields": ["<field-name>"]
      },
      "outputs": {
        "files": ["<path-template>"]
      },
      "gate_in": "<gate-function-name> | null",
      "gate_out": "<gate-function-name> | null",
      "parallel_eligible": true | false,
      "skippable_when_present": true | false
    }
  ]
}
```

**Field semantics**:

| Field | Meaning |
|-------|---------|
| `name` | Phase identifier (used in cycle-state.json, ledger, workspace paths) |
| `role` | Profile key (maps to `.evolve/profiles/<role>.json`) |
| `optional` | Phase may be skipped; see `enable_var` |
| `enable_var` | Env var that controls optionality. When `enable_var_inverted: true`, the phase is disabled when the var is set (e.g., `EVOLVE_TRIAGE_DISABLE=1`) |
| `gate_in` | Gate function called before entering the phase. `null` = not yet wired (v1 deferred) |
| `gate_out` | Gate function called after the phase completes. `null` = not yet wired |
| `parallel_eligible` | Whether fan-out dispatch is permitted for this phase |
| `skippable_when_present` | Whether the phase can be skipped if its output artifact already exists (resume mode) |

---

## Rationale

- **Phase order becomes data, not code.** Adding a new phase is an edit to one JSON file rather than three scripts.
- **Pure-function contract.** Each phase has explicit input and output declarations. The runtime can verify inputs exist before launching a phase and outputs exist after.
- **Single source of truth for `parallel_eligible`.** Pre-v10.3, this was duplicated between profile JSON and orchestrator narrative. The registry is authoritative.
- **Backward-compatible introduction.** v1 ships as a read-only data file; nothing breaks if it's missing (fallback to hardcoded order).

---

## Affects

| Component | Change | Cycle |
|-----------|--------|-------|
| `docs/architecture/phase-registry.json` | **NEW** — declarative phase order + I/O contracts | 55 (this ADR) |
| `scripts/dispatch/list-phase-order.sh` | **NEW** — reads registry, emits phase names | 55 (this ADR) |
| `agents/evolve-orchestrator.md` | Registry-driven Phase Loop rewrite | 56 |
| `scripts/lifecycle/phase-gate.sh` | `gate_run_by_name` dispatcher + tester gate functions | 56 |
| `scripts/dispatch/run-cycle.sh` | Read registry for phase sequence | 56 |
| Phase contracts (`docs/architecture/phase-contracts/<phase>.md`) | Per-phase contract documents | 57–58 |

---

## Notes: Backward-Compatibility Policy

1. **Registry absent**: `list-phase-order.sh` and (cycle 56+) the dispatcher fall back to the hardcoded 11-phase order. No disruption to existing deployments.

2. **`EVOLVE_USE_PHASE_REGISTRY=0`**: Forces the hardcoded fallback regardless of registry file presence. For operators who need to freeze phase order during migrations.

3. **Tester phase `gate_in`/`gate_out` = null (v1)**: The `gate_build_to_tester` and `gate_tester_to_audit` functions don't exist yet. Predicate 020 skips gate validation for null gate fields. These will be added in cycle 56 alongside the registry-driven dispatch wiring. Until then, the tester phase is invoked directly by the orchestrator without a gate-function wrapper.

4. **TDD phase `gate_out` = `gate_discover_to_build`**: The tdd phase terminates at the same gate as the discover/triage path. This is semantically accurate: tdd is the last pre-build step, and `gate_discover_to_build` guards entry to the build phase from any prior phase.

5. **Cycle 55 non-wiring guarantee**: `list-phase-order.sh` is installed but not invoked from any running dispatch script. It is tested standalone via predicate 021. This constraint is enforced by predicate 021's design (it tests the helper in isolation with fixture env vars, not by checking dispatcher invocations).
