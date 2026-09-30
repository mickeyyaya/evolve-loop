# Comment history: `acs/cycle1262`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1262/predicates_test.go:3` — above `package cycle1262`

```text
// Package cycle1262 materialises the cycle-1262 acceptance criteria for the
// three tasks this fleet lane committed in triage `## top_n`:
//
//	worktree-path-propagation-fallback     (S — predicates 001-002)
//	config-single-authority-sweep-alias    (S — predicates 003-004)
//	llmroute-dispatch-unification          (M — predicate  005)
//
// The two `## deferred` items (`config-single-authority-sweep-gatestage`,
// `worktree-path-propagation-fallback-fullaudit`) and the one `## dropped` item
// (`egps-regression-tia-shadow-wiring`, verified already landed at
// audit.go:699-703) carry ZERO predicates, per the R9.3 floor-binding rule: a
// predicate may only gate work this cycle committed to.
//
// # Task 1 — the silent worktree fallback
//
// `subagent.Run` (run.go:336-338) does
// `worktreePath := req.WorktreePath; if worktreePath == "" { worktreePath = req.ProjectRoot }`
// and then exports that value as the adapter's `WORKTREE_PATH`. When an
// orchestrator forgets to propagate the lane worktree, the agent silently runs
// against the MAIN repo root — the exact shape that trips the tree-diff guard
// and kills a lane, with no signal anywhere saying the fallback fired. The
// committed fix is the cheap one the inbox item accepts: keep the fallback
// (nothing may break) but make it LOUD via the already-existing
// `RunResult.Warns` channel.
//
// # Task 2 — the antigravity→agy alias, three times
//
// `if cli == "antigravity" { cli = "agy" }` is copy-pasted at run.go:246,
// dispatchparallel.go:124 and validateprofile.go:137. Three copies of a naming
// authority is three places to drift. The fix centralises it as
// `detectcli.Canonical(cli string) string` — the package that already owns CLI
// identity — and calls it from all three sites.
//
// # Task 3 — dispatch-parallel's invented CLI
//
// `subagent` imports ZERO `llmroute` symbols; `dispatchparallel.go:120-122`
// resolves its CLI by regex-scraping the profile body and then falling back to
// the bare literal `"claude"`. Every sibling entry point (`Run`,
// `ValidateProfile`) resolves through the shared resolver and FAILS LOUDLY when
// nothing resolves. Predicate 005 pins the invariant both fix branches share:
// dispatch-parallel must never invent a CLI out of a hardcoded literal.
//
// # Predicate strategy
//
// Every predicate drives an EXPORTED production entry — `subagent.Run`,
// `subagent.ValidateProfile`, `subagent.DispatchParallel` — through its real
// seams and asserts on what that entry returned or on the value it handed a
// downstream collaborator. None greps production source (the cycle-85
// degenerate-predicate ban): a `FileContains` over run.go would pass the moment
// the implementer typed the magic string, whether or not the warn ever reaches
// a caller. None sweeps `/...`, shells a 40s+ suite, hardcodes a PID, runs bare
// `git`, or spawns an un-reaped load generator (the flaky-shape bans). The one
// subprocess predicate (004) is scoped to a SINGLE named package whose measured
// wall-clock is 0.7s.
```
