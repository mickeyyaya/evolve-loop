# Comment history: `acs/cycle387`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle387/predicates_test.go:3` — above `package cycle387`

```text
// Package cycle387 materializes the cycle-387 acceptance criteria for the
// committed top_n task:
//
//   - trim-tdd-engineer-prompt-redundancy — remove ≥27 redundant lines from
//     agents/evolve-tdd-engineer.md (retired-bash repetitions and fallback shell
//     example) while preserving every real section, frontmatter field, and
//     behavioral keyword.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-tdd-engineer-prompt-redundancy:
//	  AC1  line count ≤405 (≥27-line reduction from baseline 432)   → C387_001 (RED)
//	  AC2  all 9 ## section headers present                          → C387_002 (pre-existing GREEN)
//	  AC3  frontmatter + behavioral keywords preserved               → C387_003 (pre-existing GREEN)
//	  AC4  file is git-tracked (markdown-only edit, tracked)         → C387_004 (pre-existing GREEN)
//	  [adversarial] sections not dropped to hit line count           → C387_002 (anti-gaming via AC1+AC2 combo)
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BA1, BA2, carryover infra todos) get zero predicates.
```

### `go/acs/cycle387/predicates_test.go:123` — above `func TestC387_004_FileIsGitTracked(t *testing.T) {`

```text
// TestC387_004_FileIsGitTracked verifies that agents/evolve-tdd-engineer.md
// remains a tracked git file after Builder's markdown-only edit.
//
// BEHAVIORAL: runs `git ls-files --error-unmatch` as a subprocess. Disk presence
// alone is insufficient — a gitignored worktree file would be silently dropped at
// ship (cycle-93 lesson).
//
// Pre-existing GREEN: file is already tracked. Builder's markdown edit must not
// cause it to become untracked or gitignored.
```
