# Comment history: `acs/cycle388`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle388/predicates_test.go:3` — above `package cycle388`

```text
// Package cycle388 materializes the cycle-388 acceptance criteria for the
// committed top_n task:
//
//   - trim-builder-prompt-redundancy — remove ≥9 redundant lines (and ≥56 words,
//     strictly fewer bytes) from agents/evolve-builder.md by consolidating the
//     triply-restated turn-exit rule and removing a duplicate self-assess-PASS
//     anecdote, while preserving every real section, frontmatter field, and
//     behavioral keyword.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-builder-prompt-redundancy:
//	  AC1  line count < 280 (baseline 288)                              → C388_001 (RED)
//	  AC2  word count < 2780 (baseline 2835)                           → C388_002 (RED)
//	  AC3  byte count < 21994 (baseline 21994)                         → C388_003 (RED)
//	  AC4  frontmatter intact (name: evolve-builder)                   → C388_004 (pre-existing GREEN)
//	  AC5  all 16 real ## section headers present                      → C388_005 (pre-existing GREEN)
//	  AC6  behavior keywords preserved                                  → C388_006 (pre-existing GREEN)
//	  AC7  file is git-tracked                                          → C388_007 (pre-existing GREEN)
//	  [adversarial] section-delete FAILs AC5; reflow-only FAILs AC2+AC3
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred task (auditor dedup) gets zero predicates.
```

### `go/acs/cycle388/predicates_test.go:197` — above `func TestC388_007_FileIsGitTracked(t *testing.T) {`

```text
// TestC388_007_FileIsGitTracked verifies that agents/evolve-builder.md remains
// a tracked git file after Builder's markdown-only edit.
//
// BEHAVIORAL: runs `git ls-files --error-unmatch` as a subprocess. Disk presence
// alone is insufficient — a gitignored worktree file would be silently dropped at
// ship (cycle-93 lesson).
//
// Pre-existing GREEN: file is already tracked. Builder's markdown edit must not
// cause it to become untracked or gitignored.
```
