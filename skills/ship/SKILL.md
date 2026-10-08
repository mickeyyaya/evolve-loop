---
name: ship
description: Use after audit returns Verdict PASS. Atomic git commit + ledger update; only the `evolve release` path makes a tag. Single-writer; cannot fan-out.
---

# ship

> Sprint 3 composable skill. Wraps the Ship phase. The atomic commit at the end of every successful cycle.

## When to invoke

- After `audit` returns Verdict PASS (or WARN with explicit override)
- Cycle is in `audit` phase, transitioning to `ship`

## When NOT to invoke

- Audit verdict is FAIL or ABORT
- The current tree-state SHA differs from what the auditor saw (cycle-binding violation)
- Ship class is unclear (use `--class cycle`, `--class manual`, or `--class release` explicitly)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Verify audit verdict = PASS (or WARN without `workflow.strict_audit`). `evolve ship` checks the audit binding itself (`go/internal/phases/ship/audit.go`) | Binding check passes |
| 2 | Run `evolve release-consistency <version>` for consistency check | Markers consistent |
| 3 | Run `evolve ship --class cycle "<commit message>"` | Atomic commit created. Only the `evolve release` path makes a tag |
| 4 | Verify ledger entry added | `kind: "ship"` with cycle binding |

## Single-writer invariant

Ship is ATOMIC by design — even if other phases fan out, Ship cannot. There is one git commit per cycle. The ship lock serializes concurrent ship attempts: `evolve ship` holds a flock on `.evolve/ship.lock` while it commits, merges and pushes (`go/internal/phases/ship/gitops.go`).

## Cycle-binding (v8.13.0+)

`evolve ship` (`go/internal/phases/ship/audit.go`) refuses to ship if the current tree-state SHA differs from the SHA captured at audit time (in the auditor's ledger entry). Prevents "audit cycle 50, ship cycle 51" exploits. This guarantee is preserved through Sprint 3's tri-layer refactor.

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/orchestrator.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `ship` (control archetype, mandatory) |
| Persona | `agents/evolve-orchestrator.md` |
| Profile | `.evolve/profiles/orchestrator.json` — CLI `codex-tmux`, tier `balanced`, single-writer |
| Inputs | `audit-report.md` |
<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evo:ship` (user-driven)
- `loop` macro after `/evo:audit`

## Reference

- `evolve ship` (`go/internal/phases/ship/`): atomic commit + push; only the `evolve release` path makes a tag
- `evolve release` (`go/internal/releasepipeline/`): full release lifecycle for `publish` operations
- [docs/guides/publishing-releases.md](../../docs/guides/publishing-releases.md) (vocabulary: push / tag / release / propagate / publish / ship)
