# Comment history: `acs/cycle1029`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1029/predicates_test.go:3` — above `package cycle1029`

```text
// Package cycle1029 materialises the cycle-1029 acceptance criteria for this
// fleet lane's sole inbox item, loop-skill-goal-mandatory-prompt-before-act
// (triage top_n slug: loop-skill-goal-mandatory-prompt).
//
// The defect is a DOC/BINARY DIVERGENCE, not a binary defect. The Go binary
// already requires a goal (go/cmd/evolve/cmd_loop_args.go:151-156 → rc=10
// "a goal is required …", locked by dispatch_test.go's
// TestDispatch_LoopRoutesToRunLoop). But skills/loop/SKILL.md still presents
// the goal as OPTIONAL (`[goal]` in the argument-hint and Usage lines), its
// STRICT MODE section has no rule telling the handler to prompt-and-wait for a
// goal before dispatch, and its dispatcher-exit table maps rc=10 only to a
// generic "Bad arguments" with no goal re-prompt. A first-time user running
// bare `/evo:loop` therefore gets a raw rc=10 CLI error instead of a guided
// prompt. The fix is docs + a STRICT MODE handler rule + a durable Go
// regression test; the binary itself must NOT change (Scout: "Do NOT change
// cmd_loop_args.go's goal-required gate").
//
// Predicate strategy. The deliverable of this task IS the SKILL.md wording —
// the criterion is the documentation text itself, not a proxy for a code path
// (contrast the cycle-85 degenerate-predicate ban, which forbids grepping
// PRODUCTION SOURCE for a magic string as a stand-in for behaviour). Predicates
// 001-003 assert on the emitted SKILL.md doc artifact, and each carries the
// `// acs-predicate: config-check` waiver because an inherent documentation-text
// criterion has no runtime code path to exercise (the same waiver cycle-943 used
// for its inherent doc-comment criterion). 001 additionally asserts that the
// DURABLE Go regression test (AC-1's core deliverable) physically exists in
// go/cmd/evolve and references both SKILL.md and the goal-required wording, so a
// doc-only edit that skips the permanent lock fails it.
//
// SKILL.md is a SOURCE doc that Builder edits in the worktree; per the ACS
// dual-root convention the source root is the worktree, reached via
// acsassert.RepoRoot (git toplevel), matching cycle-354's read of
// docs/architecture/control-flags.md.
//
// RED today (all three fail by assertion, not compile):
//   - 001: SKILL.md:4 argument-hint and SKILL.md:160 Usage still contain the
//     `[goal]` optional bracket and no `<goal>` required marker; and no durable
//     regression test in go/cmd/evolve references the SKILL.md goal wording yet.
//   - 002: the STRICT MODE section has no prompt-and-wait-for-goal rule.
//   - 003: the rc=10 table row reads "Bad arguments | Re-prompt with valid args"
//     with no goal re-prompt.
```

### `go/acs/cycle1029/predicates_test.go:183` — above `assertsGoalWording := strings.Contains(src, "[goal]") ||`

```text
// The optional/required goal bracket is the discriminator: only a test
// that actually locks the goal wording contains `[goal]` or `<goal>`.
// `argument-hint` alone is too weak (unrelated skill-publish tests embed
// a fake argument-hint frontmatter line, cycle-1029 false-positive).
```
