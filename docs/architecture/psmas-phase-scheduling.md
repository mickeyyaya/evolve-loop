# PSMAS Phase Scheduling (Predictive Skip via Multi-Agent System)

> **Status:** Foundation shipped v10.17.0 (cycle 98), **opt-in default-off**
> **Audience:** Operators experimenting with cycle-level token savings; persona authors
> **Source:** `agents/evolve-triage.md:125-205`, `agents/evolve-orchestrator.md:111-130`, `scripts/lifecycle/phase-gate.sh:447-508,1367-1373`

## TL;DR

When `EVOLVE_PSMAS_SKIP=1` is set, the Triage agent emits a `phase_skip[]` recommendation in its decision artifact, and the Orchestrator honors the recommendation by skipping those phases for the current cycle. Skipped phases get a `kind:phase_skipped` ledger entry so `--resume` and audit-binding both see the skip as deliberate, not as a missing-artifact failure.

**Behavior contract:**

| Triage `cycle_size_estimate` | `phase_skip[]` emitted | Condition |
|---|---|---|
| `trivial` | `["tdd-engineer", "retrospective"]` | always |
| `small` | `["retrospective"]` | only when previous cycle's audit verdict was PASS |
| `medium` | `[]` | always |
| `large` | `[]` | always (would be blocked at gate; should not reach this path) |

**Default-off rationale:** the foundation shipped without A/B verification. Until token-savings are measured against five historical cycle reruns (deferred to a future cycle per `knowledge-base/research/v10-17-0-release-debrief.md` §2), the feature is operator-flag-gated. Behavior is byte-identical to v10.10 when `EVOLVE_PSMAS_SKIP` is unset.

## Why this exists

The Reflexion-style retrospective phase (`agents/evolve-retrospective.md`) is expensive: ~$0.30-0.50 per cycle, ~2 K-4 K tokens of output. For cycles whose verdict is PASS and whose scope is `trivial` or `small`, the retrospective rarely produces actionable lessons — the cycle worked, there's nothing to extract. Similarly, the TDD-engineer phase writes failing tests *before* the Builder writes any production code; for a `trivial` cycle (e.g., a one-line config tweak with no behavioral change), the RED-test ritual adds cost without surfacing new failure modes.

PSMAS — Predictive Skip via Multi-Agent System — gives the Triage agent (which has already classified cycle size and read recent verdict history) the authority to recommend phase skips that the Orchestrator honors at gate level. This is a structural shortcut, not a prompt-level optimization: phases are skipped *at the phase-gate*, leaving a ledger entry that `--resume` and audit-binding both recognize.

## Implementation contract

### Triage side — emit recommendation

`agents/evolve-triage.md:125-136`:

```markdown
### 3a. PSMAS phase_skip[] recommendation (P3, opt-in)

When `EVOLVE_PSMAS_SKIP=1`, emit a `phase_skip[]` field in `triage-decision.md` recommending
phases the orchestrator may skip to save tokens. The mapping is fixed:

| cycle_size_estimate | phase_skip[]                          | Condition           |
| trivial             | ["tdd-engineer", "retrospective"]      | always              |
| small               | ["retrospective"]                      | PASS baseline only  |
| medium              | []                                     | always              |
| large               | []                                     | always (blocked)    |
```

The field is **always emitted** (even as `[]` when the flag is off) so downstream tooling has a consistent schema. Only its **non-empty content** activates a skip path.

### Orchestrator side — honor recommendation

`agents/evolve-orchestrator.md:111-130`:

For each phase name in `triage.phase_skip[]`, the orchestrator:

1. Emits a `kind:phase_skipped` ledger entry **before** advancing past that phase:
   ```json
   {"kind":"phase_skipped","cycle":<N>,"role":"<phase>","reason":"triage_phase_skip","psmas_flag":1}
   ```
2. Records the phase in `cycle-state.json:completed_phases[]` so `--resume` treats it as already-done and does not re-execute.
3. Advances directly to the next eligible phase in the canonical sequence.

### Precedence rule (multi-writer safety)

Two systems can request skips:

- **`adaptiveFailureDecision.skip_phases[]`** — from `scripts/failure/failure-adapter.sh` (deterministic, derived from `state.json:failedApproaches[]`)
- **`triage.phase_skip[]`** — from Triage (judgment-based, derived from cycle size + verdict baseline)

The Triage `phase_skip[]` is **additive**: it may request skips the adapter did not request, but it **cannot override** a non-skip from the adapter. Merge rule:

```
effective_skips = union(adapter.skip_phases[], triage.phase_skip[])
```

Applied only when `EVOLVE_PSMAS_SKIP=1`. When the flag is unset or `0`, the orchestrator uses **only** `adapter.skip_phases[]` — behavior is identical to pre-P3.

### Gate enforcement

`scripts/lifecycle/phase-gate.sh:447-508` adds two skip-aware gates:

| Gate | Skip target | Required precondition |
|---|---|---|
| `triage-to-build` | tdd-engineer | `EVOLVE_PSMAS_SKIP=1` AND ledger contains `kind:phase_skipped, role:tdd-engineer` for this cycle |
| `audit-to-complete` | retrospective | `EVOLVE_PSMAS_SKIP=1` AND ledger contains `kind:phase_skipped, role:retrospective` for this cycle |

