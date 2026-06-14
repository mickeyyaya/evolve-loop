---
name: build
description: Use after tdd has written RED tests and the contract is in team-context.md. Implements the minimum code to turn RED tests GREEN. Runs in a worktree with single-writer invariant.
---

# build

> Sprint 3 composable skill. Wraps the Builder phase. Builder is the **only** fan-out-incompatible phase — single worktree, single set of mutations.

## When to invoke

- After `tdd` has produced RED tests
- Cycle is in `tdd` or `discover` phase per cycle-state

## When NOT to invoke

- RED tests don't exist (TDD must run first)
- Cycle goal is research-only or planning-only

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read TDD contract from team-context.md | Contract loaded |
| 2 | Implement minimum code to turn RED → GREEN | Tests pass locally |
| 3 | Run regression suite (`legacy/scripts/utility/run-all-regression-tests.sh`) | No new regressions |
| 4 | Write `<workspace>/build-report.md` | Report present + fresh + token-bound |

## Single-writer invariant

Builder runs in a worktree with `EVOLVE_BUILDER_WORKTREE` set. Concurrent builders are STRUCTURALLY blocked because:
- The trust kernel binds the cycle via SHA256 of `git diff HEAD`
- Two concurrent writers would invalidate each other's tree-state SHA
- `phase-gate-precondition.sh` allows only one `active_agent` per cycle

This is **why Builder cannot fan-out** even though Scout and Auditor can.

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/builder.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `build` (build archetype, mandatory) |
| Persona | `agents/evolve-builder.md` |
| Profile | `.evolve/profiles/builder.json` — CLI `codex-tmux`, tier `balanced`, single-writer |
| Inputs | `scout-report.md` · `triage-report.md` |
| Artifact | `build-report.md` (cycle workspace) |

## Output contract

`build-report.md` must declare:

- `## Changes` (also accepted: `## Files Changed`, `## Files Modified`)

<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evolve-loop:build`
- `loop` macro after `/tdd`

## Reference

- `agents/evolve-builder.md`
- `.evolve/profiles/builder.json`
- `skills/loop/phase3-build.md` (legacy detailed workflow)
