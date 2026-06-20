---
name: tdd
description: Use when the plan-review verdict is PROCEED (or plan-review is disabled) and code has not yet been written. Writes RED tests first, defining the contract Builder must satisfy. The mandatory hop before any implementation.
---

# tdd

> Sprint 3 composable skill. Wraps the existing TDD-engineer phase with addyosmani-style structure. Mandatory hop before `build`.

## When to invoke

- After `plan-review` returns PROCEED, OR after Scout if plan-review disabled
- Before any new feature implementation
- Before any bug fix (write a regression test first)

## When NOT to invoke

- Eval-only cycles (no implementation needed)
- Pure documentation cycles
- Refactor cycles where existing tests provide coverage (verify first)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read scout-report.md and plan-review.md | Inputs verified |
| 2 | Write RED tests (must fail on current code) | Tests exist; running them fails |
| 3 | Document the test contract in `<workspace>/tdd-contract.md` | Contract written |
| 4 | Append section to `<workspace>/team-context.md` | Bus updated for Builder |

## RED requirement

Tests MUST fail when run against the current codebase. A test that passes immediately is NOT a TDD contract — it does not exercise the new behavior. Builder will refuse to start without confirmed RED state.

## Handoffs (beyond the contract artifact)

- `<workspace>/tdd-contract.md` with sections: `## Tests Written`, `## RED Verification`, `## Contract for Builder`
- New/modified test files in the repo (Builder reads these)
- `<workspace>/team-context.md` has populated `## TDD Contract` section

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/tdd-engineer.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `tdd` (plan archetype, optional, gated by `workflow.phase_enables.tdd=on`) |
| Persona | `agents/evolve-tdd-engineer.md` |
| Profile | `.evolve/profiles/tdd-engineer.json` — CLI `claude-tmux`, tier `balanced`, single-writer |
| Inputs | `scout-report.md` · `plan-review-report.md` |
| Artifact | `test-report.md` (cycle workspace) |

## Output contract

`test-report.md` must declare:

- `## AC-Materialization` (also accepted: `## Acceptance`, `## Coverage Map`)
- `## RED Run Output` (also accepted: `## RED Tests`, `## Test Files Written`)

<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evolve-loop:tdd`
- `loop` macro after plan-review (or after discover when plan-review off)

## Reference

- `agents/evolve-tdd-engineer.md` (persona definition)
- `.evolve/profiles/tdd-engineer.json`
- CLAUDE.md "Evolve Loop Task Priority" section
