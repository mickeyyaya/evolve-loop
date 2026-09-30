# Comment history: `acs/cycle539`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle539/predicates_test.go:3` — above `package cycle539`

```text
// Package cycle539 materialises the cycle-539 acceptance criteria for the single
// triage-committed (`## top_n`) task: fix-dossier-tree-diff-guard-blocker.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  fix-dossier-tree-diff-guard-blocker (H) — C539_001..004
//	Every `## deferred` item (the distinct real-source-leak bug, the
//	SELF_SHA_TAMPERED ship backlog, the review-gate/bridge-launch tail) gets
//	ZERO predicates here.
//
// FEATURE CONTEXT
//
//	internal/dossier/write.go's `commit bool` parameter is documented "reserved
//	for a future slice ... pass false for now", and internal/core/
//	dossier_producer.go:59 always calls Write(d, dir, false). Every cycle's
//	closeout dossier (knowledge-base/cycles/cycle-N.{json,md}) is therefore
//	written to the main tree but NEVER git-committed, leaving 40 untracked pairs
//	(cycle-474..537) that a later, unrelated phase's tree-diff guard flags as a
//	main-tree leak — aborting the whole cycle (538/524/520/...). This cycle
//	implements the reserved parameter: Write(d, dir, true) git-adds + git-commits
//	exactly the two new files, scoped, so future dossiers leave a clean tree.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real dossier.Write against a seeded temp git repo and asserts on
// the actual git side effect (committed / tracked / untracked), never a "source
// file contains text X" grep.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C539_001 Write(true) commits BOTH files (tracked, clean tree).
//   - Negative : C539_002 Write(false) commits NOTHING (backward-compat guard —
//     an implementation that ignores the flag and always commits FAILS here).
//   - Negative : C539_003 Write(true) is SCOPED — a pre-existing unrelated dirty
//     file stays UNTRACKED (a `git add -A`/`git add .` implementation FAILS here;
//     the anti-no-op that pins "scoped to just the two new files").
//   - Hygiene  : C539_004 the touched packages (dossier + core) build + vet clean.
```
