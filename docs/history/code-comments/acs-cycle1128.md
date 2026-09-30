# Comment history: `acs/cycle1128`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1128/predicates_test.go:3` — above `package cycle1128`

```text
// Package cycle1128 materialises the cycle-1128 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	core-suite-shared-tmp-p-hermeticity → widen
//	TestCoreTests_NeverPinSharedTmpProjectRoot from package-local reach
//	(os.ReadDir(".") sees only internal/core) to the whole go/internal tree.
//
// Why this cycle exists. PR #365 (97bcb4ec) killed the shared machine-global
// project-root class inside internal/core after it produced 19,521 accumulated
// run entries and the false suites_stay_green reds of cycles 1107/1116. The
// guard it added works — and currently sees exactly one package. A test added
// in ANY other package that re-pins the fixed shared root would ship silently,
// which defeats the guard's own stated purpose ("this guard keeps the class
// dead").
//
// Predicate strategy — every predicate EXERCISES the guard by running it
// (`go test -run TestCoreTests_NeverPinSharedTmpProjectRoot`) against a tree we
// mutate, and asserts on its exit code and reported offender. None of them
// greps the guard's source, so no magic string can satisfy them (the cycle-85
// degenerate-predicate ban). Concretely:
//
//   - 001 (positive / no-false-positive): the guard is GREEN on the untouched
//     worktree. A widened walk that trips on the current clean tree — or on the
//     guard's own concatenated self-reference — fails here.
//   - 002 (NEGATIVE, crux): plant the shared-root literal in a FLAT sibling
//     package (internal/redteamcheck) and require the guard to FAIL and to name
//     the planted file. Package-local reach cannot pass this; this is the
//     predicate that is RED today.
//   - 003 (NEGATIVE, depth): plant the same literal in a NESTED package
//     (internal/phases/build, two levels down) and require the same failure. A
//     naive one-level widening (os.ReadDir("..")) passes 002 but fails 003, so
//     the pair pins a real recursive walk.
//   - 004 (edge / over-broad-detector): plant the literal in a NON-test .go file
//     and require the guard to stay GREEN. The class is "tests pinning a shared
//     root"; a widened guard that greps every .go file would false-red on
//     production code that legitimately names a tmp path.
//
// Every planted probe is removed by t.Cleanup on every exit path, so the
// worktree is byte-identical before and after this suite runs.
```
