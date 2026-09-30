# Comment history: `acs/cycle1498`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1498/predicates_test.go:3` — above `package cycle1498`

```text
// Package cycle1498 materialises the cycle-1498 acceptance criteria for the one
// fleet-scoped task pinned to this lane, `retire-consumed-fleet-alias`: retire
// the consumed fleet alias `pipeline-defect-pipeline-blocker` through the
// EXISTING reviewed/locked `evolve carryover apply-decisions` path rather than a
// prompt-side suppression rule or a hand edit of state.json.
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): predicates 001-003
// drive the REAL production caller — the `evolve` CLI entry point
// (`cmd/evolve` dispatch table → runCarryover → runCarryoverApplyDecisions →
// applyCarryoverDecisions) — against an on-disk state.json fixture, and assert on
// exit code plus the resulting on-disk bytes. No source grep carries any
// assertion. The binary under test is BUILT FROM THIS WORKTREE'S SOURCE in
// TestMain, never the pre-existing `go/evolve` artifact, so a regression the
// builder introduces in cmd_carryover.go turns these predicates RED instead of
// passing against a stale binary.
//
// Predicate 004 is the task's declared verifiableBy: the named regression test
// must exist in `go/cmd/evolve/cmd_carryover_test.go` and PASS. It asserts on the
// `--- PASS: <exact name>` line rather than the exit code, because `go test -run`
// with a pattern that matches nothing exits 0 — the vacuous-green trap.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no whole-package
// `go test` (004 is narrowed by an anchored `-run`), no wall-clock deadline, no
// literal PID; every subprocess is given absolute paths or an explicit cmd.Dir,
// never process cwd.
```

### `go/acs/cycle1498/predicates_test.go:324` — above `if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch",`

```text
// Tracking check (cycle-93): an untracked test file is dropped at ship.
```
