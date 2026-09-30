# Comment history: `acs/cycle549`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle549/predicates_test.go:3` — above `package cycle549`

```text
// Package cycle549 materialises the cycle-549 acceptance criteria for this
// fleet lane's (cycle-21f9f7ae-549, fleet_scope
// cli-command-layer-test-coverage-worktree-swarm) sole `## top_n` task per
// triage-report.md:
//
//	cli-command-layer-test-coverage — raise cmd/evolve (esp.
//	runWorktreeCreate/List/Cleanup, runSwarmReap), cmd/evolve/cmdutil, and
//	internal/commitgate (incl. non-Go lane fixtures/removal) to >=80% tagged
//	coverage with fixture-based success+error-path tests.
//
// NOTE: the cycle-549 scout-report.md (this same workspace) ALSO proposed two
// OTHER tasks (memo-activation-overlay-layering,
// unroutable-phase-fail-loudly) as its own "## Selected Tasks" — but
// triage-report.md explicitly scoped THIS lane to
// cli-command-layer-test-coverage-worktree-swarm only and left those two
// tasks for a sibling lane to triage/build independently (see
// triage-report.md's "Fleet-lane scope" section). Per the AC-Materialization
// Contract (R9.3: "predicates bind ONLY to triage-committed work"), this
// package predicates ONLY the triage-committed item above.
//
// Predicate strategy: this is a COVERAGE-COMPLETION task, not a new-behavior
// task — the production code under test (cmd_worktree.go, cmd_swarm.go,
// cmdutil.go, lanes.go) already works correctly; the gap was test coverage,
// not functionality. So these predicates are BEHAVIORAL in a different sense
// than a RED/GREEN feature: each drives `go test -cover` as a subprocess over
// the real package and asserts (a) the new fixture-based tests actually ran
// (non-degenerate — closes the cycle-85 "no tests to run" trap) and (b) the
// reported coverage percentage clears the committed bar. This is exactly the
// "pre-existing GREEN" disposition test-report.md documents (Step 4's RED
// verification rules) — never a source grep.
//
// In-package tests authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_worktree_test.go
//	cmd/evolve/cmd_swarm_test.go
//	cmd/evolve/cmdutil/filterenv_promptsloader_test.go
//	internal/commitgate/lanes_test.go
//
// The Builder's role for this task (if any remains) is limited to fixing any
// GENUINE bug these tests surface; per Step 4 they are pre-existing GREEN
// (the production code was already correct), so no Builder code change is
// expected — Builder should confirm the four coverage bars below still hold
// after any concurrent change and must not modify these test files.
```
