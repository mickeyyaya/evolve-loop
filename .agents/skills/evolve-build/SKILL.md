---
name: evolve-build
description: Use after evolve-tdd has written RED tests and the contract is in team-context.md. Implements the minimum code to turn RED tests GREEN. Runs in a worktree with single-writer invariant.
---

# evolve-build

> Sprint 3 composable skill. Wraps the Builder phase. Builder is the **only** fan-out-incompatible phase — single worktree, single set of mutations.

## When to invoke

- After `evolve-tdd` has produced RED tests
- Cycle is in `tdd` or `discover` phase per cycle-state

## When NOT to invoke

- RED tests don't exist (TDD must run first)
- Cycle goal is research-only or planning-only

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read TDD contract from team-context.md | Contract loaded |
| 2 | Implement minimum code to turn RED → GREEN | Tests pass locally |
| 3 | Run regression suite (`scripts/run-all-regression-tests.sh`) | No new regressions |
| 4 | Write `<workspace>/build-report.md` | Report present + fresh + token-bound |

## Single-writer invariant

Builder runs in a worktree with `EVOLVE_BUILDER_WORKTREE` set. Concurrent builders are STRUCTURALLY blocked because:
- The trust kernel binds the cycle via SHA256 of `git diff HEAD`
- Two concurrent writers would invalidate each other's tree-state SHA
- `phase-gate-precondition.sh` allows only one `active_agent` per cycle

This is **why Builder cannot fan-out** even though Scout and Auditor can.

## Output contract

`<workspace>/build-report.md` with sections:
- `## Files Modified`
- `## Test Results` (must show RED → GREEN transition)
- `## Regression Status`
- `## Build Status` (PASS / FAIL)

## Composition

Invoked by:
- `/build` slash command
- `evolve-loop` macro after `/tdd`

## Reference

- `agents/evolve-builder.md`
- `.evolve/profiles/builder.json`
- `skills/evolve-loop/phase3-build.md` (legacy detailed workflow)
