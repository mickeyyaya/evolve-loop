# Investigation: cycles must ship when audit passes — optional-phase mortality + ship-contract noise

**Date:** 2026-06-11 · **Trigger:** coverage/adversarial batch (cycles 280–284) — 3 of 4 completed cycles FAILed before audit ever ran, discarding finished build work. Operator request: "the code or changes must ship if verdict/audit returns PASS."

## Request

Guarantee the pipeline property: **work that would pass audit reaches audit, and audit-PASS reliably ships.** Investigate why cycles in this batch died pre-audit and what stands between audit-PASS and ship.

## Evidence (batch ledger)

| Cycle | Died at | Cause | Build work fate |
|---|---|---|---|
| 280 | test-amplification (post-build) | inserted phase `Worktree=""` → tree-diff guard | destroyed by abort-cleanup |
| 281 | — PASS, shipped `02b778ef` | ship contract blocked 3×, breaker demoted enforce→advisory, then shipped | shipped |
| 282 | test-amplification | same as 280 (binary predated the 281 fix) | destroyed; salvaged to `.evolve/operator-salvage/cycle-282-main-tree/` |
| 283 | test-amplification | **double `ExitArtifactTimeout` (81) → cycle-level abort; audit/ship never ran** | preserved (281 fix) in `.evolve/worktrees/cycle-283` — 8 files, unshipped |

The 280/282 class (worktree provisioning + abort destruction) was fixed in cycle 281's ship and verified working in 283. The remaining killers are Findings A and B below.

## Finding A — optional-phase failure aborts the cycle pre-audit (the 283 killer)

**The intent is already documented in the code but was never implemented.**

- `go/internal/core/errors.go:35-37` (comment on `ErrArtifactTimeout`): *"an OPTIONAL phase that hits this degrades to WARN+advance instead of aborting the whole cycle (Workstream D — cycle-120 build-planner)."*
- Reality — the exhaustion path consults nothing about optionality:
  - `go/internal/core/orchestrator.go:1991` — retries exhausted (`EVOLVE_PHASE_MAX_ATTEMPTS`, default 2) or non-transient error;
  - backfill attempt (default-on `EVOLVE_BACKFILL_ENABLED`) tries to reconstruct the artifact from `stdout.clean.txt`; on miss →
  - `orchestrator.go:2062` — `return result, wrapCycleLevelError(next, phaseErr)`;
  - `wrapCycleLevelError` (`orchestrator.go:39-46`) checks only the three batch-fatal sentinels (phase-gate / ledger-chain / lock) — **never `spec.Optional`**;
  - `cmd_loop.go:356,372` — `ErrCycleLevelFailure` → seal cycle as FAIL, `continue` to next cycle. Audit and ship never run.
- Optionality metadata exists and is queryable at the abort site: `phaseinventory.PhaseEntry.Optional` via `o.catalog.Get(string(next)).Optional`. It is used by routing (`transitionLegal` `orchestrator.go:2974`, `router.shouldRun`) but not by failure handling.
- No existing dial covers this (checked: `EVOLVE_PHASE_RECOVERY` is orthogonal pane-recovery; skip-advance in `enforceNext` `orchestrator.go:2873-2928` applies to routing *decisions*, not failures).

**Net effect:** any advisor-inserted enrichment phase (test-amplification, adversarial-review, mutation-gate…) that exhausts its retries kills the whole cycle, discarding completed spine work — the opposite of the phases' purpose. With one CLI quota-walled (codex exit=85 all batch), claude alone routinely exceeded the artifact window on heavy cycles, making this near-deterministic.

### Fix A (proposed, minimal)

At the exhaustion site (`orchestrator.go` ~1991-2062), before `wrapCycleLevelError`:

```
if (errors.Is(phaseErr, ErrArtifactTimeout) || isTransientBridgeError(phaseErr))
   && spec, ok := o.catalog.Get(string(next)); ok && spec.Optional
   && !inShipFloor(next) && !inMandatorySet(next) {
    // record synthesized WARN outcome (reason: optional_phase_infra_skip),
    // recordFailureLearning as today, loud stderr WARN,
    // advance to next spine phase (continue) instead of returning the error
}
```

Constraints:
- Only **infra-shaped** failures qualify (timeout / transient bridge). Integrity failures (tree-diff guard, gate, ledger) stay cycle-fatal regardless of optionality.
- Ship floor (`tdd, build, audit` + mandatory set) phases never qualify — the integrity floor `ship ⇒ build ∧ audit ∧ (tdd unless trivial)` is untouched.
- Acceptance tests: (1) replay cycle-283's shape — optional insert double-timeouts post-build → cycle proceeds to audit; (2) `build` double-timeout → still cycle-fatal; (3) optional phase tree-diff violation → still cycle-fatal; (4) the WARN outcome appears in phase timings + failure-learning.

## Finding B — ship's deliverable contract is structurally unsatisfiable (the breaker-noise on audit-PASS⇒ship)

Cycle 281 (the one PASS) only shipped because the contract-gate circuit breaker demoted itself:

- There is **no explicit `ship` entry** in `go/internal/phasecontract/contract_registry.go` (build/scout/tdd/audit/intent/triage/router are explicit).
- Ship therefore receives the spec-derived default: `contract_from_spec.go:43-48` falls back to `spec.Name + "-report.md"` → `ship-report.md`.
- But ship is a **deterministic native executor** (no LLM agent writes markdown), so the enforce-stage gate blocks 3× ("missing_artifact … write it to exactly …/ship-report.md"), correction re-dispatches are meaningless for a native phase, and the breaker opens (enforce→advisory) every shipping cycle.
- Known backlog item ("ship-report contract mismatch", logged 2026-06-08) — this investigation pins the mechanism.

### Fix B (proposed — pick ONE home, single-source)

Preferred: **the contract synthesizer must not invent markdown contracts for non-LLM phases** — in `contractFromSpec`, return no contract (or a no-op contract) when `spec.Kind != "llm"` unless the spec explicitly declares `outputs.files`. Alternative (heavier): the ship executor writes a real `ship-report.md` (it has commit SHA, push result, repair-ladder actions) — gives operators a per-cycle ship record but adds an artifact to maintain.

Acceptance: a shipping cycle runs end-to-end with **zero** contract-gate BLOCK lines and no breaker demotion; audit-PASS → ship completes on the enforce setting.

## Secondary observations (not blocking, feed routing/triage)

1. **Scope-sizing:** triage packed 3 multi-package coverage tasks per cycle; with codex benched, claude exceeded artifact windows (build needed 2 attempts; test-amplification needed >2). Failure-learning records exist; consider advisor weighting of remaining-CLI capacity, or artifact-window scaling by diff size.
2. **Codex quota-wall (exit 85 `ExitUnknownPrompt`, "usage limit … 6:11 AM")** burned a boot per dispatch all batch. The bridge classified + escalated correctly; consider a per-batch "bench this CLI after N consecutive walls" memo so dispatch chains start at the fallback.
3. Dead spine gate (`handoff-*.json`) WARN noise on every transition — known cosmetic backlog item, unchanged.

## Status

Investigation only — no code changed. Fix A and Fix B are ready to implement (TDD per acceptance tests above; both are small, seam-local diffs). Cycle-283's unshipped swarm coverage suite (8 files) remains salvageable in its preserved worktree; cycle-282's salvage sits in `.evolve/operator-salvage/cycle-282-main-tree/`.
