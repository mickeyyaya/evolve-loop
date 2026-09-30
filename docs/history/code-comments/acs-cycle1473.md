# Comment history: `acs/cycle1473`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1473/predicates_test.go:3` — above `package cycle1473`

```text
// Package cycle1473 materialises the acceptance criteria for the single task
// this lane committed in triage `## top_n`:
//
//   - gitstage-deterministic-classification → classify a `git add` failure from
//     the CAPTURED git stderr before assigning the recovery class, so git's own
//     deterministic fatals stop burning the transient retry budget.
//
// The three other items in this lane's fleet scope (ship-addall-staging-surface,
// ship-push-only-recovery, gitstage-collider-quotepath) are triage `## deferred`
// — already delivered and audit-accepted in cycle 1469 — so per R9.3 they get
// ZERO predicates here.
//
// Predicate strategy. The classifier is an UNEXPORTED seam inside
// go/internal/phases/ship, reachable only through its production caller
// stageExplicitPaths, so these predicates drive the in-package reachability
// contract (stage_classify_stderr_test.go) as a subprocess and assert on its
// exit code. Neither predicate greps production source — a magic string added to
// gitops.go cannot make either pass; only a classifier that stageExplicitPaths
// actually consults can (the cycle-85 degenerate-predicate ban).
//
// Flaky-shape compliance: each predicate runs ONE named package
// (./internal/phases/ship — not a `/...` sweep, and not one of the 40s+ suites),
// narrowed further with -run, with cmd.Dir set explicitly so the invocation is
// independent of the lane's process cwd.
```

### `go/acs/cycle1473/predicates_test.go:90` — above `func TestC1473_003_TwoStrikesRouterSurvives(t *testing.T) {`

```text
// TestC1473_003_TwoStrikesRouterSurvives guards the cycle-1440 router already in
// production: the new stderr classifier must sit in FRONT of the two-strikes
// memo, not replace it. An unclassifiable refusal keeps its first retry and
// still escalates to precondition on the second consecutive same-pathspec
// attempt.
```
