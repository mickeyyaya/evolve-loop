# Comment history: `acs/cycle1777`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1777/predicates_test.go:3` — above `package cycle1777`

```text
// Package cycle1777 materialises the cycle-1777 acceptance criteria for the
// three independent size-ratchet shrink tasks pinned to this lane:
//
//   - sizeratchet-verifyeval-shellwords: internal/verifyeval.shellWords
//   - sizeratchet-versionbump-run:       internal/versionbump.Run
//   - sizeratchet-llmcalls-aggregate:    internal/llmcalls.Aggregate
//
// Each function is currently listed in go/internal/sizeratchet/offenders.json
// as exceeding sizeratchet.MaxLines (50); the task is a behavior-preserving
// extraction that shrinks it to the limit and drops its offenders.json entry.
//
// Predicate strategy — every predicate exercises a real artifact or the real
// sizeratchet.Walk/LoadOffenders production code path against the live
// worktree source, never a source-grep for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001/004/007 parse the LIVE offenders.json and assert the task's key is
//     absent — the "entry dropped" half of the acceptance bar.
//   - 002/005/008 run the real sizeratchet.Walk over the live go/ tree and
//     assert the task's function span is at or under MaxLines — the "shrunk"
//     half. This is the crux predicate: today it fails because the function is
//     still 51/52/53 lines.
//   - 003/006/009 run the package's own test suite (a single named package,
//     never a repo-wide sweep) so the extraction is caught the moment it
//     changes observable behavior. These pass today (pre-existing GREEN) and
//     stay green as a regression guard through the refactor.
//
// gofmt/go vet cleanliness on the touched files is dispositioned
// manual+checklist in test-report.md (Builder runs and reports; a leaf
// two-command lint check has no meaningful RED/GREEN state before the
// refactor exists to lint).
```
