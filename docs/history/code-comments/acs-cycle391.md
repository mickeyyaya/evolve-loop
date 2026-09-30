# Comment history: `acs/cycle391`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle391/predicates_test.go:3` — above `package cycle391`

```text
// Package cycle391 materializes the cycle-391 acceptance criteria for the
// committed top_n task:
//
//   - intent-prompt-token-reduction — remove ≥15 inert lines (≥600 bytes) from
//     agents/evolve-intent.md by deleting the C69–C73 calibration table
//     (lines ~116-126), the v9.0.1 design-correction paragraph (lines ~100-101),
//     and the "### No web research deadline" subsection (lines ~112-114),
//     while preserving every behavioral instruction, required section header,
//     and anchor.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	intent-prompt-token-reduction:
//	  AC1  line count ≤219 (baseline 234)                              → C391_001 (RED)
//	  AC1b byte count < 12769 (baseline 12769)                         → C391_002 (RED)
//	  AC2  all required section headers + behavioral anchors present   → C391_003 (pre-existing GREEN, config-check)
//	  AC3  archaeology markers absent (C69 / cycle 11 / No web ...)    → C391_004 (RED — markers present before Builder)
//	  AC4  negative: only agents/evolve-intent.md changed              → C391_005 (RED — wrong file in diff before Builder)
//	  AC5  frontmatter + Reflection Authoring tail present             → C391_006 (pre-existing GREEN, config-check)
//	  [adversarial] section-delete FAILs AC2; reflow-only FAILs AC1+AC1b;
//	                adding archaeology text FAILs AC3
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BA1 multi-file strip, BA2 docguard) get zero predicates.
```

### `go/acs/cycle391/predicates_test.go:175` — above `archaeologyMarkers := []struct {`

```text
// Each marker uniquely identifies one of the three inert blocks to remove.
// FileNotContains returns true and logs nothing when the substring is absent;
// fails + logs when the substring is still present (cycle-352 lesson).
```

### `go/acs/cycle391/predicates_test.go:194` — above `func TestC391_005_OnlyIntentFileChanged(t *testing.T) {`

```text
// TestC391_005_OnlyIntentFileChanged verifies that the cycle-391 commit changed
// exactly one file: agents/evolve-intent.md.
//
// BEHAVIORAL: runs `git diff HEAD~1..HEAD --name-only` in the worktree to list
// files changed in the most recent commit.
//
// NEGATIVE (adversarial): if Builder accidentally touched a control-plane file,
// a Go source file, or any file other than agents/evolve-intent.md, this
// predicate fails. The goal's HARD constraints prohibit all such edits.
//
// RED: before Builder's commit, HEAD in the worktree is the cycle-389 commit
// (c50c281f). The diff HEAD~1..HEAD shows agents/evolve-tester.md (cycle-389's
// change), NOT agents/evolve-intent.md, so the check fails.
```

### `go/acs/cycle391/predicates_test.go:223` — above `func TestC391_006_FrontmatterAndReflectionTailPresent(t *testing.T) {`

```text
// TestC391_006_FrontmatterAndReflectionTailPresent verifies that the YAML
// frontmatter identity fields and the Reflection Authoring tail are both intact
// after the trim.
//
// acs-predicate: config-check
//
// AC5 (edge/OOD): guards against truncation. A Builder who hits the line target
// by truncating the file's end loses the Reflection Authoring section and fails
// here. A Builder who strips the frontmatter loses the agent's identity.
//
// Also verifies git-tracking (cycle-93 lesson): disk presence alone is
// insufficient — a gitignored file would be silently dropped at ship.
//
// Pre-existing GREEN: both are present in the current 234-line file.
// Must remain GREEN after Builder's edit.
```

### `go/acs/cycle391/predicates_test.go:251` — above `_, _, gitCode, gitErr := acsassert.SubprocessOutput(`

```text
// Git-tracking check (cycle-93 lesson: disk presence alone is insufficient).
```