Both gates fail loudly (exit non-zero) when the flag is set without a matching ledger entry — operators can't accidentally skip a phase by setting the env-var alone. The ledger entry IS the skip; the flag merely permits it.

## Resume safety

A `kind:phase_skipped` entry for phase X in cycle N means `--resume` of cycle N MUST treat X as already completed. Without this discipline, a session interruption mid-cycle would re-execute the skipped phase on resume, defeating the savings.

Implementation: `cycle-state.json:completed_phases[]` is populated when the ledger entry is written. The resume path (`scripts/lifecycle/resume-cycle.sh`) reads `completed_phases[]` before deciding which phase to dispatch next.

## Audit-binding considerations

The Auditor verifies that all expected phase outputs exist (`scout-report.md`, `build-report.md`, etc.). A skipped phase's output WILL be missing. The auditor reads the ledger BEFORE checking artifact presence and treats `kind:phase_skipped` entries as legitimate output substitutes:

- Skipping `tdd-engineer` → `tdd-report.md` absence is acceptable; ledger entry is the substitute.
- Skipping `retrospective` → `retrospective-report.md` absence is acceptable; ledger entry is the substitute.

The cycle's `acs-verdict.json:red_count == 0` predicate still gates ship. If the Builder's predicates fail in a trivial-cycle PSMAS path, the cycle FAILs at audit just like a non-PSMAS cycle would.

## Measured impact (foundation cycle)

Cycle 98 shipped the foundation but did NOT execute the A/B verification. Expected savings (from `agents/evolve-retrospective.md` and `agents/evolve-tdd-engineer.md` per-phase cost estimates):

| Phase skipped | Approximate savings per cycle |
|---|---|
| `retrospective` (PASS cycles) | ~$0.30 + ~3-5 min wall time |
| `tdd-engineer` (trivial cycles) | ~$0.50 + ~2-4 min wall time |

For a batch with 5 trivial PASS cycles, conservatively 5 × ($0.30 + $0.50) = $4.00 saved (~20-30% of typical batch cost). This estimate is **not yet validated** — the A/B verification (rerun 5 historical cycles under `EVOLVE_PSMAS_SKIP=1`, measure delta) is queued for a future cycle.

## Anti-patterns and guards

| Anti-pattern | Why bad | Guard |
|---|---|---|
| Skip `tdd-engineer` on a `medium` or `large` cycle | Real behavioral changes need RED tests | Triage maps `medium`/`large` to `phase_skip[]=[]` always |
| Skip `retrospective` after a FAIL | Lessons would be lost | Triage maps `small` to `retrospective` only when previous verdict is PASS |
| Skip a phase via env-var alone (no ledger entry) | Audit can't distinguish skip from missing | Phase-gates require BOTH `EVOLVE_PSMAS_SKIP=1` AND a matching ledger entry |
| Replay-skip on `--resume` re-execution | Wasted work | `completed_phases[]` tracks the skip; resume reads this |

## Operator controls

| Need | How |
|---|---|
| Enable PSMAS for a session | `EVOLVE_PSMAS_SKIP=1 /evolve-loop ...` |
| Check what was skipped | `grep '"kind":"phase_skipped"' .evolve/ledger.jsonl \| tail` |
| Disable for one cycle only | `EVOLVE_PSMAS_SKIP=0 /evolve-loop ...` for that invocation |
| Force a phase to run despite skip recommendation | Triage emits the recommendation, but operator can edit `triage-decision.md` to clear `phase_skip[]` before Build phase starts (interactive use only) |

## Roadmap

Per `knowledge-base/research/v10-17-0-release-debrief.md`:

1. **A/B verification** — rerun cycles 80, 82, 85, 87, 89 (all PASS, varied sizes) under `EVOLVE_PSMAS_SKIP=1`, measure token + cost delta. Target: ≥20% reduction on the skipped-phase subset to justify default-on.
2. **Mutation testing extension** — add `psmas_safe: bool` field to lesson YAMLs. When a lesson's failure-mode could only be caught by the retrospective phase, mark unsafe; Triage refuses to skip retrospective on cycles that touch lesson-relevant files.
3. **Default-on flip** — after ≥10 cycles of green A/B verification + operator confirmation, flip default to `EVOLVE_PSMAS_SKIP=1`. Foundation already supports backward-compat: behavior is byte-identical when flag is `0`.

## See also

- `docs/architecture/phase-architecture.md` — canonical phase sequence (PSMAS is an opt-in shortcut within this sequence)
- `docs/architecture/sequential-write-discipline.md` — companion: parallel vs sequential phase execution
- `docs/architecture/control-flags.md` — full env-var control surface
- `docs/architecture/token-economics-2026.md` §P3 — original P3 plan
- `knowledge-base/research/v10-17-0-release-debrief.md` — release retrospective + A/B verification deferral
- ACS predicates that lock this contract:
  - `acs/regression-suite/cycle-98/001-triage-schema-documents-phase-skip.sh`
  - `acs/regression-suite/cycle-98/002-orchestrator-honors-phase-skip-with-precedence.sh`
  - `acs/regression-suite/cycle-98/003-phase-gate-accepts-forward-skip-under-flag.sh`
  - `acs/regression-suite/cycle-98/004-phase-skipped-implies-no-role-execution.sh`
  - `acs/regression-suite/cycle-98/005-default-off-no-phase-skipped-baseline.sh`
