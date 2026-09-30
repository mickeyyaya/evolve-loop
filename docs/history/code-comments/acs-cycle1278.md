# Comment history: `acs/cycle1278`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1278/predicates_test.go:3` — above `package cycle1278`

```text
// Package cycle1278 materialises the cycle-1278 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	retro-fleet-stale-worktree-fallback → retroWorktree
//	(go/internal/phases/retro/retro.go:87-92) falls back to the workspace-owned
//	scratch cwd when req.Worktree is empty OR names a directory that does not
//	exist, matching the bridge guard's isDir() predicate
//	(driver_tmux_repl.go:123-126) exactly; and cs.ActiveWorktree is cleared at
//	fleet-lane teardown (go/internal/core/cyclerun.go ~L466-472) once the prune
//	succeeds, so a torn-down lane's path is never handed to the next dispatch.
//
// The deferred task (fix-changelog-false-closure-claim) and the seven dropped
// ids get ZERO predicates — R9.3 floor-binding: predicates bind only to
// triage-committed work.
//
// Predicate-quality note (cycle-85 ban). No predicate here greps source for a
// magic string. 001-003 RUN the acceptance tests in their own packages and
// require an explicit `--- PASS: <name>` line, so a `-run` pattern that matched
// nothing — a deleted or renamed test — cannot green vacuously. 004 takes no
// subprocess at all: it invokes the public bridge helper retro falls back to and
// asserts its return value against the guard's own predicate.
//
// Diversity axes: 001 is the happy path (the two shapes that are RED today);
// 002 is the semantic/regression axis (a fallback that fires unconditionally, or
// leaks outside fleet mode, is the opposite defect); 003 carries its own negative
// case (a PRESERVED worktree must NOT be cleared — resume reclaims the lane by
// that path); 004 is the edge axis on the fallback's own degenerate input.
//
// Flaky-shape compliance: every subprocess is ONE named package with an anchored
// `-run` (measured 0.4-0.5s each at RED, plus compile), `cmd.Dir` is set
// explicitly rather than inherited from the lane's cwd, and there is no
// wall-clock bound, no literal PID, and no `./...` sweep.
```
