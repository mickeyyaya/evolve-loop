# Comment history: `acs/cycle1474`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1474/predicates_test.go:3` — above `package cycle1474`

```text
// Package cycle1474 materialises the cycle-1474 acceptance criteria for the two
// fleet-scoped pipeline tasks pinned to this lane:
//
//   - worktree-retry-diagnostic-integrity   → the shared `git worktree add`
//     retry must keep BOTH the initiating and the terminal failure, and must
//     announce contention BEFORE it pays the backoff.
//   - worktree-provisioning-cause-fingerprint → a failed worktree provision must
//     put its git cause into the cycle's failure record, so the recorded
//     identity names the provisioning failure instead of the downstream
//     source-phase refusal it caused.
//
// Predicate strategy — every predicate DRIVES the production seam in-process and
// asserts on returned values or emitted artifacts; none greps source (the
// cycle-85 degenerate-predicate ban):
//
//   - 001/002 call gitexec.Git.AddWorktreeWithRetry directly with a scripted
//     runner and assert on what it returns / the order it calls back.
//   - 003/004 run the REAL core.Orchestrator.RunCycle with a provisioner that
//     fails, then read the cycle's own workspace and the REAL
//     core.AssembleFailureDigest — the same assembler production uses.
//   - 005 is the negative axis: a clean provision must fabricate no failure.
//
// No `go test` subprocess, no whole-package sweep, no wall-clock bound, no
// literal PID: all five are in-process and deterministic.
```
