---
description: Implement the minimum code to turn RED tests GREEN. Single-writer; runs in worktree.
---

# /build

Run the Builder phase against the current cycle. Implements the minimum code to turn the RED tests written by `/tdd` into GREEN. Builder is the **only fan-out-incompatible phase** because of the single-worktree, single-writer invariant.

## When to use

- After `/tdd` produces RED tests
- Cycle is in `tdd` or `discover` phase per `cycle-state.sh get phase`

## Execution

```bash
bash scripts/subagent-run.sh builder <cycle> <workspace>
```

The builder runs in an isolated worktree (`EVOLVE_BUILDER_WORKTREE` set by the runner). Concurrent builders are structurally blocked by `phase-gate-precondition.sh`.

## Why no fan-out?

The trust kernel binds the cycle via SHA256 of `git diff HEAD`. Two concurrent writers would invalidate each other's tree-state SHA. Build stays serial — the latency win comes from Scout/Audit/Retro fan-out, not Build.

## See also

- `skills/evolve-build/SKILL.md`
- `agents/evolve-builder.md`
- `.evolve/profiles/builder.json`
