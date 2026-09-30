# Comment history: `acs/cycle1101`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1101/predicates_test.go:3` — above `package cycle1101`

```text
// Package cycle1101 materialises the cycle-1101 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	persona-budget-inlane-gate → the in-lane build-floor gate that runs the
//	`internal/prompts` persona line-budget test when a lane's diff touches
//	`agents/evolve-*.md`.
//
// The defect (third instance of the "per-cycle-gate ≠ repo-wide-gate" class,
// after warnship-apicover-ci-gap and acs-predicate-compile-gate-at-build-exit):
// `changedPackageFloorChecks` derives its test set purely from
// `changedGoTestPackages(paths)`, which keeps only paths matching `go/**.go`.
// A lane that grows a persona doc past the 751-line budget pinned by
// `TestPersonaStopCriterionDedupe_CombinedLineCountReduced` therefore produces
// ZERO packages, hits the `len(pkgs) == 0 → return nil` early return
// (build_floor_reviewer.go:110-112), and sails through build handoff — the
// breach only lands on main's CI, reddening the build for every concurrent lane
// sharing the branch (observed twice on 2026-07-23).
//
// Predicate strategy — every predicate EXERCISES the real production entrypoint
// `core.DefaultBuildFloorChecks(ctx, ReviewInput{Phase: "build", ...})` against
// a purpose-built git worktree fixture and asserts on its RETURN VALUE. No
// predicate greps `build_floor_reviewer.go` for a magic string (the cycle-85
// degenerate-predicate ban): an implementer who writes the word "persona" into
// the source without wiring the check fails all three.
//
// The fixture is a minimal but REAL repo: a git-initialised tree with an
// `agents/evolve-scout.md` persona doc and a self-contained `go/` module whose
// `internal/prompts` package carries one test that is either RED or GREEN by
// construction. That lets each predicate vary exactly one input:
//
//   - 001 (positive / crux): persona doc touched + `internal/prompts` RED
//     ⇒ DefaultBuildFloorChecks MUST return a failure naming `internal/prompts`.
//     A no-op implementation returns an empty slice and fails here.
//   - 002 (negative / regression-safety): persona doc UNTOUCHED (only a
//     `docs/` file changed) + `internal/prompts` RED ⇒ MUST return zero
//     failures. An implementation that runs the prompts tests unconditionally
//     (or on any changed path) fails here — this is the fail-open guarantee for
//     lanes that never touch persona docs.
//   - 003 (negative / anti-false-positive): persona doc touched but
//     `internal/prompts` GREEN ⇒ MUST return zero failures. An implementation
//     that rejects on the mere PRESENCE of an `agents/evolve-*.md` path,
//     without actually running the budget test, fails here.
//
// 002 and 003 together pin the gate to the CONJUNCTION (path matched AND test
// red), which is the only behaviour the acceptance criteria admit.
//
// RED today: no persona-budget check exists in `DefaultBuildFloorChecks`, so
// 001 fails (empty failure list) while 002 and 003 pass vacuously — the
// expected RED shape for a gate that is entirely absent. 002/003 are recorded
// as pre-existing GREEN in test-report.md; they are the guards that keep the
// forthcoming implementation from over-firing.
```
