# Comment history: `acs/cycle1035`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1035/predicates_test.go:3` — above `package cycle1035`

```text
// Package cycle1035 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `guard-phase-hook-inert` (inbox
// weight 0.89), scoped as scout task `rewire-or-retire-guard-phase-hook`.
//
// THE DEFECT (verified live on this worktree). `go/internal/guards/phase.go`
// (`Phase.Decide`) only returns a non-Allow decision when `in.ToolName ==
// "Agent"`. But `.claude/settings.json` wires `evolve guard phase` EXCLUSIVELY
// under the `"Bash"` PreToolUse matcher, so the hook is only ever invoked with
// ToolName=="Bash", takes the `ToolName != "Agent"` early-return, and Allows
// unconditionally. The guard is a WIRED NO-OP: its one real branch can never
// fire. Meanwhile five doc surfaces assert that `evolve guard phase` enforces
// phase transitions / denies in-process Agent — enforcement that does not
// happen.
//
// DISPOSITION-AGNOSTIC BY DESIGN. The ticket names TWO valid fix shapes and
// the Builder chooses one; these predicates MUST green on EITHER and stay RED
// on the current wired-no-op state:
//
//   - REWIRE  → `.claude/settings.json` gains a matcher that covers the Agent
//     (subagent-dispatch) tool, so `Phase.Decide`'s Agent branch can
//     actually fire.
//   - RETIRE  → the guard source is removed and `evolve guard phase` is no
//     longer wired, with docs re-pointed at the state machine
//     (`go/internal/core`) as the real enforcement point.
//
// The load-bearing test is 001: it reads the REAL consumed config artifact
// (`.claude/settings.json`) plus the guard source presence and asserts the
// guard is no longer a wired no-op — a genuine end-to-end wiring assertion, not
// a source-grep of a magic fix-string (the cycle-85 degenerate-predicate ban).
// 003 couples DOC truth to that same wiring determination: a doc may claim
// `evolve guard phase` enforces phase order ONLY when the wiring actually makes
// it fire.
//
// Roots: settings.json / phase.go / docs / the ADR are all Builder deliverables
// in this cycle's WORKTREE, so they are read under acsassert.RepoRoot(t) (the
// worktree). Their absence / un-fixed state is a FAILURE this cycle, not a skip.
```
