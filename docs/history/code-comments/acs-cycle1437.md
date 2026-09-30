# Comment history: `acs/cycle1437`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1437/predicates_test.go:3` — above `package cycle1437`

```text
// Package cycle1437 materialises the cycle-1437 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - salvage-baseline-measured-writeup → §6 of
//     docs/research/deliverable-alignment-2026-08/README.md still summarises the
//     recoverable-malformed rate as "not yet instrumented", contradicting §7,
//     which already carries the audited measured table (cycle-1389). This cycle
//     replaces that stale clause with the measured figures + an evidence
//     citation, and adds a §6.3 entry per the doc's own §3.8 issue/gap/solution
//     convention.
//
// Predicate strategy — the deliverable of this task IS a document, so the
// "system under test" is the emitted artifact itself. To stay out of the
// cycle-85 degenerate-predicate class (a grep for a magic string the
// implementer can trivially paste), NO predicate here asserts a hardcoded
// sentence. Every assertion is either:
//
//   - CROSS-REFERENTIAL — the expected values are *derived at runtime* from §7's
//     committed measured table and then required to appear in §6, so §6 can only
//     go green by agreeing with the audited source of record (001, 002, 003);
//   - STRUCTURAL-BY-DERIVATION — §6.3's required subsection markers are derived
//     from §6.1's own committed structure, not from a literal list (004);
//   - a real FILESYSTEM check — the code path §6.3 cites must exist on disk (004);
//   - a NEGATIVE / anti-invention assertion — history in §7 must survive the edit
//     (001), and no count may appear in §6 that §7 does not license (005).
//
// Root resolution: everything is read under acsassert.RepoRoot(t) (the cycle
// worktree, where Builder writes). Deliberately NO read of
// .evolve/runs/cycle-1389/bad-verdict-baseline.jsonl — that main-plane read is
// exactly what false-RED'd cycle-1434 (wrong-project-root, fixed by #449); the
// citation is checked as a textual cross-reference to §7 instead, which is the
// actual acceptance criterion and is worktree-local.
```
