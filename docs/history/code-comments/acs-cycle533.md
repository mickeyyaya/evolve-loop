# Comment history: `acs/cycle533`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle533/predicates_test.go:3` — above `package cycle533`

```text
// Package cycle533 materialises the cycle-533 acceptance criteria.
//
// TASK BINDING & CROSS-ARTIFACT CONFLICT (resolved here, documented in
// test-report.md):
//
//   - triage-report.md `## top_n` names `cache-stable-prompt-prefixes` (a
//     fleet_scope inbox item for a SIBLING lane) and defers scout's two bugfix
//     tasks. But that task has NO materialised eval, NO fault-localization, and
//     the LOCKED phase-plan actually driving dispatch (fault-localization →
//     bug-reproduction → tdd → build → adversarial-review → coverage-gate,
//     coverage-gate justified "diff_loc >= 50 for a guard/leak fix") is the
//     tree-diff-guard BUGFIX. fault-localization (this cycle's phase immediately
//     before tdd) explicitly resolved the conflict toward the bug per
//     follow_approved_plan_no_relitigate. These predicates therefore bind to the
//     work the executing plan committed — the bugfix — which is what
//     bug-reproduction/tdd/build all target.
//
//   - Scout proposed TWO fix tasks (both evals materialised in the workspace):
//     Task 1  fix-role-gate-worktree-write-predicate      (go/internal/guards/role.go)
//     Task 2  fix-treediff-leak-recovery-catalog-predicate (go/internal/core/cyclerun_review.go)
//     CONTROL-PLANE FINDING (new this cycle, missed by scout/fault-localization):
//     `/go/internal/guards/` is a PROTECTED integrity surface
//     (guards.protectedSurfaceFragments / ADR-0064) — an autonomous `--class
//     cycle` run STRUCTURALLY cannot edit role.go (the role gate denies it, and
//     Builder would hit the same wall). Task 1 is therefore dispositioned
//     manual+checklist (operator-gated `evolve ship --class manual`), NOT a
//     predicate — a predicate requiring a control-plane edit would set Builder
//     up to fail the integrity boundary. Per scout hypothesis #2, Task 2 is also
//     the MORE load-bearing half (the only leak backstop for non-Claude drivers,
//     which bypass the role gate entirely), so the cycle-shippable fix closes the
//     directly-recurring failure mode.
//
// These predicates bind ONLY to the triage/plan-committed, cycle-implementable
// work: Task 2 (R9.3 — predicates for committed work only). Task 1 rides in
// test-report.md as a manual+checklist item.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate RUNS the
// system-under-test as a real subprocess (`go test -tags integration` against
// the orchestrator's production RunCycle → recoverBuildLeak → tree-diff guard
// path on a real git repo) and asserts on its exit code — never a
// "source file contains text X" grep. The behavioural test
// TestGuardRecoversCatalogWritesSourcePhaseLeak is RED on the current tree (the
// scout-leak cycle aborts with "tree-diff guard: phase \"scout\" wrote to the
// main tree"); Builder makes it green by swapping the bare `WorktreePhase(next)`
// at cyclerun_review.go:263 for the catalog-aware `cr.o.worktreePhase(next)`.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive:  C533_001 — a catalog source-writer phase's leak is RECOVERED.
//   - Negative:  C533_002 — a non-source phase's leak still hard-ABORTS (the
//     anti-over-broadening pin; kills the "always recover" fake).
//   - Regression: C533_003 — the built-in tdd/build leak paths + guard
//     classification suite stay green (the fix must not disturb them).
```
