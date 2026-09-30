# Comment history: `acs/cycle389`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle389/predicates_test.go:3` — above `package cycle389`

```text
// Package cycle389 materializes the cycle-389 acceptance criteria for the
// committed top_n task:
//
//   - trim-evolve-tester-prompt — remove ≥8 redundant lines (and ≥40 words)
//     from agents/evolve-tester.md by collapsing the triplicated worktree-resolution
//     boilerplate, tightening the adversarial-mindset restatement, and condensing
//     low-density preamble prose, while preserving every real section, frontmatter
//     field, and behavioral contract.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-evolve-tester-prompt:
//	  AC1  line count ≤185 (baseline 193)                              → C389_001 (RED)
//	  AC1b word count ≤1280 (baseline 1320)                           → C389_002 (RED)
//	  AC2  all 10 ## section headers present                          → C389_003 (pre-existing GREEN)
//	  AC3  banned-patterns list + metadata contract intact            → C389_004 (pre-existing GREEN)
//	  AC4  negative: only agents/evolve-tester.md changed             → C389_005 (RED — wrong file in diff before Builder)
//	  AC5  frontmatter + Reflection Authoring tail present            → C389_006 (pre-existing GREEN)
//	  [adversarial] section-delete FAILs AC2; reflow-only FAILs AC1+AC1b
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (B1 DRY pass, B2 linter) get zero predicates.
```

### `go/acs/cycle389/predicates_test.go:167` — above `func TestC389_005_OnlyTesterFileChanged(t *testing.T) {`

```text
// TestC389_005_OnlyTesterFileChanged verifies that the cycle-389 commit changed
// exactly one file: agents/evolve-tester.md.
//
// BEHAVIORAL: runs `git diff HEAD~1..HEAD --name-only` in the worktree to list
// files changed in the most recent commit.
//
// NEGATIVE (adversarial): if Builder accidentally touched a control-plane file,
// a Go source file, or any file other than agents/evolve-tester.md, this
// predicate fails. The goal's HARD constraints prohibit all such edits.
//
// RED: before Builder's commit, HEAD in the worktree is the cycle-388 commit
// (d84e1fa7). The diff HEAD~1..HEAD shows agents/evolve-builder.md (the
// cycle-388 edit), NOT agents/evolve-tester.md, so the check fails.
```

### `go/acs/cycle389/predicates_test.go:219` — above `_, _, gitCode, gitErr := acsassert.SubprocessOutput(`

```text
// Git-tracking check (cycle-93 lesson: disk presence alone is insufficient).
```
