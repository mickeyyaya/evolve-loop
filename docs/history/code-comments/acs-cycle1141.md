# Comment history: `acs/cycle1141`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1141/predicates_test.go:3` — above `package cycle1141`

```text
// Package cycle1141 materialises the cycle-1141 acceptance criteria for the
// three fleet-scoped SSOT/gate tasks pinned to this lane:
//
//   - artifact-name-ssot-retro-backfill      → predicates 001-003
//   - required-roles-ssot                    → predicates 004-005
//   - cycle-docs-floor-architecture-changes  → predicates 006-008
//
// Predicate strategy. Tasks 1 and 2 are SSOT *refactors*: deriving a filename
// from phasecontract.For(phase).ArtifactName produces the SAME string the frozen
// literal produced, so no single behavioural assertion can distinguish "derived"
// from "re-typed". The honest materialisation is therefore a PAIR per caller:
//
//	(a) a behavioural assertion that EXECUTES the caller and compares its output
//	    against the registry value computed at test time (so if the registry ever
//	    moves, the caller must move with it), and
//	(b) an anti-freeze assertion that the raw literal is ABSENT from the caller's
//	    source.
//
// (b) alone would be the banned degenerate form — but it is inverted here: it
// demands the magic string be REMOVED, which cannot be satisfied by pasting text
// in, and can only be satisfied while (a) still passes by actually deriving the
// value. The load-bearing half is always the executed one.
//
// Task 3 is new machinery, so its predicates are pure behaviour: a table-driven
// exercise of the gate's decision function plus a policy round-trip proving the
// stage is config-injected and a wiring proof that the gate is not inert.
//
// Root resolution: acsassert.RepoRoot(t) is the worktree (where Builder writes,
// per worktree isolation). Source-path assertions resolve under it; behavioural
// assertions link the packages directly, so they exercise the worktree's code by
// construction.
```
