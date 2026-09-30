# Comment history: `acs/cycle678`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle678/predicates_test.go:3` — above `package cycle678`

```text
// Package cycle678 materialises the acceptance criteria for cycle 678's
// triage-committed task, inbox `new-package-graduation-buildentry-gate`
// (weight 0.92, RETRY — 3rd recurrence: cycles 575/587/652).
//
// CLOSURE-VERIFICATION CYCLE: the implementation itself landed in cycle 675
// (commit 1370807e — buildGraduationCheck + abort wiring at the post-build
// seam, audit-side gate registration pinned; audit PASS 0.92). The inbox item
// re-triaged as RETRY only because its id never matched cycle-675's
// differently-named slugs (`build-entry-graduation-guard[-audit-regression]`),
// so it was never promoted out of .evolve/inbox/. This cycle closes the item
// under its OWN id: predicates re-pin every acceptance criterion against HEAD
// so a regression to the landed guard fails here, and C678_005 adds the one
// check cycle 675 did not predicate — the inbox fix seam (2): the repo-wide
// apicover COMPLETENESS predicate is inside the cycle audit's repo-wide gate
// set and GREEN at HEAD.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…675 precedent) —
// each predicate shells `go test -run` over unit tests that EXERCISE the SUT
// (buildGraduationCheck over real git worktree fixtures; recordAndBranch
// (PhaseBuild); the production-constructed audit phase end-to-end); none is
// source-grep. All are verify-only GREEN by design (regression pins for a
// landed fix), honestly declared — the RED-first arm of this item was run and
// satisfied in cycle 675.
```
